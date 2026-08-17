package component

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
	goldmarkutil "github.com/yuin/goldmark/util"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

var (
	markdownParser          = goldmark.New(goldmark.WithExtensions(extension.GFM)).Parser()
	markdownWikiImageSyntax = regexp.MustCompile(`!\[\[([^\]]+)\]\]`)
	markdownImageLine       = regexp.MustCompile(`^!\[[^\]]*\]\([^)]+\)$`)
)

type markdownBlockKind uint8

const (
	markdownParagraph markdownBlockKind = iota
	markdownHeading
	markdownCode
	markdownQuote
	markdownList
	markdownRule
	markdownTable
	markdownImage
)

type markdownInlineStyle struct {
	bold   bool
	code   bool
	strike bool
	link   string
}

type markdownRun struct {
	text  string
	style markdownInlineStyle
}

type markdownBlock struct {
	kind       markdownBlockKind
	level      int
	language   string
	image      string
	imageLabel string
	runs       []markdownRun
	children   []markdownBlock
	items      []markdownListItem
	table      markdownTableData
}

type markdownListItem struct {
	marker string
	blocks []markdownBlock
}

type markdownTableData struct {
	rows       [][]string
	headerRows int
}

// MarkdownDocument is the reusable parsed representation consumed by WoxMarkdown.
type MarkdownDocument struct {
	blocks []markdownBlock
}

// MarkdownProps describes one native Markdown document and its external actions.
type MarkdownProps struct {
	ID       string
	Document MarkdownDocument
	Width    float32
	FontSize float32
	// BlockGap overrides the default 12px preview spacing between top-level blocks.
	// Compact form help text should pass a smaller gap so multi-paragraph tips stay dense.
	BlockGap float32
	// ExcludeLinkFocus keeps pointer-activated links out of the keyboard focus chain.
	// Flutter wraps form-table tooltips in ExcludeFocus for the same reason.
	ExcludeLinkFocus bool
	Theme            Theme
	Window           *woxui.Window
	ResolveImage     func(source string) (*woxui.Image, string)
	OnOpenImage      func(source string)
	OnOpenLink       func(target string)
	// InlineTrailing appends a control to the final top-level inline paragraph.
	InlineTrailing woxwidget.Widget
}

// ParseMarkdown parses CommonMark with the GitHub-flavored extensions used by Wox previews.
func ParseMarkdown(value string) MarkdownDocument {
	source := []byte(normalizeMarkdownImages(value))
	document := markdownParser.Parse(text.NewReader(source))
	return MarkdownDocument{blocks: parseMarkdownBlocks(document, source)}
}

// WoxMarkdown builds a native Markdown widget tree without a browser surface.
func WoxMarkdown(props MarkdownProps) woxwidget.Widget {
	width := max(float32(0), props.Width)
	linkIndex := 0
	blockGap := props.BlockGap
	if blockGap <= 0 {
		blockGap = 12
	}
	blocks := renderMarkdownBlocks(props.Document.blocks, props, width, &linkIndex)
	if props.InlineTrailing != nil && len(blocks) > 0 {
		if paragraph, ok := blocks[len(blocks)-1].(woxwidget.Wrap); ok {
			paragraph.Children = append(paragraph.Children, props.InlineTrailing)
			blocks[len(blocks)-1] = paragraph
		}
	}
	return woxwidget.Flex{Axis: woxwidget.Vertical, Gap: blockGap, Children: blocks}
}

// normalizeMarkdownImages preserves Wox's wiki-image shorthand before CommonMark parsing.
func normalizeMarkdownImages(value string) string {
	value = markdownWikiImageSyntax.ReplaceAllStringFunc(value, func(match string) string {
		parts := markdownWikiImageSyntax.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		content := strings.TrimSpace(parts[1])
		path, label, _ := strings.Cut(content, "|")
		path = strings.TrimSpace(path)
		label = strings.TrimSpace(label)
		if path == "" {
			return match
		}
		return fmt.Sprintf("![%s](%s)", label, path)
	})
	lines := strings.Split(value, "\n")
	normalized := make([]string, 0, len(lines)+2)
	for index, line := range lines {
		standaloneImage := markdownImageLine.MatchString(strings.TrimSpace(line))
		if standaloneImage && len(normalized) > 0 && strings.TrimSpace(normalized[len(normalized)-1]) != "" {
			normalized = append(normalized, "")
		}
		normalized = append(normalized, line)
		if standaloneImage && index+1 < len(lines) && strings.TrimSpace(lines[index+1]) != "" {
			normalized = append(normalized, "")
		}
	}
	return strings.Join(normalized, "\n")
}

