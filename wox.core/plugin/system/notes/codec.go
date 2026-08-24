package notes

import (
	"fmt"
	"html"
	"net/url"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/uuid"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"

	"wox/common"
)

const (
	noteSchemaVersion   = 1
	documentVersion     = 1
	untitledNote        = "Untitled Note"
	conflictTitleSuffix = " (Sync Conflict)"
)

var noteMarkdownParser = goldmark.New(goldmark.WithExtensions(extension.GFM)).Parser()

// EmptyDocument creates the smallest valid editable document.
func EmptyDocument() common.NoteDocument {
	return common.NoteDocument{Version: documentVersion, Blocks: []common.NoteBlock{{ID: uuid.NewString(), Type: common.NoteBlockParagraph}}}
}

// NormalizeDocument repairs persisted or externally parsed documents into the current schema.
func NormalizeDocument(document common.NoteDocument) common.NoteDocument {
	document.Version = documentVersion
	if len(document.Blocks) == 0 {
		return EmptyDocument()
	}
	blocks := make([]common.NoteBlock, 0, len(document.Blocks))
	for _, source := range document.Blocks {
		block := source
		if block.ID == "" {
			block.ID = uuid.NewString()
		}
		if !validBlockType(block.Type) {
			block.Type = common.NoteBlockParagraph
		}
		if block.Type == common.NoteBlockParagraph && block.Table == nil && looksLikeGFMTable(block.Text) {
			if upgraded := tableFromMarkdown(block.Text); upgraded != nil {
				block.Type, block.Table, block.Spans = common.NoteBlockTable, upgraded, nil
			}
		}
		if block.Type != common.NoteBlockTask {
			block.Checked = false
		}
		if noteListBlockType(block.Type) {
			block.Indent = max(0, min(common.NoteMaximumIndent, block.Indent))
		} else {
			block.Indent = 0
		}
		if block.Type == common.NoteBlockTable {
			block.Table = normalizeTable(block.Table, block.Text)
			block.Text = tableToMarkdown(*block.Table)
			block.Spans = nil
			blocks = append(blocks, block)
			continue
		}
		block.Table = nil
		block.Spans = normalizeSpans(block.Text, block.Spans)
		if block.Type == common.NoteBlockCode && strings.Contains(block.Text, "\n") {
			for index, line := range strings.Split(strings.ReplaceAll(block.Text, "\r\n", "\n"), "\n") {
				lineBlock := block
				lineBlock.Text, lineBlock.Spans = line, nil
				if index > 0 {
					lineBlock.ID = uuid.NewString()
				}
				blocks = append(blocks, lineBlock)
			}
			continue
		}
		blocks = append(blocks, block)
	}
	document.Blocks = blocks
	return document
}

func noteListBlockType(blockType common.NoteBlockType) bool {
	return blockType == common.NoteBlockBullet || blockType == common.NoteBlockOrdered || blockType == common.NoteBlockTask
}

func validBlockType(blockType common.NoteBlockType) bool {
	switch blockType {
	case common.NoteBlockParagraph, common.NoteBlockHeading1, common.NoteBlockHeading2, common.NoteBlockHeading3,
		common.NoteBlockQuote, common.NoteBlockCode, common.NoteBlockBullet, common.NoteBlockOrdered,
		common.NoteBlockTask, common.NoteBlockDivider, common.NoteBlockTable:
		return true
	default:
		return false
	}
}

// normalizeSpans clamps persisted rune ranges and removes spans with no visible style.
func normalizeSpans(value string, spans []common.NoteSpan) []common.NoteSpan {
	limit := utf8.RuneCountInString(value)
	normalized := make([]common.NoteSpan, 0, len(spans))
	for _, span := range spans {
		span.Start = max(0, min(limit, span.Start))
		span.End = max(span.Start, min(limit, span.End))
		span.Link = strings.TrimSpace(span.Link)
		if span.Start == span.End || (!span.Bold && !span.Italic && !span.Underline && !span.Strike && !span.Code && span.Link == "") {
			continue
		}
		normalized = append(normalized, span)
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		if normalized[i].Start == normalized[j].Start {
			return normalized[i].End < normalized[j].End
		}
		return normalized[i].Start < normalized[j].Start
	})
	return normalized
}

