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
	markdownListItemLine    = regexp.MustCompile(`^(\s*)([*+-]|[0-9]{1,9}[.)])(\s+|$)`)
	markdownFenceLine       = regexp.MustCompile("^[ \t]{0,3}(`{3,}|~{3,})")
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
	marker  string
	label   string
	task    bool
	checked bool
	blocks  []markdownBlock
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
	// Window enables pointer hit-testing so rendered text can be selected and copied.
	Window       *woxui.Window
	ResolveImage func(source string) (*woxui.Image, string)
	OnOpenImage  func(source string)
	OnOpenLink   func(target string)
	// InlineTrailing appends a control to the final top-level inline paragraph.
	InlineTrailing woxwidget.Widget
	// strikeText paints every inline run as completed checklist text.
	strikeText bool
}

// ParseMarkdown parses CommonMark with the GitHub-flavored extensions used by Wox previews.
func ParseMarkdown(value string) MarkdownDocument {
	source := []byte(normalizeMarkdownListIndent(normalizeMarkdownImages(value)))
	document := markdownParser.Parse(text.NewReader(source))
	return MarkdownDocument{blocks: parseMarkdownBlocks(document, source, 0)}
}

// WoxMarkdown builds a native Markdown widget tree without a browser surface.
func WoxMarkdown(props MarkdownProps) woxwidget.Widget {
	width := max(float32(0), props.Width)
	linkIndex := 0
	blockGap := props.BlockGap
	if blockGap <= 0 {
		blockGap = 12
	}
	textIndex := 0
	blocks := renderMarkdownBlocks(props.Document.blocks, props, width, &linkIndex, &textIndex)
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

type markdownListIndentLevel struct {
	indent      int
	childIndent int
}

// normalizeMarkdownListIndent lifts 2-space nested ordered items into the parent item.
// CommonMark treats "  1." after "4. " as a sibling because "4. " occupies 3 columns.
func normalizeMarkdownListIndent(value string) string {
	lines := strings.Split(value, "\n")
	normalized := make([]string, 0, len(lines))
	levels := make([]markdownListIndentLevel, 0, 4)
	inFence := false
	for _, line := range lines {
		if markdownFenceLine.MatchString(line) {
			inFence = !inFence
			levels = levels[:0]
			normalized = append(normalized, line)
			continue
		}
		if inFence {
			normalized = append(normalized, line)
			continue
		}
		indent, childIndent, isList := markdownListItemColumns(line)
		if !isList {
			if strings.TrimSpace(line) == "" {
				normalized = append(normalized, line)
				continue
			}
			if markdownLeadingColumns(line) == 0 {
				levels = levels[:0]
			}
			normalized = append(normalized, line)
			continue
		}
		for len(levels) > 0 && indent < levels[len(levels)-1].indent {
			levels = levels[:len(levels)-1]
		}
		if len(levels) > 0 {
			parent := levels[len(levels)-1]
			if indent > parent.indent && indent < parent.childIndent {
				line = strings.Repeat(" ", parent.childIndent-indent) + line
				indent = parent.childIndent
			}
		}
		if len(levels) == 0 || indent == 0 {
			levels = []markdownListIndentLevel{{indent: indent, childIndent: childIndent}}
		} else if indent == levels[len(levels)-1].indent {
			levels[len(levels)-1] = markdownListIndentLevel{indent: indent, childIndent: childIndent}
		} else if indent >= levels[len(levels)-1].childIndent {
			levels = append(levels, markdownListIndentLevel{indent: indent, childIndent: childIndent})
		}
		normalized = append(normalized, line)
	}
	return strings.Join(normalized, "\n")
}

func markdownListItemColumns(line string) (indent, childIndent int, ok bool) {
	parts := markdownListItemLine.FindStringSubmatch(line)
	if len(parts) != 4 {
		return 0, 0, false
	}
	indent = markdownLeadingColumns(parts[1])
	markerWidth := len(parts[2])
	spaceAfter := len(parts[3])
	if spaceAfter == 0 {
		spaceAfter = 1
	} else if spaceAfter > 4 {
		spaceAfter = 4
	}
	return indent, indent + markerWidth + spaceAfter, true
}

func markdownLeadingColumns(value string) int {
	columns := 0
	for _, r := range value {
		switch r {
		case ' ':
			columns++
		case '\t':
			columns += 4 - columns%4
		default:
			return columns
		}
	}
	return columns
}

// parseMarkdownBlocks converts Goldmark nodes into a renderer-owned immutable block model.
func parseMarkdownBlocks(parent ast.Node, source []byte, orderedDepth int) []markdownBlock {
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
			blocks = append(blocks, markdownBlock{kind: markdownQuote, children: parseMarkdownBlocks(value, source, orderedDepth)})
		case *ast.List:
			blocks = append(blocks, parseMarkdownList(value, source, orderedDepth))
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
				blocks = append(blocks, parseMarkdownBlocks(value, source, orderedDepth)...)
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
func parseMarkdownList(list *ast.List, source []byte, orderedDepth int) markdownBlock {
	block := markdownBlock{kind: markdownList}
	index := list.Start
	if index <= 0 {
		index = 1
	}
	childDepth := orderedDepth
	if list.IsOrdered() {
		childDepth++
	}
	for child := list.FirstChild(); child != nil; child = child.NextSibling() {
		item, ok := child.(*ast.ListItem)
		if !ok {
			continue
		}
		marker := "•"
		if list.IsOrdered() {
			marker = markdownOrderedMarker(orderedDepth, index)
			index++
		}
		task, checked := markdownTaskState(item)
		if task {
			marker = ""
		}
		blocks := parseMarkdownBlocks(item, source, childDepth)
		block.items = append(block.items, markdownListItem{marker: marker, label: strings.TrimSpace(markdownPlainText(item, source)), task: task, checked: checked, blocks: blocks})
	}
	return block
}

// markdownOrderedMarker uses outline sequences so nested numbered lists stay visually distinct: 1. / a. / i. / A.
func markdownOrderedMarker(depth, index int) string {
	if index <= 0 {
		index = 1
	}
	switch depth % 4 {
	case 1:
		return markdownAlphaMarker(index, false) + "."
	case 2:
		return markdownRomanMarker(index) + "."
	case 3:
		return markdownAlphaMarker(index, true) + "."
	default:
		return fmt.Sprintf("%d.", index)
	}
}

// markdownAlphaMarker converts a 1-based index into a spreadsheet-style letter sequence.
func markdownAlphaMarker(index int, upper bool) string {
	letters := make([]byte, 0, 4)
	for index > 0 {
		index--
		letter := byte('a' + index%26)
		if upper {
			letter = byte('A' + index%26)
		}
		letters = append(letters, letter)
		index /= 26
	}
	for i, j := 0, len(letters)-1; i < j; i, j = i+1, j-1 {
		letters[i], letters[j] = letters[j], letters[i]
	}
	return string(letters)
}

// markdownRomanMarker converts a 1-based index into lowercase Roman numerals.
func markdownRomanMarker(index int) string {
	values := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	symbols := []string{"m", "cm", "d", "cd", "c", "xc", "l", "xl", "x", "ix", "v", "iv", "i"}
	var builder strings.Builder
	for i, value := range values {
		for index >= value {
			builder.WriteString(symbols[i])
			index -= value
		}
	}
	return builder.String()
}

// markdownTaskState reads the GFM task node without leaking emoji markers into text runs.
func markdownTaskState(item *ast.ListItem) (bool, bool) {
	firstBlock := item.FirstChild()
	if firstBlock == nil {
		return false, false
	}
	for node := firstBlock.FirstChild(); node != nil; node = node.NextSibling() {
		if checkbox, ok := node.(*extast.TaskCheckBox); ok {
			return true, checkbox.IsChecked
		}
	}
	return false, false
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
func renderMarkdownBlocks(blocks []markdownBlock, props MarkdownProps, width float32, linkIndex, textIndex *int) []woxwidget.Widget {
	widgets := make([]woxwidget.Widget, 0, len(blocks))
	for _, block := range blocks {
		widgets = append(widgets, renderMarkdownBlock(block, props, width, linkIndex, textIndex))
	}
	return widgets
}

// renderMarkdownBlock picks the simplest native surface for one block.
func renderMarkdownBlock(block markdownBlock, props MarkdownProps, width float32, linkIndex, textIndex *int) woxwidget.Widget {
	fontSize := markdownFontSize(props)
	switch block.kind {
	case markdownHeading:
		return markdownRunsWidget(block.runs, props, width, fontSize, linkIndex, textIndex)
	case markdownCode:
		return markdownCodeWidget(block, props, width, textIndex)
	case markdownQuote:
		innerWidth := max(float32(0), width-documentQuoteWidth(fontSize))
		return documentQuote(width, fontSize, DocumentListMarkerColor, woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, Children: renderMarkdownBlocks(block.children, props, innerWidth, linkIndex, textIndex)})
	case markdownList:
		return markdownListWidget(block, props, width, linkIndex, textIndex)
	case markdownRule:
		return documentHorizontalRule(width, props.Theme.PreviewSplit)
	case markdownTable:
		return markdownTableWidget(block.table, props, width, textIndex)
	case markdownImage:
		return markdownImageWidget(block, props, width, linkIndex)
	default:
		return markdownRunsWidget(block.runs, props, width, fontSize, linkIndex, textIndex)
	}
}

func markdownFontSize(props MarkdownProps) float32 {
	if props.FontSize > 0 {
		return props.FontSize
	}
	return 12
}

// markdownRunsWidget wraps inline text while retaining native link actions.
func markdownRunsWidget(runs []markdownRun, props MarkdownProps, width, fontSize float32, linkIndex, textIndex *int) woxwidget.Widget {
	if field := markdownSelectableRuns(runs, props, width, fontSize, textIndex); field != nil {
		return field
	}
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
					Child: woxwidget.Text{Value: token, Style: woxui.TextStyle{Size: max(float32(10), fontSize-1)}, Color: color, Strike: run.style.strike || props.strikeText},
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
			// The caret color can match body text; use the document accent to keep links recognizable.
			link := woxwidget.Gesture{ID: id, Cursor: woxui.PointerCursorHand, OnTap: func() { props.OnOpenLink(target) }, Child: woxwidget.Text{Value: label, Style: style, Color: DocumentListMarkerColor, Underline: true, Strike: run.style.strike || props.strikeText}}
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
			children = append(children, woxwidget.Text{Value: token, Style: style, Color: color, Strike: run.style.strike || props.strikeText})
		}
	}
	return woxwidget.Wrap{Gap: 0, RunGap: max(float32(3), fontSize*0.25), Children: children}
}