// parseMarkdownBlocks converts Goldmark nodes into a renderer-owned immutable block model.
func parseMarkdownBlocks(parent ast.Node, source []byte) []markdownBlock {
	blocks := make([]markdownBlock, 0)
	for node := parent.FirstChild(); node != nil; node = node.NextSibling() {
		switch value := node.(type) {
		case *ast.Paragraph:
			if image, ok := paragraphMarkdownImage(value, source); ok {
				blocks = append(blocks, image)
			} else {
				blocks = append(blocks, markdownBlock{kind: markdownParagraph, runs: collectMarkdownRuns(value, source, markdownInlineStyle{})})
			}
		case *ast.TextBlock:
			blocks = append(blocks, markdownBlock{kind: markdownParagraph, runs: collectMarkdownRuns(value, source, markdownInlineStyle{})})
		case *ast.Heading:
			blocks = append(blocks, markdownBlock{kind: markdownHeading, level: value.Level, runs: collectMarkdownRuns(value, source, markdownInlineStyle{bold: true})})
		case *ast.CodeBlock:
			blocks = append(blocks, markdownBlock{kind: markdownCode, runs: []markdownRun{{text: string(value.Text(source))}}})
		case *ast.FencedCodeBlock:
			blocks = append(blocks, markdownBlock{kind: markdownCode, language: string(value.Language(source)), runs: []markdownRun{{text: string(value.Text(source))}}})
		case *ast.Blockquote:
			blocks = append(blocks, markdownBlock{kind: markdownQuote, children: parseMarkdownBlocks(value, source)})
		case *ast.List:
			blocks = append(blocks, parseMarkdownList(value, source))
		case *ast.ThematicBreak:
			blocks = append(blocks, markdownBlock{kind: markdownRule})
		case *extast.Table:
			blocks = append(blocks, markdownBlock{kind: markdownTable, table: parseMarkdownTable(value, source)})
		case *ast.HTMLBlock:
			text := strings.TrimSpace(string(value.Text(source)))
			if text != "" {
				blocks = append(blocks, markdownBlock{kind: markdownParagraph, runs: []markdownRun{{text: text}}})
			}
		default:
			if value.Type() == ast.TypeBlock {
				blocks = append(blocks, parseMarkdownBlocks(value, source)...)
			}
		}
	}
	return blocks
}

// paragraphMarkdownImage promotes standalone images into native image blocks.
func paragraphMarkdownImage(paragraph *ast.Paragraph, source []byte) (markdownBlock, bool) {
	var image *ast.Image
	for child := paragraph.FirstChild(); child != nil; child = child.NextSibling() {
		switch value := child.(type) {
		case *ast.Image:
			if image != nil {
				return markdownBlock{}, false
			}
			image = value
		case *ast.Text:
			if strings.TrimSpace(string(value.Segment.Value(source))) != "" {
				return markdownBlock{}, false
			}
		default:
			return markdownBlock{}, false
		}
	}
	if image == nil {
		return markdownBlock{}, false
	}
	return markdownBlock{
		kind: markdownImage, image: strings.TrimSpace(markdownText(image.Destination, false)), imageLabel: strings.TrimSpace(markdownPlainText(image, source)),
	}, true
}

// parseMarkdownList keeps nested block structure while assigning visible markers.
func parseMarkdownList(list *ast.List, source []byte) markdownBlock {
	block := markdownBlock{kind: markdownList}
	index := list.Start
	for child := list.FirstChild(); child != nil; child = child.NextSibling() {
		item, ok := child.(*ast.ListItem)
		if !ok {
			continue
		}
		marker := "•"
		if list.IsOrdered() {
			marker = fmt.Sprintf("%d.", index)
			index++
		}
		blocks := parseMarkdownBlocks(item, source)
		if len(blocks) > 0 && len(blocks[0].runs) > 0 {
			firstText := blocks[0].runs[0].text
			if strings.HasPrefix(firstText, "☐ ") || strings.HasPrefix(firstText, "☑ ") {
				marker = ""
			}
		}
		block.items = append(block.items, markdownListItem{marker: marker, blocks: blocks})
	}
	return block
}