// DocumentIsEmpty reports whether the user has not entered any note content.
func DocumentIsEmpty(document common.NoteDocument) bool {
	for _, block := range NormalizeDocument(document).Blocks {
		if block.Type == common.NoteBlockDivider || block.Type == common.NoteBlockTable || strings.TrimSpace(block.Text) != "" {
			return false
		}
	}
	return true
}

// NoteTitle returns the plain first line used throughout the Notes UI.
func NoteTitle(document common.NoteDocument) string {
	if len(document.Blocks) == 0 {
		return untitledNote
	}
	title := strings.TrimSpace(strings.SplitN(document.Blocks[0].Text, "\n", 2)[0])
	if title == "" && document.Blocks[0].Type == common.NoteBlockTable && document.Blocks[0].Table != nil {
		for _, row := range document.Blocks[0].Table.Rows {
			for _, cell := range row {
				if title = strings.TrimSpace(cell.Text); title != "" {
					return title
				}
			}
		}
	}
	if title == "" {
		return untitledNote
	}
	return title
}

// NotePreview returns a compact plain-text list preview.
func NotePreview(document common.NoteDocument) string {
	value := strings.TrimSpace(ToPlainText(document))
	value = strings.Join(strings.Fields(value), " ")
	if utf8.RuneCountInString(value) <= 160 {
		return value
	}
	return string([]rune(value)[:160]) + "…"
}

// AppendTitleSuffix preserves document formatting while distinguishing a conflict copy.
func AppendTitleSuffix(document common.NoteDocument, suffix string) common.NoteDocument {
	document = NormalizeDocument(document)
	block := &document.Blocks[0]
	first, rest, found := strings.Cut(block.Text, "\n")
	block.Text = first + suffix
	if found {
		block.Text += "\n" + rest
	}
	return document
}

// ToPlainText exports the note without inline formatting.
func ToPlainText(document common.NoteDocument) string {
	lines := make([]string, 0, len(document.Blocks))
	ordered := 1
	for _, block := range NormalizeDocument(document).Blocks {
		prefix := ""
		switch block.Type {
		case common.NoteBlockBullet:
			prefix = "• "
		case common.NoteBlockOrdered:
			prefix = fmt.Sprintf("%d. ", ordered)
			ordered++
		case common.NoteBlockTask:
			if block.Checked {
				prefix = "☑ "
			} else {
				prefix = "☐ "
			}
		case common.NoteBlockQuote:
			prefix = "> "
		case common.NoteBlockDivider:
			lines = append(lines, "────────")
			continue
		case common.NoteBlockTable:
			if block.Table != nil {
				lines = append(lines, tableToPlainText(*block.Table))
			}
			continue
		default:
			ordered = 1
		}
		if noteListBlockType(block.Type) {
			prefix = strings.Repeat("    ", block.Indent) + prefix
		}
		lines = append(lines, prefix+block.Text)
	}
	return strings.Join(lines, "\n")
}