type markdownLinkRange struct {
	start  int
	end    int
	target string
}

// markdownSelectableRuns uses a read-only text field so users can drag-select and copy rendered text.
func markdownSelectableRuns(runs []markdownRun, props MarkdownProps, width, fontSize float32, textIndex *int) woxwidget.Widget {
	if props.Window == nil || props.InlineTrailing != nil || textIndex == nil {
		return nil
	}
	value, rich, links := markdownRunsContent(runs, fontSize, props.Theme, props.strikeText)
	if strings.TrimSpace(value) == "" {
		return nil
	}
	*textIndex++
	return markdownSelectableText(fmt.Sprintf("%s-text-%d", markdownSelectableID(props.ID), *textIndex), value, rich, links, props, width, fontSize)
}

func markdownSelectableID(id string) string {
	if strings.TrimSpace(id) == "" {
		return "markdown"
	}
	return id
}

func markdownRunsContent(runs []markdownRun, fontSize float32, theme Theme, strikeText bool) (string, []TextFieldRichRun, []markdownLinkRange) {
	var builder strings.Builder
	rich := make([]TextFieldRichRun, 0, len(runs))
	links := make([]markdownLinkRange, 0)
	offset := 0
	for _, run := range runs {
		if run.text == "" {
			continue
		}
		start := offset
		builder.WriteString(run.text)
		offset += utf8.RuneCountInString(run.text)
		style := woxui.TextStyle{Size: fontSize}
		if run.style.bold {
			style.Weight = woxui.FontWeightSemibold
		}
		color := theme.PreviewText
		if run.style.strike {
			color = withAlpha(color, 150)
		}
		underline := false
		if run.style.link != "" {
			color = DocumentListMarkerColor
			underline = true
			links = append(links, markdownLinkRange{start: start, end: offset, target: run.style.link})
		}
		if run.style.code {
			style.Size = max(float32(10), fontSize-1)
		}
		background := woxui.Color{}
		if run.style.code {
			background = withAlpha(theme.PreviewText, 18)
		}
		rich = append(rich, TextFieldRichRun{
			Start: start, End: offset, Style: style, Color: color, Underline: underline, Strike: run.style.strike || strikeText, Background: background,
		})
	}
	return builder.String(), rich, links
}