// parseMarkdownTable flattens cell content into the compact native table model.
func parseMarkdownTable(table *extast.Table, source []byte) markdownTableData {
	data := markdownTableData{}
	for row := table.FirstChild(); row != nil; row = row.NextSibling() {
		_, header := row.(*extast.TableHeader)
		if header {
			data.headerRows++
		}
		cells := make([]string, 0)
		for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
			cells = append(cells, strings.TrimSpace(markdownPlainText(cell, source)))
		}
		if len(cells) > 0 {
			data.rows = append(data.rows, cells)
		}
	}
	return data
}

// collectMarkdownRuns preserves inline styles supported by the portable renderer.
func collectMarkdownRuns(parent ast.Node, source []byte, style markdownInlineStyle) []markdownRun {
	runs := make([]markdownRun, 0)
	var visit func(ast.Node, markdownInlineStyle)
	visit = func(node ast.Node, inherited markdownInlineStyle) {
		current := inherited
		switch value := node.(type) {
		case *ast.Text:
			text := markdownText(value.Segment.Value(source), value.IsRaw() || current.code)
			if value.HardLineBreak() {
				text += "\n"
			} else if value.SoftLineBreak() {
				text += " "
			}
			appendMarkdownRun(&runs, text, current)
			return
		case *ast.String:
			appendMarkdownRun(&runs, markdownText(value.Value, value.IsRaw() || value.IsCode() || current.code), current)
			return
		case *ast.Emphasis:
			if value.Level >= 2 {
				current.bold = true
			}
		case *ast.CodeSpan:
			current.code = true
		case *ast.Link:
			current.link = safeMarkdownLink(string(value.Destination))
		case *ast.AutoLink:
			target := safeMarkdownLink(string(value.URL(source)))
			appendMarkdownRun(&runs, string(value.Label(source)), markdownInlineStyle{link: target})
			return
		case *extast.Strikethrough:
			current.strike = true
		case *extast.TaskCheckBox:
			if value.IsChecked {
				appendMarkdownRun(&runs, "☑ ", current)
			} else {
				appendMarkdownRun(&runs, "☐ ", current)
			}
			return
		case *ast.Image:
			label := strings.TrimSpace(markdownPlainText(value, source))
			if label == "" {
				label = strings.TrimSpace(string(value.Destination))
			}
			appendMarkdownRun(&runs, "🖼 "+label, current)
			return
		case *ast.RawHTML:
			appendMarkdownRun(&runs, string(value.Text(source)), current)
			return
		}
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			visit(child, current)
		}
	}
	for child := parent.FirstChild(); child != nil; child = child.NextSibling() {
		visit(child, style)
	}
	return runs
}

func appendMarkdownRun(runs *[]markdownRun, value string, style markdownInlineStyle) {
	if value == "" {
		return
	}
	if len(*runs) > 0 && (*runs)[len(*runs)-1].style == style {
		(*runs)[len(*runs)-1].text += value
		return
	}
	*runs = append(*runs, markdownRun{text: value, style: style})
}

// markdownText applies the same punctuation and entity decoding as Goldmark's HTML writer.
func markdownText(value []byte, raw bool) string {
	if raw {
		return string(value)
	}
	value = goldmarkutil.UnescapePunctuations(value)
	value = goldmarkutil.ResolveNumericReferences(value)
	return string(goldmarkutil.ResolveEntityNames(value))
}

// markdownPlainText extracts accessible labels and compact table values.
func markdownPlainText(parent ast.Node, source []byte) string {
	var value strings.Builder
	_ = ast.Walk(parent, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch current := node.(type) {
		case *ast.Text:
			value.Write(current.Segment.Value(source))
		case *ast.String:
			value.Write(current.Value)
		case *ast.AutoLink:
			value.Write(current.Label(source))
		case *extast.TaskCheckBox:
			if current.IsChecked {
				value.WriteString("☑ ")
			} else {
				value.WriteString("☐ ")
			}
		}
		return ast.WalkContinue, nil
	})
	return value.String()
}