// ToMarkdown exports every Notes block and supported inline style.
func ToMarkdown(document common.NoteDocument) string {
	document = NormalizeDocument(document)
	lines := make([]string, 0, len(document.Blocks))
	ordered := [common.NoteMaximumIndent + 1]int{1, 1, 1}
	for index := 0; index < len(document.Blocks); index++ {
		block := document.Blocks[index]
		indent := max(0, min(common.NoteMaximumIndent, block.Indent))
		value := markdownInline(block.Text, block.Spans)
		switch block.Type {
		case common.NoteBlockHeading1:
			value = "# " + value
		case common.NoteBlockHeading2:
			value = "## " + value
		case common.NoteBlockHeading3:
			value = "### " + value
		case common.NoteBlockQuote:
			value = "> " + value
		case common.NoteBlockCode:
			codeLines := []string{block.Text}
			for index+1 < len(document.Blocks) && document.Blocks[index+1].Type == common.NoteBlockCode {
				index++
				codeLines = append(codeLines, document.Blocks[index].Text)
			}
			value = "```\n" + strings.Join(codeLines, "\n") + "\n```"
		case common.NoteBlockBullet:
			value = "- " + value
		case common.NoteBlockOrdered:
			value = fmt.Sprintf("%d. %s", ordered[indent], value)
			ordered[indent]++
			for level := indent + 1; level < len(ordered); level++ {
				ordered[level] = 1
			}
		case common.NoteBlockTask:
			marker := "[ ]"
			if block.Checked {
				marker = "[x]"
			}
			value = "- " + marker + " " + value
		case common.NoteBlockDivider:
			value = "---"
		case common.NoteBlockTable:
			if block.Table != nil {
				value = tableToMarkdown(*block.Table)
			}
		default:
			ordered = [common.NoteMaximumIndent + 1]int{1, 1, 1}
		}
		if noteListBlockType(block.Type) {
			value = strings.Repeat("    ", indent) + value
		}
		lines = append(lines, value)
	}
	return strings.Join(lines, "\n")
}

type inlineStyle struct {
	bold, italic, underline, strike, code bool
	link                                  string
}

// markdownInline emits minimal Markdown markers for each contiguous style run.
func markdownInline(value string, spans []common.NoteSpan) string {
	return encodeInline(value, spans, func(text string, style inlineStyle) string {
		text = strings.ReplaceAll(text, "\\", "\\\\")
		if style.code {
			text = "`" + strings.ReplaceAll(text, "`", "\\`") + "`"
		}
		if style.bold {
			text = "**" + text + "**"
		}
		if style.italic {
			text = "*" + text + "*"
		}
		if style.underline {
			text = "<u>" + text + "</u>"
		}
		if style.strike {
			text = "~~" + text + "~~"
		}
		if style.link != "" {
			text = "[" + text + "](" + strings.ReplaceAll(style.link, ")", "%29") + ")"
		}
		return text
	})
}

// ToHTML exports semantic HTML with all user content escaped.
func ToHTML(document common.NoteDocument) string {
	var output strings.Builder
	output.WriteString("<!doctype html><meta charset=\"utf-8\"><article>\n")
	orderedOpen := false
	bulletOpen := false
	closeLists := func() {
		if orderedOpen {
			output.WriteString("</ol>\n")
			orderedOpen = false
		}
		if bulletOpen {
			output.WriteString("</ul>\n")
			bulletOpen = false
		}
	}
	for _, block := range NormalizeDocument(document).Blocks {
		value := htmlInline(block.Text, block.Spans)
		switch block.Type {
		case common.NoteBlockOrdered:
			if !orderedOpen {
				closeLists()
				output.WriteString("<ol>\n")
				orderedOpen = true
			}
			output.WriteString("<li>" + value + "</li>\n")
		case common.NoteBlockBullet, common.NoteBlockTask:
			if !bulletOpen {
				closeLists()
				output.WriteString("<ul>\n")
				bulletOpen = true
			}
			if block.Type == common.NoteBlockTask {
				output.WriteString(fmt.Sprintf("<li data-task=\"true\" data-checked=\"%t\">%s</li>\n", block.Checked, value))
			} else {
				output.WriteString("<li>" + value + "</li>\n")
			}
		default:
			closeLists()
			switch block.Type {
			case common.NoteBlockHeading1:
				output.WriteString("<h1>" + value + "</h1>\n")
			case common.NoteBlockHeading2:
				output.WriteString("<h2>" + value + "</h2>\n")
			case common.NoteBlockHeading3:
				output.WriteString("<h3>" + value + "</h3>\n")
			case common.NoteBlockQuote:
				output.WriteString("<blockquote>" + value + "</blockquote>\n")
			case common.NoteBlockCode:
				output.WriteString("<pre><code>" + html.EscapeString(block.Text) + "</code></pre>\n")
			case common.NoteBlockDivider:
				output.WriteString("<hr>\n")
			case common.NoteBlockTable:
				if block.Table != nil {
					output.WriteString(tableToHTML(*block.Table))
				}
			default:
				output.WriteString("<p>" + value + "</p>\n")
			}
		}
	}
	closeLists()
	output.WriteString("</article>\n")
	return output.String()
}