func markdownLinkAt(links []markdownLinkRange, offset int) string {
	for _, link := range links {
		if offset >= link.start && offset < link.end {
			return link.target
		}
	}
	return ""
}

func markdownSelectableText(id, value string, rich []TextFieldRichRun, links []markdownLinkRange, props MarkdownProps, width, fontSize float32) woxwidget.Widget {
	lineHeight := markdownSelectableLineHeight(fontSize)
	style := woxui.TextStyle{Size: fontSize}
	lines := textFieldRichLines(value, props.Window, style, width, true, rich)
	height := max(lineHeight, float32(max(1, len(lines)))*lineHeight+1)
	var onTapOffset func(int) bool
	var cursorAt func(int) woxui.PointerCursor
	if len(links) > 0 {
		cursorAt = func(offset int) woxui.PointerCursor { return markdownCursorAt(links, offset) }
		if props.OnOpenLink != nil {
			onTapOffset = func(offset int) bool {
				target := markdownLinkAt(links, offset)
				if target == "" {
					return false
				}
				props.OnOpenLink(target)
				return true
			}
		}
	}
	return WoxTextField(TextFieldProps{
		ID: id, Label: value, Width: width, Height: height, Padding: woxwidget.Insets{Bottom: 1},
		Transparent: true, DisableHover: true, Style: style, RichRuns: rich, LineHeight: lineHeight,
		TextColor: props.Theme.PreviewText, Value: value, ReadOnly: true, MaxLines: max(8, len(lines)+4),
		Window: props.Window, Theme: props.Theme, OnTapOffset: onTapOffset, CursorAtOffset: cursorAt,
	})
}