// safeMarkdownLink accepts only schemes supported by Window.OpenExternalURL.
func safeMarkdownLink(target string) string {
	target = strings.TrimSpace(markdownText([]byte(target), false))
	parsed, err := url.ParseRequestURI(target)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	return parsed.String()
}

// renderMarkdownBlocks maps the parsed document to portable widgets.
func renderMarkdownBlocks(blocks []markdownBlock, props MarkdownProps, width float32, linkIndex *int) []woxwidget.Widget {
	widgets := make([]woxwidget.Widget, 0, len(blocks))
	for _, block := range blocks {
		widgets = append(widgets, renderMarkdownBlock(block, props, width, linkIndex))
	}
	return widgets
}

// renderMarkdownBlock picks the simplest native surface for one block.
func renderMarkdownBlock(block markdownBlock, props MarkdownProps, width float32, linkIndex *int) woxwidget.Widget {
	fontSize := markdownFontSize(props)
	switch block.kind {
	case markdownHeading:
		return markdownRunsWidget(block.runs, props, width, fontSize, linkIndex)
	case markdownCode:
		return markdownCodeWidget(block, props, width)
	case markdownQuote:
		innerWidth := max(float32(0), width-22)
		return woxwidget.Container{
			Width: width, Padding: woxwidget.Insets{Left: 14, Top: 10, Right: 8, Bottom: 10},
			Color: withAlpha(props.Theme.PreviewText, 10), BorderColor: withAlpha(props.Theme.PreviewSplit, 110), BorderWidth: 1, Radius: 5,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, Children: renderMarkdownBlocks(block.children, props, innerWidth, linkIndex)},
		}
	case markdownList:
		return markdownListWidget(block, props, width, linkIndex)
	case markdownRule:
		return woxwidget.Container{Width: width, Height: 1, Color: withAlpha(props.Theme.PreviewSplit, 150)}
	case markdownTable:
		return markdownTableWidget(block.table, props, width)
	case markdownImage:
		return markdownImageWidget(block, props, width, linkIndex)
	default:
		return markdownRunsWidget(block.runs, props, width, fontSize, linkIndex)
	}
}

func markdownFontSize(props MarkdownProps) float32 {
	if props.FontSize > 0 {
		return props.FontSize
	}
	return 12
}

// markdownRunsWidget wraps inline text while retaining native link actions.
func markdownRunsWidget(runs []markdownRun, props MarkdownProps, width, fontSize float32, linkIndex *int) woxwidget.Widget {
	children := make([]woxwidget.Widget, 0, len(runs)*2)
	for _, run := range runs {
		style := woxui.TextStyle{Size: fontSize}
		if run.style.bold {
			style.Weight = woxui.FontWeightSemibold
		}
		color := props.Theme.PreviewText
		if run.style.strike {
			color = withAlpha(color, 150)
		}
		if run.style.code {
			for _, token := range markdownTokens(run.text) {
				children = append(children, woxwidget.Container{
					Padding: woxwidget.Insets{Left: 4, Top: 2, Right: 4, Bottom: 2}, Radius: 3, Color: withAlpha(props.Theme.PreviewText, 18),
					Child: woxwidget.Text{Value: token, Style: woxui.TextStyle{Size: max(float32(10), fontSize-1)}, Color: color},
				})
			}
			continue
		}
		if run.style.link != "" && props.OnOpenLink != nil {
			(*linkIndex)++
			id := fmt.Sprintf("%s-link-%d", props.ID, *linkIndex)
			target := run.style.link
			label := strings.TrimSpace(run.text)
			if label == "" {
				continue
			}
			link := woxwidget.Gesture{ID: id, OnTap: func() { props.OnOpenLink(target) }, Child: woxwidget.Text{Value: label, Style: style, Color: props.Theme.Cursor, Underline: true}}
			semantics := woxwidget.Semantics{
				Key: woxwidget.Key(id), AutomationID: id, Role: woxui.AccessibilityRoleLink, Label: label, Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate},
				OnAction: func(action woxui.AccessibilityAction, _ string) error {
					if action == woxui.AccessibilityActionActivate {
						props.OnOpenLink(target)
					}
					return nil
				},
				Child: link,
			}
			if !props.ExcludeLinkFocus {
				semantics.Child = woxwidget.Focusable{Key: woxwidget.Key(id), FocusRingColor: props.Theme.Cursor, FocusRingRadius: 2, OnKey: func(event woxui.KeyEvent) bool {
					if event.Key != woxui.KeyEnter && event.Key != woxui.KeySpace {
						return false
					}
					if event.Down {
						props.OnOpenLink(target)
					}
					return true
				}, Child: link}
			}
			children = append(children, semantics)
			continue
		}
		for _, token := range markdownTokens(run.text) {
			children = append(children, woxwidget.Text{Value: token, Style: style, Color: color})
		}
	}
	return woxwidget.Wrap{Gap: 0, RunGap: max(float32(3), fontSize*0.25), Children: children}
}