// htmlInline escapes user text before wrapping supported semantic inline tags.
func htmlInline(value string, spans []common.NoteSpan) string {
	return encodeInline(value, spans, func(text string, style inlineStyle) string {
		text = html.EscapeString(text)
		if style.code {
			text = "<code>" + text + "</code>"
		}
		if style.bold {
			text = "<strong>" + text + "</strong>"
		}
		if style.italic {
			text = "<em>" + text + "</em>"
		}
		if style.underline {
			text = "<u>" + text + "</u>"
		}
		if style.strike {
			text = "<s>" + text + "</s>"
		}
		if style.link != "" {
			if link := safeNoteLink(style.link); link != "" {
				text = "<a href=\"" + html.EscapeString(link) + "\">" + text + "</a>"
			}
		}
		return text
	})
}

// safeNoteLink allows only schemes that are safe to place in an exported href.
func safeNoteLink(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return ""
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "mailto":
		return parsed.String()
	default:
		return ""
	}
}

// encodeInline splits overlapping spans into contiguous runs for the selected codec.
func encodeInline(value string, spans []common.NoteSpan, encode func(string, inlineStyle) string) string {
	runes := []rune(value)
	spans = normalizeSpans(value, spans)
	var output strings.Builder
	for start := 0; start < len(runes); {
		style := styleAt(start, spans)
		end := start + 1
		for end < len(runes) && styleAt(end, spans) == style {
			end++
		}
		output.WriteString(encode(string(runes[start:end]), style))
		start = end
	}
	return output.String()
}

func styleAt(offset int, spans []common.NoteSpan) inlineStyle {
	style := inlineStyle{}
	for _, span := range spans {
		if span.Start > offset || span.End <= offset {
			continue
		}
		style.bold = style.bold || span.Bold
		style.italic = style.italic || span.Italic
		style.underline = style.underline || span.Underline
		style.strike = style.strike || span.Strike
		style.code = style.code || span.Code
		if span.Link != "" {
			style.link = span.Link
		}
	}
	return style
}

// ParseMarkdown converts GFM input into the Notes block model.
func ParseMarkdown(value string) common.NoteDocument {
	source := []byte(value)
	root := noteMarkdownParser.Parse(text.NewReader(source))
	document := common.NoteDocument{Version: documentVersion}
	for node := root.FirstChild(); node != nil; node = node.NextSibling() {
		document.Blocks = append(document.Blocks, markdownBlocks(node, source)...)
	}
	return NormalizeDocument(document)
}

// markdownBlocks maps one Goldmark block node into the line-oriented Notes model.
func markdownBlocks(node ast.Node, source []byte) []common.NoteBlock {
	block := common.NoteBlock{ID: uuid.NewString(), Type: common.NoteBlockParagraph}
	switch value := node.(type) {
	case *ast.Paragraph, *ast.TextBlock:
		block.Text, block.Spans = markdownInlineContent(node, source)
	case *ast.Heading:
		block.Text, block.Spans = markdownInlineContent(node, source)
		switch value.Level {
		case 1:
			block.Type = common.NoteBlockHeading1
		case 2:
			block.Type = common.NoteBlockHeading2
		default:
			block.Type = common.NoteBlockHeading3
		}
	case *ast.CodeBlock:
		block.Type, block.Text = common.NoteBlockCode, string(value.Text(source))
	case *ast.FencedCodeBlock:
		block.Type, block.Text = common.NoteBlockCode, strings.TrimSuffix(string(value.Text(source)), "\n")
	case *ast.Blockquote:
		blocks := markdownChildBlocks(value, source)
		for index := range blocks {
			blocks[index].Type = common.NoteBlockQuote
		}
		return blocks
	case *ast.List:
		return markdownListBlocks(value, source)
	case *ast.ThematicBreak:
		block.Type = common.NoteBlockDivider
	case *extast.Table:
		block.Type = common.NoteBlockTable
		block.Table = markdownTable(value, source)
		block.Text = tableToMarkdown(*block.Table)
	default:
		if node.Type() == ast.TypeBlock {
			return markdownChildBlocks(node, source)
		}
		return nil
	}
	return []common.NoteBlock{block}
}