func markdownCursorAt(links []markdownLinkRange, offset int) woxui.PointerCursor {
	if markdownLinkAt(links, offset) != "" {
		return woxui.PointerCursorHand
	}
	return woxui.PointerCursorText
}

func markdownSelectableLineHeight(fontSize float32) float32 {
	return max(float32(15), fontSize+3)
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
func markdownCodeWidget(block markdownBlock, props MarkdownProps, width float32, textIndex *int) woxwidget.Widget {
	innerWidth := max(float32(0), width-20)
	code := strings.TrimSuffix(block.runs[0].text, "\n")
	fontSize := markdownFontSize(props)
	style := woxui.TextStyle{Size: max(float32(10), fontSize-1)}
	children := make([]woxwidget.Widget, 0, 2)
	if block.language != "" {
		children = append(children, woxwidget.Text{Value: block.language, Style: woxui.TextStyle{Size: max(float32(9), fontSize-2), Weight: woxui.FontWeightSemibold}, Color: withAlpha(props.Theme.PreviewText, 180)})
	}
	if props.Window != nil && textIndex != nil {
		*textIndex++
		rich := []TextFieldRichRun{{Start: 0, End: utf8.RuneCountInString(code), Style: style, Color: props.Theme.PreviewText}}
		children = append(children, markdownSelectableText(fmt.Sprintf("%s-code-%d", markdownSelectableID(props.ID), *textIndex), code, rich, nil, props, innerWidth, style.Size))
	} else {
		layout := woxwidget.LayoutTextBlock(props.Window, code, style, innerWidth, 0, 17)
		children = append(children, woxwidget.TextBlock{Value: code, Width: innerWidth, Height: layout.Size.Height, Layout: &layout, Style: style, LineHeight: 17, Color: props.Theme.PreviewText})
	}
	return woxwidget.Container{
		Width: width, Padding: woxwidget.UniformInsets(10), Radius: 5, Color: withAlpha(props.Theme.PreviewText, 14), BorderColor: withAlpha(props.Theme.PreviewSplit, 90), BorderWidth: 1,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 7, Children: children},
	}
}