// markdownTokens exposes word and CJK boundaries to the existing Wrap widget.
func markdownTokens(value string) []string {
	tokens := make([]string, 0, utf8.RuneCountInString(value))
	var word strings.Builder
	flush := func() {
		if word.Len() > 0 {
			tokens = append(tokens, word.String())
			word.Reset()
		}
	}
	for _, r := range value {
		if r == '\n' || unicode.IsSpace(r) {
			flush()
			tokens = append(tokens, " ")
			continue
		}
		if isMarkdownWideRune(r) {
			flush()
			tokens = append(tokens, string(r))
			continue
		}
		word.WriteRune(r)
	}
	flush()
	return tokens
}

func isMarkdownWideRune(value rune) bool {
	return unicode.Is(unicode.Han, value) || unicode.Is(unicode.Hangul, value) || unicode.Is(unicode.Hiragana, value) || unicode.Is(unicode.Katakana, value)
}

// markdownCodeWidget reuses the cross-platform text layout inside a code surface.
func markdownCodeWidget(block markdownBlock, props MarkdownProps, width float32) woxwidget.Widget {
	innerWidth := max(float32(0), width-20)
	code := strings.TrimSuffix(block.runs[0].text, "\n")
	fontSize := markdownFontSize(props)
	style := woxui.TextStyle{Size: max(float32(10), fontSize-1)}
	layout := woxwidget.LayoutTextBlock(props.Window, code, style, innerWidth, 0, 17)
	children := make([]woxwidget.Widget, 0, 2)
	if block.language != "" {
		children = append(children, woxwidget.Text{Value: block.language, Style: woxui.TextStyle{Size: max(float32(9), fontSize-2), Weight: woxui.FontWeightSemibold}, Color: withAlpha(props.Theme.PreviewText, 180)})
	}
	children = append(children, woxwidget.TextBlock{Value: code, Width: innerWidth, Height: layout.Size.Height, Layout: &layout, Style: style, LineHeight: 17, Color: props.Theme.PreviewText})
	return woxwidget.Container{
		Width: width, Padding: woxwidget.UniformInsets(10), Radius: 5, Color: withAlpha(props.Theme.PreviewText, 14), BorderColor: withAlpha(props.Theme.PreviewSplit, 90), BorderWidth: 1,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 7, Children: children},
	}
}

// markdownListWidget preserves nested blocks inside one row per list item.
func markdownListWidget(block markdownBlock, props MarkdownProps, width float32, linkIndex *int) woxwidget.Widget {
	rows := make([]woxwidget.Widget, 0, len(block.items))
	for _, item := range block.items {
		markerWidth := float32(28)
		rows = append(rows, woxwidget.Flex{Axis: woxwidget.Horizontal, CrossAxisAlignment: woxwidget.CrossAxisStart, Children: []woxwidget.Widget{
			woxwidget.Container{Width: markerWidth, Padding: woxwidget.Insets{Top: 1}, Child: woxwidget.Text{Value: item.marker, Style: woxui.TextStyle{Size: markdownFontSize(props), Weight: woxui.FontWeightSemibold}, Color: props.Theme.PreviewText}},
			woxwidget.Container{Width: max(float32(0), width-markerWidth), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 7, Children: renderMarkdownBlocks(item.blocks, props, max(float32(0), width-markerWidth), linkIndex)}},
		}})
	}
	return woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 7, Children: rows}
}