func markdownChildBlocks(parent ast.Node, source []byte) []common.NoteBlock {
	var blocks []common.NoteBlock
	for child := parent.FirstChild(); child != nil; child = child.NextSibling() {
		blocks = append(blocks, markdownBlocks(child, source)...)
	}
	return blocks
}

// markdownListBlocks flattens list items while preserving ordered and task semantics.
func markdownListBlocks(list *ast.List, source []byte) []common.NoteBlock {
	return markdownListBlocksAtIndent(list, source, 0)
}

func markdownListBlocksAtIndent(list *ast.List, source []byte, indent int) []common.NoteBlock {
	blocks := make([]common.NoteBlock, 0, list.ChildCount())
	for itemNode := list.FirstChild(); itemNode != nil; itemNode = itemNode.NextSibling() {
		item, ok := itemNode.(*ast.ListItem)
		if !ok {
			continue
		}
		for childNode := item.FirstChild(); childNode != nil; childNode = childNode.NextSibling() {
			if nested, ok := childNode.(*ast.List); ok {
				blocks = append(blocks, markdownListBlocksAtIndent(nested, source, min(common.NoteMaximumIndent, indent+1))...)
				continue
			}
			for _, child := range markdownBlocks(childNode, source) {
				child.Type = common.NoteBlockBullet
				child.Indent = indent
				if list.IsOrdered() {
					child.Type = common.NoteBlockOrdered
				}
				if task, checked := markdownTask(child.Text); task {
					child.Type, child.Checked = common.NoteBlockTask, checked
					child.Text = strings.TrimSpace(child.Text[3:])
					child.Spans = nil
				}
				blocks = append(blocks, child)
			}
		}
	}
	return blocks
}

func markdownTask(value string) (bool, bool) {
	value = strings.TrimSpace(value)
	if len(value) < 3 || value[0] != '[' || value[2] != ']' {
		return false, false
	}
	return value[1] == ' ' || value[1] == 'x' || value[1] == 'X', value[1] == 'x' || value[1] == 'X'
}

// markdownInlineContent collects plain text and rune-based styles from inline AST nodes.
func markdownInlineContent(parent ast.Node, source []byte) (string, []common.NoteSpan) {
	var output strings.Builder
	var spans []common.NoteSpan
	var visit func(ast.Node, inlineStyle)
	visit = func(node ast.Node, style inlineStyle) {
		current := style
		switch value := node.(type) {
		case *ast.Text:
			appendParsedText(&output, &spans, string(value.Segment.Value(source)), current)
			if value.HardLineBreak() || value.SoftLineBreak() {
				output.WriteRune('\n')
			}
			return
		case *ast.String:
			appendParsedText(&output, &spans, string(value.Value), current)
			return
		case *ast.Emphasis:
			if value.Level >= 2 {
				current.bold = true
			} else {
				current.italic = true
			}
		case *ast.CodeSpan:
			current.code = true
		case *ast.Link:
			current.link = string(value.Destination)
		case *extast.Strikethrough:
			current.strike = true
		case *extast.TaskCheckBox:
			if value.IsChecked {
				output.WriteString("[x] ")
			} else {
				output.WriteString("[ ] ")
			}
			return
		}
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			visit(child, current)
		}
	}
	for child := parent.FirstChild(); child != nil; child = child.NextSibling() {
		visit(child, inlineStyle{})
	}
	return output.String(), normalizeSpans(output.String(), spans)
}

func appendParsedText(output *strings.Builder, spans *[]common.NoteSpan, value string, style inlineStyle) {
	start := utf8.RuneCountInString(output.String())
	output.WriteString(value)
	end := start + utf8.RuneCountInString(value)
	if start == end || style == (inlineStyle{}) {
		return
	}
	*spans = append(*spans, common.NoteSpan{
		Start: start, End: end, Bold: style.bold, Italic: style.italic, Underline: style.underline,
		Strike: style.strike, Code: style.code, Link: style.link,
	})
}