// markdownListMarkerWidth keeps the marker column wide enough for roman or multi-letter labels.
func markdownListMarkerWidth(marker string) float32 {
	if marker == "" {
		return float32(28)
	}
	return max(float32(28), float32(len([]rune(marker)))*8+8)
}

// markdownListWidget preserves nested blocks inside one row per list item.
func markdownListWidget(block markdownBlock, props MarkdownProps, width float32, linkIndex, textIndex *int) woxwidget.Widget {
	rows := make([]woxwidget.Widget, 0, len(block.items))
	for _, item := range block.items {
		markerWidth := markdownListMarkerWidth(item.marker)
		marker := woxwidget.Widget(woxwidget.Text{Value: item.marker, Style: woxui.TextStyle{Size: markdownFontSize(props), Weight: woxui.FontWeightSemibold}, Color: DocumentListMarkerColor})
		itemProps := props
		if item.task {
			fontSize := markdownFontSize(props)
			markerWidth = documentCheckboxWidth(fontSize) + 4
			marker = woxwidget.Semantics{Role: woxui.AccessibilityRoleCheckBox, Label: item.label, Checked: item.checked, Disabled: true, Child: documentCheckbox(fontSize, 18, DocumentListMarkerColor, item.checked)}
			if item.checked {
				itemProps.Theme.PreviewText = props.Theme.ResultSubtitle
				itemProps.strikeText = true
			}
		}
		rows = append(rows, woxwidget.Flex{Axis: woxwidget.Horizontal, CrossAxisAlignment: woxwidget.CrossAxisStart, Children: []woxwidget.Widget{
			woxwidget.Container{Width: markerWidth, Padding: woxwidget.Insets{Top: 1}, Child: marker},
			woxwidget.Container{Width: max(float32(0), width-markerWidth), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 7, Children: renderMarkdownBlocks(item.blocks, itemProps, max(float32(0), width-markerWidth), linkIndex, textIndex)}},
		}})
	}
	return woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 7, Children: rows}
}

// markdownTableWidget keeps wide GFM tables horizontally scrollable.
func markdownTableWidget(table markdownTableData, props MarkdownProps, width float32, textIndex *int) woxwidget.Widget {
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
			cellWidthInner := max(float32(0), cellWidth-16)
			var cellText woxwidget.Widget = woxwidget.TextBlock{
				Value: value, Width: cellWidthInner, Height: 18, LineHeight: 18, MaxLines: 1, AlignmentY: 0.5, Style: woxui.TextStyle{Size: markdownFontSize(props), Weight: weight}, Color: props.Theme.PreviewText,
			}
			if props.Window != nil && textIndex != nil && strings.TrimSpace(value) != "" {
				*textIndex++
				style := woxui.TextStyle{Size: markdownFontSize(props), Weight: weight}
				rich := []TextFieldRichRun{{Start: 0, End: utf8.RuneCountInString(value), Style: style, Color: props.Theme.PreviewText}}
				cellText = markdownSelectableText(fmt.Sprintf("%s-cell-%d", markdownSelectableID(props.ID), *textIndex), value, rich, nil, props, cellWidthInner, style.Size)
			}
			cells = append(cells, WoxTableGridCell(TableGridCellProps{
				Width: cellWidth, Height: 38, Color: background, Border: withAlpha(props.Theme.PreviewSplit, 100),
				Trailing: column < columns-1, Bottom: rowIndex < len(table.rows)-1,
				Padding: woxwidget.Insets{Left: 8, Right: 8},
				Child:   woxwidget.Align{Width: cellWidthInner, Height: 38, Vertical: 0.5, Child: cellText},
			}))
		}
		rows = append(rows, woxwidget.Flex{Axis: woxwidget.Horizontal, Children: cells})
	}
	height := float32(len(rows)) * 38
	return WoxTableGridFrame(width, height, withAlpha(props.Theme.PreviewSplit, 100), woxwidget.ScrollView{
		Width: width, Height: height, ContentWidth: contentWidth, Horizontal: true,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
	})
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