// markdownTableWidget keeps wide GFM tables horizontally scrollable.
func markdownTableWidget(table markdownTableData, props MarkdownProps, width float32) woxwidget.Widget {
	if len(table.rows) == 0 {
		return woxwidget.Container{Width: width}
	}
	columns := 1
	for _, row := range table.rows {
		columns = max(columns, len(row))
	}
	cellWidth := max(float32(120), width/float32(columns))
	contentWidth := cellWidth * float32(columns)
	rows := make([]woxwidget.Widget, 0, len(table.rows))
	for rowIndex, row := range table.rows {
		cells := make([]woxwidget.Widget, 0, columns)
		for column := 0; column < columns; column++ {
			value := ""
			if column < len(row) {
				value = row[column]
			}
			weight := woxui.FontWeightRegular
			background := woxui.Color{}
			if rowIndex < table.headerRows {
				weight = woxui.FontWeightSemibold
				background = withAlpha(props.Theme.PreviewText, 12)
			}
			cells = append(cells, woxwidget.Container{
				Width: cellWidth, Height: 38, Padding: woxwidget.Insets{Left: 8, Right: 8}, Color: background, BorderColor: withAlpha(props.Theme.PreviewSplit, 100), BorderWidth: 1,
				Child: woxwidget.Align{Width: max(float32(0), cellWidth-16), Height: 38, Vertical: 0.5, Child: woxwidget.TextBlock{
					Value: value, Width: max(float32(0), cellWidth-16), Height: 18, LineHeight: 18, MaxLines: 1, AlignmentY: 0.5, Style: woxui.TextStyle{Size: markdownFontSize(props), Weight: weight}, Color: props.Theme.PreviewText,
				}},
			})
		}
		rows = append(rows, woxwidget.Flex{Axis: woxwidget.Horizontal, Children: cells})
	}
	height := float32(len(rows)) * 38
	return woxwidget.ScrollView{Width: width, Height: height, ContentWidth: contentWidth, Horizontal: true, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}}
}

// markdownImageWidget reuses launcher image loading and overlay callbacks without owning I/O.
func markdownImageWidget(block markdownBlock, props MarkdownProps, width float32, linkIndex *int) woxwidget.Widget {
	image, imageError := (*woxui.Image)(nil), ""
	if props.ResolveImage != nil {
		image, imageError = props.ResolveImage(block.image)
	}
	if image == nil {
		label := block.imageLabel
		if label == "" {
			label = block.image
		}
		if imageError != "" {
			label = imageError
		}
		return woxwidget.Container{Width: width, Height: 52, Padding: woxwidget.UniformInsets(10), Color: withAlpha(props.Theme.PreviewText, 10), Child: woxwidget.TextBlock{
			Value: label, Width: max(float32(0), width-20), Height: 32, MaxLines: 2, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.PreviewText,
		}}
	}
	if image.Width <= 0 || image.Height <= 0 {
		return woxwidget.Container{Width: width, Height: 32, Child: woxwidget.Text{Value: "Invalid Markdown image", Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ErrorText}}
	}
	availableWidth := max(float32(1), width)
	scale := availableWidth / float32(image.Width)
	drawWidth := float32(image.Width) * scale
	drawHeight := float32(image.Height) * scale
	content := woxwidget.Align{Width: width, Height: drawHeight, Horizontal: 0.5, Child: woxwidget.Image{Source: image, Width: drawWidth, Height: drawHeight}}
	if props.OnOpenImage == nil {
		return content
	}
	(*linkIndex)++
	id := fmt.Sprintf("%s-image-%d", props.ID, *linkIndex)
	return woxwidget.Semantics{
		Key: woxwidget.Key(id), AutomationID: id, Role: woxui.AccessibilityRoleImage, Label: block.imageLabel, Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate},
		OnAction: func(action woxui.AccessibilityAction, _ string) error {
			if action == woxui.AccessibilityActionActivate {
				props.OnOpenImage(block.image)
			}
			return nil
		},
		Child: woxwidget.Gesture{ID: id, OnTap: func() { props.OnOpenImage(block.image) }, Child: content},
	}
}