// EmptyNoteTable creates the default 3x2 editable table (one header row and one body row).
func EmptyNoteTable() common.NoteTable {
	row := func() []common.NoteTableCell {
		return []common.NoteTableCell{{}, {}, {}}
	}
	return common.NoteTable{HeaderRows: 1, Rows: [][]common.NoteTableCell{row(), row()}}
}

// ParseClipboard turns pasted Markdown, HTML, or TSV into a Notes document.
func ParseClipboard(value string) common.NoteDocument {
	if looksLikeHTML(value) {
		return ParseHTML(value)
	}
	if looksLikeTSV(value) {
		return NormalizeDocument(common.NoteDocument{Version: documentVersion, Blocks: []common.NoteBlock{{
			ID: uuid.NewString(), Type: common.NoteBlockTable, Table: tableFromTSV(value),
		}}})
	}
	return ParseMarkdown(value)
}

// ParseHTML converts Notes export HTML and pasted table markup into the block model.
func ParseHTML(value string) common.NoteDocument {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(value))
	if err != nil {
		return ParseMarkdown(value)
	}
	root := document.Find("article")
	if root.Length() == 0 {
		root = document.Selection
	}
	parsed := common.NoteDocument{Version: documentVersion}
	root.Children().Each(func(_ int, node *goquery.Selection) {
		parsed.Blocks = append(parsed.Blocks, htmlBlocks(node)...)
	})
	if len(parsed.Blocks) == 0 {
		if table := htmlTable(document.Find("table").First()); table != nil {
			parsed.Blocks = append(parsed.Blocks, common.NoteBlock{ID: uuid.NewString(), Type: common.NoteBlockTable, Table: table})
		}
	}
	return NormalizeDocument(parsed)
}

func htmlBlocks(node *goquery.Selection) []common.NoteBlock {
	block := common.NoteBlock{ID: uuid.NewString(), Type: common.NoteBlockParagraph}
	switch strings.ToLower(goquery.NodeName(node)) {
	case "h1":
		block.Type = common.NoteBlockHeading1
		block.Text, block.Spans = htmlSelectionText(node)
	case "h2":
		block.Type = common.NoteBlockHeading2
		block.Text, block.Spans = htmlSelectionText(node)
	case "h3", "h4", "h5", "h6":
		block.Type = common.NoteBlockHeading3
		block.Text, block.Spans = htmlSelectionText(node)
	case "p":
		block.Text, block.Spans = htmlSelectionText(node)
	case "blockquote":
		block.Type = common.NoteBlockQuote
		block.Text, block.Spans = htmlSelectionText(node)
	case "pre":
		block.Type, block.Text = common.NoteBlockCode, node.Text()
	case "hr":
		block.Type = common.NoteBlockDivider
	case "ul", "ol":
		return htmlListBlocks(node, strings.ToLower(goquery.NodeName(node)) == "ol", 0)
	case "table":
		if table := htmlTable(node); table != nil {
			block.Type, block.Table = common.NoteBlockTable, table
			break
		}
		return nil
	default:
		var blocks []common.NoteBlock
		node.Children().Each(func(_ int, child *goquery.Selection) {
			blocks = append(blocks, htmlBlocks(child)...)
		})
		return blocks
	}
	return []common.NoteBlock{block}
}

func htmlListBlocks(list *goquery.Selection, ordered bool, indent int) []common.NoteBlock {
	var blocks []common.NoteBlock
	list.ChildrenFiltered("li").Each(func(_ int, item *goquery.Selection) {
		text, spans := htmlSelectionText(item)
		block := common.NoteBlock{ID: uuid.NewString(), Type: common.NoteBlockBullet, Text: text, Spans: spans, Indent: indent}
		if ordered {
			block.Type = common.NoteBlockOrdered
		}
		if task, _ := item.Attr("data-task"); task == "true" {
			block.Type = common.NoteBlockTask
			block.Checked = item.AttrOr("data-checked", "") == "true"
		}
		blocks = append(blocks, block)
		item.ChildrenFiltered("ul,ol").Each(func(_ int, nested *goquery.Selection) {
			blocks = append(blocks, htmlListBlocks(nested, goquery.NodeName(nested) == "ol", min(common.NoteMaximumIndent, indent+1))...)
		})
	})
	return blocks
}

func htmlSelectionText(node *goquery.Selection) (string, []common.NoteSpan) {
	clone := node.Clone()
	clone.Find("ul,ol,table").Remove()
	return markdownInlineContentFromPlain(strings.TrimSpace(clone.Text()))
}

func markdownInlineContentFromPlain(value string) (string, []common.NoteSpan) {
	return value, nil
}

func htmlTable(node *goquery.Selection) *common.NoteTable {
	if node == nil || node.Length() == 0 {
		return nil
	}
	table := common.NoteTable{}
	node.Find("tr").Each(func(_ int, row *goquery.Selection) {
		cells := make([]common.NoteTableCell, 0)
		row.ChildrenFiltered("th,td").Each(func(_ int, cell *goquery.Selection) {
			text, spans := markdownInlineContentFromPlain(strings.TrimSpace(cell.Text()))
			if cell.Is("th") && table.HeaderRows == 0 {
				table.HeaderRows = 1
			}
			cells = append(cells, common.NoteTableCell{Text: text, Spans: spans})
		})
		if len(cells) > 0 {
			table.Rows = append(table.Rows, cells)
		}
	})
	if len(table.Rows) == 0 {
		return nil
	}
	return &table
}

func markdownTable(table *extast.Table, source []byte) *common.NoteTable {
	data := common.NoteTable{}
	for row := table.FirstChild(); row != nil; row = row.NextSibling() {
		_, header := row.(*extast.TableHeader)
		if header {
			data.HeaderRows++
		}
		cells := make([]common.NoteTableCell, 0)
		for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
			text, spans := markdownInlineContent(cell, source)
			cells = append(cells, common.NoteTableCell{Text: strings.TrimSpace(text), Spans: normalizeSpans(strings.TrimSpace(text), spans)})
		}
		if len(cells) > 0 {
			data.Rows = append(data.Rows, cells)
		}
	}
	return &data
}

func normalizeTable(table *common.NoteTable, fallback string) *common.NoteTable {
	if table == nil || len(table.Rows) == 0 {
		if parsed := tableFromMarkdown(fallback); parsed != nil {
			table = parsed
		} else {
			empty := EmptyNoteTable()
			return &empty
		}
	}
	columns := 1
	for _, row := range table.Rows {
		columns = max(columns, len(row))
	}
	normalized := common.NoteTable{HeaderRows: max(0, min(len(table.Rows), table.HeaderRows)), Rows: make([][]common.NoteTableCell, 0, len(table.Rows))}
	for _, row := range table.Rows {
		cells := make([]common.NoteTableCell, columns)
		for index := 0; index < columns; index++ {
			if index < len(row) {
				cells[index] = row[index]
				cells[index].Spans = normalizeSpans(cells[index].Text, cells[index].Spans)
			}
		}
		normalized.Rows = append(normalized.Rows, cells)
	}
	return &normalized
}

func tableFromMarkdown(value string) *common.NoteTable {
	document := common.NoteDocument{Version: documentVersion}
	source := []byte(value)
	root := noteMarkdownParser.Parse(text.NewReader(source))
	for node := root.FirstChild(); node != nil; node = node.NextSibling() {
		document.Blocks = append(document.Blocks, markdownBlocks(node, source)...)
	}
	if len(document.Blocks) == 1 && document.Blocks[0].Type == common.NoteBlockTable && document.Blocks[0].Table != nil {
		return document.Blocks[0].Table
	}
	return nil
}

func tableToMarkdown(table common.NoteTable) string {
	if len(table.Rows) == 0 {
		return ""
	}
	columns := 1
	for _, row := range table.Rows {
		columns = max(columns, len(row))
	}
	lines := make([]string, 0, len(table.Rows)+1)
	for index, row := range table.Rows {
		lines = append(lines, markdownTableRow(row, columns))
		if index+1 == max(1, table.HeaderRows) || (table.HeaderRows == 0 && index == 0) {
			lines = append(lines, markdownTableSeparator(columns))
		}
	}
	return strings.Join(lines, "\n")
}

func markdownTableRow(row []common.NoteTableCell, columns int) string {
	cells := make([]string, columns)
	for index := 0; index < columns; index++ {
		value := ""
		if index < len(row) {
			value = markdownInline(row[index].Text, row[index].Spans)
		}
		cells[index] = " " + strings.ReplaceAll(value, "|", "\\|") + " "
	}
	return "|" + strings.Join(cells, "|") + "|"
}

func markdownTableSeparator(columns int) string {
	cells := make([]string, columns)
	for index := range cells {
		cells[index] = " --- "
	}
	return "|" + strings.Join(cells, "|") + "|"
}

func tableToHTML(table common.NoteTable) string {
	var output strings.Builder
	output.WriteString("<table>\n")
	for index, row := range table.Rows {
		tag := "td"
		if index < table.HeaderRows {
			if index == 0 {
				output.WriteString("<thead>\n")
			}
			tag = "th"
		} else if index == table.HeaderRows {
			if table.HeaderRows > 0 {
				output.WriteString("</thead>\n")
			}
			output.WriteString("<tbody>\n")
		}
		output.WriteString("<tr>")
		for _, cell := range row {
			output.WriteString("<" + tag + ">" + htmlInline(cell.Text, cell.Spans) + "</" + tag + ">")
		}
		output.WriteString("</tr>\n")
	}
	if table.HeaderRows > 0 && table.HeaderRows < len(table.Rows) {
		output.WriteString("</tbody>\n")
	} else if table.HeaderRows > 0 {
		output.WriteString("</thead>\n")
	}
	output.WriteString("</table>\n")
	return output.String()
}

func tableToPlainText(table common.NoteTable) string {
	lines := make([]string, 0, len(table.Rows))
	for _, row := range table.Rows {
		cells := make([]string, 0, len(row))
		for _, cell := range row {
			cells = append(cells, cell.Text)
		}
		lines = append(lines, strings.Join(cells, "\t"))
	}
	return strings.Join(lines, "\n")
}

func tableFromTSV(value string) *common.NoteTable {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	table := common.NoteTable{HeaderRows: 1}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" && len(table.Rows) == 0 {
			continue
		}
		parts := strings.Split(line, "\t")
		row := make([]common.NoteTableCell, 0, len(parts))
		for _, part := range parts {
			row = append(row, common.NoteTableCell{Text: part})
		}
		table.Rows = append(table.Rows, row)
	}
	return &table
}

func looksLikeGFMTable(value string) bool {
	lines := nonEmptyLines(value)
	return len(lines) >= 2 && strings.Contains(lines[0], "|") && isTableSeparator(lines[1])
}

func looksLikeHTML(value string) bool {
	trimmed := strings.TrimSpace(value)
	return strings.Contains(strings.ToLower(trimmed), "<table") || strings.Contains(strings.ToLower(trimmed), "<article")
}

func looksLikeTSV(value string) bool {
	lines := nonEmptyLines(value)
	if len(lines) < 2 {
		return false
	}
	columns := 0
	for _, line := range lines {
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			return false
		}
		if columns == 0 {
			columns = len(parts)
			continue
		}
		if len(parts) != columns {
			return false
		}
	}
	return columns >= 2
}

func isTableSeparator(line string) bool {
	line = strings.TrimSpace(line)
	if !strings.Contains(line, "|") && !strings.Contains(line, "-") {
		return false
	}
	line = strings.Trim(line, "|")
	for _, cell := range strings.Split(line, "|") {
		cell = strings.TrimSpace(cell)
		if cell == "" {
			continue
		}
		cell = strings.Trim(cell, ":")
		if strings.Trim(cell, "-") != "" {
			return false
		}
	}
	return strings.Contains(line, "-")
}

func nonEmptyLines(value string) []string {
	var lines []string
	for _, line := range strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
