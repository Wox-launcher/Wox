package component

import (
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"wox/common"
	woxui "wox/ui/runtime"
)

// NoteBlockRange maps one document block onto the editor's plain-text backing value.
type NoteBlockRange struct {
	Block     int
	Start     int
	Marker    int
	TextStart int
	TextEnd   int
	End       int
}

// NoteTextRun is the Notes-owned rich run, including document decorations.
type NoteTextRun struct {
	Start          int
	End            int
	Style          woxui.TextStyle
	Color          woxui.Color
	Underline      bool
	Strike         bool
	Background     woxui.Color
	Checkbox       bool
	Checked        bool
	LeadingBar     bool
	HorizontalRule bool
}

type noteInlineStyle struct {
	bold, italic, underline, strike, code bool
	link                                  string
}

// NoteDocumentSegment is one linear text run or one structural table/image block.
type NoteDocumentSegment struct {
	Start int
	End   int
	Table bool
	Image bool
}

// Structural reports segments that render outside the linear text field.
func (s NoteDocumentSegment) Structural() bool {
	return s.Table || s.Image
}

var noteInlineRules = []struct {
	pattern *regexp.Regexp
	apply   func(*common.NoteSpan)
}{
	{regexp.MustCompile(`\[([^\]]+)\]\(([^\s)]+)\)`), func(span *common.NoteSpan) { span.Link = "$2" }},
	{regexp.MustCompile("`([^`]+)`"), func(span *common.NoteSpan) { span.Code = true }},
	{regexp.MustCompile(`\*\*([^*]+)\*\*`), func(span *common.NoteSpan) { span.Bold = true }},
	{regexp.MustCompile(`~~([^~]+)~~`), func(span *common.NoteSpan) { span.Strike = true }},
	{regexp.MustCompile(`\*([^*]+)\*`), func(span *common.NoteSpan) { span.Italic = true }},
}

var noteOrderedPrefix = regexp.MustCompile(`^\d+\. `)

// FieldRun converts a Notes decoration into a generic text-field run.
func (run NoteTextRun) FieldRun() TextFieldRichRun {
	field := TextFieldRichRun{
		Start: run.Start, End: run.End, Style: run.Style, Color: run.Color,
		Underline: run.Underline, Strike: run.Strike, Background: run.Background,
	}
	if run.Checkbox {
		size, color, checked := run.Style.Size, run.Color, run.Checked
		field.Advance = documentCheckboxWidth(size)
		field.HideText = true
		field.Paint = func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			paintDocumentCheckbox(displayList, bounds, size, color, checked)
		}
	}
	if run.LeadingBar {
		size, color := run.Style.Size, run.Color
		field.LineGutter = true
		field.LineGutterWidth = documentQuoteWidth(size)
		field.PaintLineGutter = func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			paintDocumentQuoteBar(displayList, bounds, size, color)
		}
	}
	if run.HorizontalRule {
		color := run.Color
		field.HideText = true
		field.Paint = func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			paintDocumentHorizontalRule(displayList, woxui.Rect{
				X: bounds.X, Y: bounds.Y + (bounds.Height-documentRuleHeight)/2,
				Width: bounds.Width, Height: documentRuleHeight,
			}, color)
		}
	}
	return field
}

// NoteFieldRuns converts Notes-owned decorations into generic text-field runs.
func NoteFieldRuns(runs []NoteTextRun) []TextFieldRichRun {
	fields := make([]TextFieldRichRun, 0, len(runs))
	for _, run := range runs {
		fields = append(fields, run.FieldRun())
	}
	return fields
}

// ProjectNoteDocument creates the editor's plain-text backing value and its non-overlapping visual runs.
func ProjectNoteDocument(document common.NoteDocument, base woxui.TextStyle, theme Theme) (string, []NoteTextRun, []NoteBlockRange) {
	var output strings.Builder
	runs := make([]NoteTextRun, 0)
	ranges := make([]NoteBlockRange, 0, len(document.Blocks))
	ordered := [common.NoteMaximumIndent + 1]int{1, 1, 1}
	offset := 0
	codeBackground := theme.QueryBackground
	codeBackground.A = min(uint8(150), codeBackground.A)
	first := true
	for index, block := range document.Blocks {
		if block.IsStructural() {
			ordered = [common.NoteMaximumIndent + 1]int{1, 1, 1}
			continue
		}
		if !first {
			output.WriteRune('\n')
			offset++
		}
		first = false
		start := offset
		indent := max(0, min(common.NoteMaximumIndent, block.Indent))
		marker := start + indent*4
		prefix := strings.Repeat("    ", indent) + noteBlockPrefix(block, ordered[indent])
		if block.Type == common.NoteBlockOrdered {
			ordered[indent]++
			for level := indent + 1; level < len(ordered); level++ {
				ordered[level] = 1
			}
		} else if block.Type != common.NoteBlockBullet && block.Type != common.NoteBlockTask {
			ordered = [common.NoteMaximumIndent + 1]int{1, 1, 1}
		}
		output.WriteString(prefix)
		textStart := start + utf8.RuneCountInString(prefix)
		offset = textStart
		value := block.Text
		if block.Type == common.NoteBlockDivider {
			value = "────────"
		}
		output.WriteString(value)
		textEnd := textStart + utf8.RuneCountInString(value)
		offset = textEnd
		ranges = append(ranges, NoteBlockRange{Block: index, Start: start, Marker: marker, TextStart: textStart, TextEnd: textEnd, End: textEnd})
		if block.Type == common.NoteBlockTask {
			runs = append(runs, NoteTextRun{Start: marker, End: marker + 1, Style: base, Color: DocumentListMarkerColor, Checkbox: true, Checked: block.Checked})
		} else if block.Type == common.NoteBlockBullet || block.Type == common.NoteBlockOrdered {
			runs = append(runs, NoteTextRun{Start: marker, End: textStart, Style: base, Color: DocumentListMarkerColor})
		}
		if block.Type == common.NoteBlockQuote {
			runs = append(runs, NoteTextRun{Start: start, End: textEnd, Style: base, Color: DocumentListMarkerColor, LeadingBar: true})
		}
		if block.Type == common.NoteBlockDivider {
			runs = append(runs, NoteTextRun{Start: textStart, End: textEnd, Style: base, Color: theme.PreviewSplit, HorizontalRule: true})
			continue
		}
		styles := noteBlockStyles(block, value)
		for offset := 0; offset < len(styles); {
			end := offset + 1
			for end < len(styles) && styles[end] == styles[offset] {
				end++
			}
			style := base
			scale := base.Size / 14
			if scale <= 0 {
				scale = 1
			}
			switch block.Type {
			case common.NoteBlockHeading1:
				style.Size, style.Weight = 20*scale, woxui.FontWeightSemibold
			case common.NoteBlockHeading2:
				style.Size, style.Weight = 17*scale, woxui.FontWeightSemibold
			case common.NoteBlockHeading3:
				style.Size, style.Weight = 15*scale, woxui.FontWeightSemibold
			case common.NoteBlockCode:
				style.Family = woxui.FontFamilyMonospace
			}
			inline := styles[offset]
			if inline.bold {
				style.Weight = woxui.FontWeightSemibold
			}
			style.Italic = inline.italic
			if inline.code {
				style.Family = woxui.FontFamilyMonospace
			}
			background := woxui.Color{}
			color := woxui.Color{}
			if inline.code || block.Type == common.NoteBlockCode {
				background = codeBackground
			}
			if block.Type == common.NoteBlockTask && block.Checked {
				color = theme.ResultSubtitle
			}
			link := NoteOpenableLink(inline.link)
			if link != "" && color == (woxui.Color{}) {
				color = theme.Cursor
			}
			runs = append(runs, NoteTextRun{
				Start: textStart + offset, End: textStart + end, Style: style,
				Color: color, Underline: inline.underline || inline.link != "", Strike: inline.strike, Background: background,
			})
			offset = end
		}
	}
	return output.String(), runs, ranges
}

func noteBlockPrefix(block common.NoteBlock, ordered int) string {
	switch block.Type {
	case common.NoteBlockQuote:
		return "│ "
	case common.NoteBlockBullet:
		return "• "
	case common.NoteBlockOrdered:
		return strconv.Itoa(ordered) + ". "
	case common.NoteBlockTask:
		if block.Checked {
			return "☑ "
		}
		return "☐ "
	default:
		return ""
	}
}

func noteBlockStyles(block common.NoteBlock, value string) []noteInlineStyle {
	styles := make([]noteInlineStyle, utf8.RuneCountInString(value))
	for _, span := range block.Spans {
		start := max(0, min(len(styles), span.Start))
		end := max(start, min(len(styles), span.End))
		for index := start; index < end; index++ {
			styles[index] = noteInlineStyle{
				bold: styles[index].bold || span.Bold, italic: styles[index].italic || span.Italic,
				underline: styles[index].underline || span.Underline, strike: styles[index].strike || span.Strike,
				code: styles[index].code || span.Code, link: noteFirstNonEmpty(styles[index].link, span.Link),
			}
		}
	}
	return styles
}

func noteFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// DocumentFromEditor applies Markdown input rules and preserves existing inline styles across local edits.
func DocumentFromEditor(value string, previous common.NoteDocument) common.NoteDocument {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	blocks := make([]common.NoteBlock, 0, len(lines))
	inCodeFence := false
	for _, raw := range lines {
		if strings.HasPrefix(strings.TrimSpace(raw), "```") {
			inCodeFence = !inCodeFence
			continue
		}
		old := common.NoteBlock{ID: uuid.NewString(), Type: common.NoteBlockParagraph}
		index := len(blocks)
		if index < len(previous.Blocks) {
			old = previous.Blocks[index]
			if old.IsStructural() {
				old = common.NoteBlock{ID: old.ID, Type: common.NoteBlockParagraph}
			}
		} else if index > 0 && blocks[index-1].Type == common.NoteBlockCode {
			old.Type = common.NoteBlockCode
		}
		if inCodeFence {
			old.Type = common.NoteBlockCode
		}
		blockType, checked, indent, text := old.Type, old.Checked, old.Indent, raw
		var spans []common.NoteSpan
		if blockType == common.NoteBlockCode {
			checked = false
		} else {
			blockType, checked, indent, text = parseNoteBlock(raw, old.Type, old.Checked)
			spans = remapNoteSpans(old.Text, text, old.Spans)
			text, spans = parseNoteInlineMarkdown(text, spans)
		}
		blocks = append(blocks, common.NoteBlock{ID: old.ID, Type: blockType, Text: text, Checked: checked, Indent: indent, Spans: compactNoteSpans(text, spans)})
	}
	if len(blocks) == 0 {
		blocks = append(blocks, common.NoteBlock{ID: uuid.NewString(), Type: common.NoteBlockParagraph})
	}
	return common.NoteDocument{Version: 1, Blocks: promoteTypedTables(blocks)}
}

func parseNoteBlock(raw string, previous common.NoteBlockType, checked bool) (common.NoteBlockType, bool, int, string) {
	type prefixRule struct {
		prefix string
		typeID common.NoteBlockType
	}
	for _, rule := range []prefixRule{{"### ", common.NoteBlockHeading3}, {"## ", common.NoteBlockHeading2}, {"# ", common.NoteBlockHeading1}, {"> ", common.NoteBlockQuote}, {"│ ", common.NoteBlockQuote}} {
		if strings.HasPrefix(strings.ToLower(raw), strings.ToLower(rule.prefix)) {
			return rule.typeID, false, 0, raw[len(rule.prefix):]
		}
	}
	indent, listValue := noteListIndent(raw)
	for _, rule := range []prefixRule{{"- [ ] ", common.NoteBlockTask}, {"[ ] ", common.NoteBlockTask}, {"- [x] ", common.NoteBlockTask}, {"[x] ", common.NoteBlockTask}, {"- ", common.NoteBlockBullet}, {"• ", common.NoteBlockBullet}} {
		if strings.HasPrefix(strings.ToLower(listValue), strings.ToLower(rule.prefix)) {
			return rule.typeID, strings.Contains(strings.ToLower(rule.prefix), "x"), indent, listValue[len(rule.prefix):]
		}
	}
	if strings.HasPrefix(listValue, "☐ ") {
		return common.NoteBlockTask, false, indent, strings.TrimPrefix(listValue, "☐ ")
	}
	if strings.HasPrefix(listValue, "☑ ") {
		return common.NoteBlockTask, true, indent, strings.TrimPrefix(listValue, "☑ ")
	}
	if noteOrderedPrefix.MatchString(listValue) {
		return common.NoteBlockOrdered, false, indent, noteOrderedPrefix.ReplaceAllString(listValue, "")
	}
	if raw == "---" || raw == "────────" {
		return common.NoteBlockDivider, false, 0, ""
	}
	if previous == common.NoteBlockBullet || previous == common.NoteBlockOrdered || previous == common.NoteBlockTask || previous == common.NoteBlockQuote || previous == common.NoteBlockDivider || previous == common.NoteBlockTable || previous == common.NoteBlockImage {
		previous = common.NoteBlockParagraph
	}
	return previous, checked && previous == common.NoteBlockTask, 0, raw
}

func noteListIndent(value string) (int, string) {
	indent := 0
	for indent < common.NoteMaximumIndent {
		switch {
		case strings.HasPrefix(value, "    "):
			value = strings.TrimPrefix(value, "    ")
		case strings.HasPrefix(value, "\t"):
			value = strings.TrimPrefix(value, "\t")
		default:
			return indent, value
		}
		indent++
	}
	return indent, value
}

func promoteTypedTables(blocks []common.NoteBlock) []common.NoteBlock {
	promoted := make([]common.NoteBlock, 0, len(blocks))
	for index := 0; index < len(blocks); {
		if table, consumed := typedTableAt(blocks, index); consumed > 0 {
			promoted = append(promoted, common.NoteBlock{ID: blocks[index].ID, Type: common.NoteBlockTable, Table: table, Text: noteTableMarkdown(*table)})
			index += consumed
			continue
		}
		promoted = append(promoted, blocks[index])
		index++
	}
	return promoted
}

func typedTableAt(blocks []common.NoteBlock, start int) (*common.NoteTable, int) {
	if start+1 >= len(blocks) || blocks[start].Type == common.NoteBlockCode || blocks[start+1].Type == common.NoteBlockCode {
		return nil, 0
	}
	lines := []string{blocks[start].Text, blocks[start+1].Text}
	if !strings.Contains(lines[0], "|") || !noteTypedTableSeparator(lines[1]) {
		return nil, 0
	}
	end := start + 2
	for end < len(blocks) && blocks[end].Type != common.NoteBlockCode && strings.Contains(blocks[end].Text, "|") {
		lines = append(lines, blocks[end].Text)
		end++
	}
	table := noteTableFromPipeLines(lines)
	if table == nil {
		return nil, 0
	}
	return table, end - start
}

func noteTableFromPipeLines(lines []string) *common.NoteTable {
	if len(lines) < 2 {
		return nil
	}
	table := common.NoteTable{HeaderRows: 1}
	for index, line := range lines {
		if index == 1 && noteTypedTableSeparator(line) {
			continue
		}
		cells := splitNotePipeRow(line)
		if len(cells) == 0 {
			return nil
		}
		table.Rows = append(table.Rows, cells)
	}
	if len(table.Rows) == 0 {
		return nil
	}
	return &table
}

func splitNotePipeRow(line string) []common.NoteTableCell {
	line = strings.TrimSpace(line)
	line = strings.Trim(line, "|")
	if strings.TrimSpace(line) == "" {
		return nil
	}
	parts := strings.Split(line, "|")
	cells := make([]common.NoteTableCell, 0, len(parts))
	for _, part := range parts {
		text, spans := parseNoteInlineMarkdown(strings.TrimSpace(part), nil)
		cells = append(cells, common.NoteTableCell{Text: text, Spans: compactNoteSpans(text, spans)})
	}
	return cells
}

func noteTableMarkdown(table common.NoteTable) string {
	if len(table.Rows) == 0 {
		return ""
	}
	columns := noteTableColumns(table)
	lines := make([]string, 0, len(table.Rows)+1)
	for index, row := range table.Rows {
		cells := make([]string, columns)
		for column := 0; column < columns; column++ {
			value := ""
			if column < len(row) {
				value = row[column].Text
			}
			cells[column] = " " + strings.ReplaceAll(value, "|", "\\|") + " "
		}
		lines = append(lines, "|"+strings.Join(cells, "|")+"|")
		if index+1 == max(1, table.HeaderRows) || (table.HeaderRows == 0 && index == 0) {
			sep := make([]string, columns)
			for column := range sep {
				sep[column] = " --- "
			}
			lines = append(lines, "|"+strings.Join(sep, "|")+"|")
		}
	}
	return strings.Join(lines, "\n")
}

func defaultNoteTable() common.NoteTable {
	row := func() []common.NoteTableCell { return []common.NoteTableCell{{}, {}, {}} }
	return common.NoteTable{HeaderRows: 1, Rows: [][]common.NoteTableCell{row(), row()}}
}

func noteTypedTableSeparator(value string) bool {
	value = strings.TrimSpace(value)
	if !strings.Contains(value, "-") {
		return false
	}
	trimmed := strings.Trim(value, "|")
	for _, cell := range strings.Split(trimmed, "|") {
		cell = strings.TrimSpace(strings.Trim(strings.TrimSpace(cell), ":"))
		if cell != "" && strings.Trim(cell, "-") != "" {
			return false
		}
	}
	return true
}

func remapNoteSpans(oldValue, newValue string, spans []common.NoteSpan) []common.NoteSpan {
	oldRunes, newRunes := []rune(oldValue), []rune(newValue)
	prefix := 0
	for prefix < len(oldRunes) && prefix < len(newRunes) && oldRunes[prefix] == newRunes[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(oldRunes)-prefix && suffix < len(newRunes)-prefix && oldRunes[len(oldRunes)-1-suffix] == newRunes[len(newRunes)-1-suffix] {
		suffix++
	}
	oldEnd, newEnd := len(oldRunes)-suffix, len(newRunes)-suffix
	delta := newEnd - oldEnd
	result := make([]common.NoteSpan, 0, len(spans))
	for _, span := range spans {
		if span.End <= prefix {
			result = append(result, span)
			continue
		}
		if span.Start >= oldEnd {
			span.Start += delta
			span.End += delta
			result = append(result, span)
			continue
		}
		span.Start = min(span.Start, prefix)
		span.End = max(span.Start, span.End+delta)
		if span.End > span.Start {
			result = append(result, span)
		}
	}
	return result
}

func parseNoteInlineMarkdown(value string, spans []common.NoteSpan) (string, []common.NoteSpan) {
	for _, rule := range noteInlineRules {
		for {
			match := rule.pattern.FindStringSubmatchIndex(value)
			if match == nil {
				break
			}
			runes := []rune(value)
			byteToRune := func(byteOffset int) int { return utf8.RuneCountInString(value[:byteOffset]) }
			fullStart, fullEnd := byteToRune(match[0]), byteToRune(match[1])
			contentStart, contentEnd := byteToRune(match[2]), byteToRune(match[3])
			content := string(runes[contentStart:contentEnd])
			span := common.NoteSpan{Start: fullStart, End: fullStart + utf8.RuneCountInString(content)}
			rule.apply(&span)
			if span.Link == "$2" && len(match) >= 6 {
				span.Link = value[match[4]:match[5]]
			}
			value = string(runes[:fullStart]) + content + string(runes[fullEnd:])
			spans = remapNoteSpans(string(runes), value, spans)
			spans = append(spans, span)
		}
	}
	return value, spans
}

func compactNoteSpans(value string, spans []common.NoteSpan) []common.NoteSpan {
	styles := make([]noteInlineStyle, utf8.RuneCountInString(value))
	for _, span := range spans {
		for index := max(0, span.Start); index < min(len(styles), span.End); index++ {
			styles[index].bold = styles[index].bold || span.Bold
			styles[index].italic = styles[index].italic || span.Italic
			styles[index].underline = styles[index].underline || span.Underline
			styles[index].strike = styles[index].strike || span.Strike
			styles[index].code = styles[index].code || span.Code
			styles[index].link = noteFirstNonEmpty(styles[index].link, span.Link)
		}
	}
	return spansFromNoteStyles(styles)
}

// ToggleNoteInline applies or removes one style across every selected block range.
func ToggleNoteInline(document common.NoteDocument, ranges []NoteBlockRange, selection woxui.TextSelection, kind string, link string) common.NoteDocument {
	start, end := selection.Start(), selection.End()
	if start == end {
		return document
	}
	allActive := true
	for _, blockRange := range ranges {
		from, to := max(start, blockRange.TextStart), min(end, blockRange.TextEnd)
		if from >= to {
			continue
		}
		styles := noteBlockStyles(document.Blocks[blockRange.Block], document.Blocks[blockRange.Block].Text)
		for index := from - blockRange.TextStart; index < to-blockRange.TextStart; index++ {
			if !noteStyleActive(styles[index], kind, link) {
				allActive = false
			}
		}
	}
	for _, blockRange := range ranges {
		from, to := max(start, blockRange.TextStart), min(end, blockRange.TextEnd)
		if from >= to {
			continue
		}
		block := &document.Blocks[blockRange.Block]
		styles := noteBlockStyles(*block, block.Text)
		for index := from - blockRange.TextStart; index < to-blockRange.TextStart; index++ {
			setNoteStyle(&styles[index], kind, !allActive, link)
		}
		block.Spans = spansFromNoteStyles(styles)
	}
	return document
}

// ToggleNoteTableInline applies one inline style to the focused table cell.
func ToggleNoteTableInline(document common.NoteDocument, block, row, column int, kind, link string) common.NoteDocument {
	cell := noteTableCell(document, block, row, column)
	if cell == nil || cell.Text == "" {
		return document
	}
	styles := noteBlockStyles(common.NoteBlock{Text: cell.Text, Spans: cell.Spans}, cell.Text)
	allActive := len(styles) > 0
	for _, style := range styles {
		if !noteStyleActive(style, kind, link) {
			allActive = false
			break
		}
	}
	for index := range styles {
		setNoteStyle(&styles[index], kind, !allActive, link)
	}
	cell.Spans = spansFromNoteStyles(styles)
	return document
}

func noteTableCell(document common.NoteDocument, block, row, column int) *common.NoteTableCell {
	if block < 0 || block >= len(document.Blocks) || document.Blocks[block].Table == nil {
		return nil
	}
	table := document.Blocks[block].Table
	if row < 0 || row >= len(table.Rows) || column < 0 || column >= len(table.Rows[row]) {
		return nil
	}
	return &table.Rows[row][column]
}

// NoteActiveFormats reports which format-bar controls match the caret or selection.
func NoteActiveFormats(document common.NoteDocument, ranges []NoteBlockRange, selection woxui.TextSelection) map[string]bool {
	active := map[string]bool{}
	if len(document.Blocks) == 0 {
		return active
	}
	if len(ranges) > 0 {
		index := NoteBlockAt(ranges, selection.Focus)
		if index >= 0 && index < len(document.Blocks) {
			applyNoteBlockFormat(active, document.Blocks[index].Type)
		}
		inline := noteInlineStyleAt(document, ranges, selection)
		applyNoteInlineFormat(active, inline)
	}
	return active
}

// NoteActiveFormatsForTable reports format-bar state for a focused table cell.
// Block styles come from the table itself, not leftover text-caret selection outside the grid.
func NoteActiveFormatsForTable(document common.NoteDocument, block, row, column int) map[string]bool {
	active := map[string]bool{}
	applyNoteBlockFormat(active, common.NoteBlockTable)
	cell := noteTableCell(document, block, row, column)
	if cell == nil || cell.Text == "" {
		return active
	}
	styles := noteBlockStyles(common.NoteBlock{Text: cell.Text, Spans: cell.Spans}, cell.Text)
	if len(styles) == 0 {
		return active
	}
	combined := styles[0]
	for _, style := range styles[1:] {
		combined.bold = combined.bold && style.bold
		combined.italic = combined.italic && style.italic
		combined.underline = combined.underline && style.underline
		combined.strike = combined.strike && style.strike
		combined.code = combined.code && style.code
		if combined.link != style.link {
			combined.link = ""
		}
	}
	applyNoteInlineFormat(active, combined)
	return active
}

func applyNoteBlockFormat(active map[string]bool, blockType common.NoteBlockType) {
	switch blockType {
	case common.NoteBlockHeading1, common.NoteBlockHeading2, common.NoteBlockHeading3:
		active["block"] = true
	case common.NoteBlockCode:
		active["code"] = true
	case common.NoteBlockBullet:
		active["bullet"] = true
	case common.NoteBlockOrdered:
		active["ordered"] = true
	case common.NoteBlockTask:
		active["task"] = true
	case common.NoteBlockQuote:
		active["quote"] = true
	case common.NoteBlockDivider:
		active["divider"] = true
	case common.NoteBlockTable:
		active["table"] = true
	}
}

func applyNoteInlineFormat(active map[string]bool, inline noteInlineStyle) {
	if inline.bold {
		active["bold"] = true
	}
	if inline.italic {
		active["italic"] = true
	}
	if inline.underline {
		active["underline"] = true
	}
	if inline.strike {
		active["strike"] = true
	}
	if inline.code {
		active["code"] = true
	}
	if inline.link != "" {
		active["link"] = true
	}
}

func noteInlineStyleAt(document common.NoteDocument, ranges []NoteBlockRange, selection woxui.TextSelection) noteInlineStyle {
	start, end := selection.Start(), selection.End()
	if start == end {
		return noteInlineStyleAtOffset(document, ranges, start)
	}
	var combined *noteInlineStyle
	for _, blockRange := range ranges {
		from, to := max(start, blockRange.TextStart), min(end, blockRange.TextEnd)
		if from >= to {
			continue
		}
		styles := noteBlockStyles(document.Blocks[blockRange.Block], document.Blocks[blockRange.Block].Text)
		for index := from - blockRange.TextStart; index < to-blockRange.TextStart; index++ {
			style := styles[index]
			if combined == nil {
				current := style
				combined = &current
				continue
			}
			combined.bold = combined.bold && style.bold
			combined.italic = combined.italic && style.italic
			combined.underline = combined.underline && style.underline
			combined.strike = combined.strike && style.strike
			combined.code = combined.code && style.code
			if combined.link != style.link {
				combined.link = ""
			}
		}
	}
	if combined == nil {
		return noteInlineStyle{}
	}
	return *combined
}

func noteInlineStyleAtOffset(document common.NoteDocument, ranges []NoteBlockRange, offset int) noteInlineStyle {
	index := NoteBlockAt(ranges, offset)
	if index < 0 || index >= len(document.Blocks) {
		return noteInlineStyle{}
	}
	blockRange := ranges[0]
	for _, candidate := range ranges {
		if candidate.Block == index {
			blockRange = candidate
			break
		}
	}
	styles := noteBlockStyles(document.Blocks[index], document.Blocks[index].Text)
	if len(styles) == 0 {
		return noteInlineStyle{}
	}
	runeOffset := offset - blockRange.TextStart
	if runeOffset >= len(styles) {
		runeOffset = len(styles) - 1
	}
	if runeOffset < 0 {
		return noteInlineStyle{}
	}
	return styles[runeOffset]
}

func noteStyleActive(style noteInlineStyle, kind, link string) bool {
	switch kind {
	case "bold":
		return style.bold
	case "italic":
		return style.italic
	case "underline":
		return style.underline
	case "strike":
		return style.strike
	case "code":
		return style.code
	case "link":
		return style.link != "" && (link == "" || style.link == link)
	default:
		return false
	}
}

func setNoteStyle(style *noteInlineStyle, kind string, enabled bool, link string) {
	switch kind {
	case "bold":
		style.bold = enabled
	case "italic":
		style.italic = enabled
	case "underline":
		style.underline = enabled
	case "strike":
		style.strike = enabled
	case "code":
		style.code = enabled
	case "link":
		if enabled {
			style.link = link
		} else {
			style.link = ""
		}
	}
}

func spansFromNoteStyles(styles []noteInlineStyle) []common.NoteSpan {
	spans := make([]common.NoteSpan, 0)
	for start := 0; start < len(styles); {
		if styles[start] == (noteInlineStyle{}) {
			start++
			continue
		}
		end := start + 1
		for end < len(styles) && styles[end] == styles[start] {
			end++
		}
		style := styles[start]
		spans = append(spans, common.NoteSpan{Start: start, End: end, Bold: style.bold, Italic: style.italic, Underline: style.underline, Strike: style.strike, Code: style.code, Link: style.link})
		start = end
	}
	return spans
}

// NoteBlockAt returns the document block that contains the projected caret offset.
func NoteBlockAt(ranges []NoteBlockRange, offset int) int {
	for _, blockRange := range ranges {
		if offset <= blockRange.End {
			return blockRange.Block
		}
	}
	if len(ranges) == 0 {
		return 0
	}
	return ranges[len(ranges)-1].Block
}

// NoteOpenableLink keeps pointer activation on the same schemes Window.OpenExternalURL accepts.
func NoteOpenableLink(target string) string {
	target = strings.TrimSpace(target)
	parsed, err := url.ParseRequestURI(target)
	if err != nil {
		return ""
	}
	if (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" {
		return parsed.String()
	}
	if parsed.Scheme == "mailto" && parsed.Opaque != "" {
		return parsed.String()
	}
	return ""
}

// NoteLinkAtOffset returns the openable URL under a projected editor offset.
func NoteLinkAtOffset(document common.NoteDocument, ranges []NoteBlockRange, offset int) string {
	if len(document.Blocks) == 0 || len(ranges) == 0 {
		return ""
	}
	index := NoteBlockAt(ranges, offset)
	blockRange := NoteRangeForBlock(ranges, index)
	if offset < blockRange.TextStart || offset > blockRange.TextEnd {
		return ""
	}
	block := document.Blocks[index]
	styles := noteBlockStyles(block, block.Text)
	if len(styles) == 0 {
		return ""
	}
	runeOffset := offset - blockRange.TextStart
	if runeOffset < 0 || runeOffset > len(styles) {
		return ""
	}
	if runeOffset < len(styles) {
		if link := NoteOpenableLink(styles[runeOffset].link); link != "" {
			return link
		}
	}
	if runeOffset > 0 {
		return NoteOpenableLink(styles[runeOffset-1].link)
	}
	return ""
}

// NoteRangeForBlock resolves a document block index to offsets within the projected text segment.
func NoteRangeForBlock(ranges []NoteBlockRange, block int) NoteBlockRange {
	for _, blockRange := range ranges {
		if blockRange.Block == block {
			return blockRange
		}
	}
	return ranges[0]
}

// NoteTaskAtOffset limits task activation to the rendered checkbox prefix.
func NoteTaskAtOffset(document common.NoteDocument, ranges []NoteBlockRange, offset int) (int, bool) {
	if len(document.Blocks) == 0 || len(ranges) == 0 {
		return 0, false
	}
	index := NoteBlockAt(ranges, offset)
	blockRange := NoteRangeForBlock(ranges, index)
	return index, document.Blocks[index].Type == common.NoteBlockTask && offset >= blockRange.Marker && offset <= blockRange.TextStart
}

// NoteDividerAtOffset treats a horizontal rule as an atomic visual block rather than selectable placeholder text.
func NoteDividerAtOffset(document common.NoteDocument, ranges []NoteBlockRange, offset int) bool {
	if len(document.Blocks) == 0 || len(ranges) == 0 {
		return false
	}
	index := NoteBlockAt(ranges, offset)
	blockRange := NoteRangeForBlock(ranges, index)
	return document.Blocks[index].Type == common.NoteBlockDivider && offset >= blockRange.Start && offset <= blockRange.End
}

// ContinueNoteBlock splits a list or quote and keeps the same marker on the new block.
func ContinueNoteBlock(document common.NoteDocument, ranges []NoteBlockRange, selection woxui.TextSelection) (common.NoteDocument, int, bool) {
	if !selection.Collapsed() || len(document.Blocks) == 0 || len(ranges) == 0 {
		return document, 0, false
	}
	index := NoteBlockAt(ranges, selection.Focus)
	block := document.Blocks[index]
	switch block.Type {
	case common.NoteBlockBullet, common.NoteBlockOrdered, common.NoteBlockTask, common.NoteBlockQuote:
	default:
		return document, 0, false
	}

	updated := CloneNoteDocument(document)
	if block.Text == "" {
		updated.Blocks[index].Type = common.NoteBlockParagraph
		updated.Blocks[index].Checked = false
		updated.Blocks[index].Indent = 0
		return updated, index, true
	}

	runes := []rune(block.Text)
	blockRange := NoteRangeForBlock(ranges, index)
	offset := max(0, min(len(runes), selection.Focus-blockRange.TextStart))
	styles := noteBlockStyles(block, block.Text)
	updated.Blocks[index].Text = string(runes[:offset])
	updated.Blocks[index].Spans = spansFromNoteStyles(styles[:offset])
	next := common.NoteBlock{ID: uuid.NewString(), Type: block.Type, Text: string(runes[offset:]), Indent: block.Indent, Spans: spansFromNoteStyles(styles[offset:])}
	updated.Blocks = slices.Insert(updated.Blocks, index+1, next)
	return updated, index + 1, true
}

// AdjustNoteListIndent changes the active list item without exposing indentation as document text.
func AdjustNoteListIndent(document common.NoteDocument, ranges []NoteBlockRange, selection woxui.TextSelection, delta int) (common.NoteDocument, int, bool, bool) {
	if !selection.Collapsed() || len(document.Blocks) == 0 || len(ranges) == 0 {
		return document, 0, false, false
	}
	index := NoteBlockAt(ranges, selection.Focus)
	block := document.Blocks[index]
	if block.Type != common.NoteBlockBullet && block.Type != common.NoteBlockOrdered && block.Type != common.NoteBlockTask {
		return document, index, false, false
	}
	next := max(0, min(common.NoteMaximumIndent, block.Indent+delta))
	if delta > 0 {
		if index == 0 {
			next = block.Indent
		} else {
			previous := document.Blocks[index-1]
			if (previous.Type != common.NoteBlockBullet && previous.Type != common.NoteBlockOrdered && previous.Type != common.NoteBlockTask) || previous.Indent < block.Indent {
				next = block.Indent
			}
		}
	}
	if next == block.Indent {
		return document, index, false, true
	}
	updated := CloneNoteDocument(document)
	updated.Blocks[index].Indent = next
	return updated, index, true, true
}

// CloneNoteDocument copies blocks, spans, and table cells for undo snapshots.
func CloneNoteDocument(document common.NoteDocument) common.NoteDocument {
	clone := common.NoteDocument{Version: document.Version, Blocks: make([]common.NoteBlock, len(document.Blocks))}
	for index, block := range document.Blocks {
		clone.Blocks[index] = block
		clone.Blocks[index].Spans = append([]common.NoteSpan(nil), block.Spans...)
		if block.Table != nil {
			table := *block.Table
			table.Rows = make([][]common.NoteTableCell, len(block.Table.Rows))
			for rowIndex, row := range block.Table.Rows {
				table.Rows[rowIndex] = make([]common.NoteTableCell, len(row))
				for column, cell := range row {
					table.Rows[rowIndex][column] = cell
					table.Rows[rowIndex][column].Spans = append([]common.NoteSpan(nil), cell.Spans...)
				}
			}
			clone.Blocks[index].Table = &table
		}
		if block.Image != nil {
			image := *block.Image
			clone.Blocks[index].Image = &image
		}
	}
	return clone
}

// NoteDocumentSegments splits a document into linear text runs and table blocks.
func NoteDocumentSegments(document common.NoteDocument) []NoteDocumentSegment {
	return noteDocumentSegments(document)
}

// NoteSegmentDocument returns the blocks owned by one editor segment.
func NoteSegmentDocument(document common.NoteDocument, segment NoteDocumentSegment) common.NoteDocument {
	return noteSegmentDocument(document, segment)
}

func noteDocumentSegments(document common.NoteDocument) []NoteDocumentSegment {
	segments := make([]NoteDocumentSegment, 0)
	start := 0
	for index, block := range document.Blocks {
		if !block.IsStructural() {
			continue
		}
		if start < index {
			segments = append(segments, NoteDocumentSegment{Start: start, End: index})
		}
		segments = append(segments, NoteDocumentSegment{Start: index, End: index + 1, Table: block.Type == common.NoteBlockTable, Image: block.Type == common.NoteBlockImage})
		start = index + 1
	}
	if start < len(document.Blocks) || len(segments) == 0 {
		segments = append(segments, NoteDocumentSegment{Start: start, End: max(start, len(document.Blocks))})
	}
	return segments
}

// ProjectNoteSegment projects one text segment and remaps ranges onto the full document.
func ProjectNoteSegment(document common.NoteDocument, segment NoteDocumentSegment, base woxui.TextStyle, theme Theme) (string, []NoteTextRun, []NoteBlockRange) {
	value, runs, ranges := ProjectNoteDocument(noteSegmentDocument(document, segment), base, theme)
	for index := range ranges {
		ranges[index].Block += segment.Start
	}
	return value, runs, ranges
}

// ReplaceNoteSegment writes parsed text-segment blocks back without disturbing neighboring tables.
func ReplaceNoteSegment(document common.NoteDocument, segment NoteDocumentSegment, blocks []common.NoteBlock) common.NoteDocument {
	return replaceNoteSegment(document, segment, blocks)
}

// NoteSegmentAtBlock returns the text or table segment that contains the document block.
func NoteSegmentAtBlock(document common.NoteDocument, block int) NoteDocumentSegment {
	for _, segment := range noteDocumentSegments(document) {
		if block >= segment.Start && block < segment.End {
			return segment
		}
	}
	return NoteDocumentSegment{Start: 0, End: len(document.Blocks)}
}

func noteSegmentDocument(document common.NoteDocument, segment NoteDocumentSegment) common.NoteDocument {
	if segment.Start >= len(document.Blocks) {
		return common.NoteDocument{Version: document.Version, Blocks: []common.NoteBlock{{ID: uuid.NewString(), Type: common.NoteBlockParagraph}}}
	}
	return common.NoteDocument{Version: document.Version, Blocks: append([]common.NoteBlock(nil), document.Blocks[segment.Start:segment.End]...)}
}

func noteBlockIsEmptyText(block common.NoteBlock) bool {
	if block.IsStructural() {
		return false
	}
	return strings.TrimSpace(block.Text) == ""
}

// NoteSegmentIsEmpty reports whether a text segment contains only blank blocks.
func NoteSegmentIsEmpty(document common.NoteDocument, segment NoteDocumentSegment) bool {
	if segment.Structural() || segment.Start < 0 || segment.Start >= segment.End || segment.End > len(document.Blocks) {
		return false
	}
	for index := segment.Start; index < segment.End; index++ {
		if !noteBlockIsEmptyText(document.Blocks[index]) {
			return false
		}
	}
	return true
}

func ensureNoteDocumentHasTextBlock(document common.NoteDocument) common.NoteDocument {
	if len(document.Blocks) == 0 {
		document.Blocks = []common.NoteBlock{{ID: uuid.NewString(), Type: common.NoteBlockParagraph}}
		return document
	}
	hasText := false
	for _, block := range document.Blocks {
		if !block.IsStructural() {
			hasText = true
			break
		}
	}
	if !hasText {
		document.Blocks = append(document.Blocks, common.NoteBlock{ID: uuid.NewString(), Type: common.NoteBlockParagraph})
	}
	return ensureNoteImageEditGaps(document)
}

// ensureNoteImageEditGaps keeps empty paragraphs around images so clicks and arrow keys can land a caret.
func ensureNoteImageEditGaps(document common.NoteDocument) common.NoteDocument {
	if len(document.Blocks) == 0 {
		return document
	}
	blocks := make([]common.NoteBlock, 0, len(document.Blocks)+2)
	if document.Blocks[0].Type == common.NoteBlockImage {
		blocks = append(blocks, common.NoteBlock{ID: uuid.NewString(), Type: common.NoteBlockParagraph})
	}
	for index, block := range document.Blocks {
		if index > 0 && document.Blocks[index-1].Type == common.NoteBlockImage && block.Type == common.NoteBlockImage {
			blocks = append(blocks, common.NoteBlock{ID: uuid.NewString(), Type: common.NoteBlockParagraph})
		}
		blocks = append(blocks, block)
	}
	if blocks[len(blocks)-1].Type == common.NoteBlockImage {
		blocks = append(blocks, common.NoteBlock{ID: uuid.NewString(), Type: common.NoteBlockParagraph})
	}
	document.Blocks = blocks
	return document
}

// RemoveEmptyNoteSegment deletes a blank text run so neighboring tables can sit together.
func RemoveEmptyNoteSegment(document common.NoteDocument, segment NoteDocumentSegment) (common.NoteDocument, bool) {
	if !NoteSegmentIsEmpty(document, segment) || len(document.Blocks) <= segment.End-segment.Start {
		return document, false
	}
	updated := CloneNoteDocument(document)
	updated.Blocks = slices.Delete(updated.Blocks, segment.Start, segment.End)
	return ensureNoteDocumentHasTextBlock(updated), true
}

func replaceNoteSegment(document common.NoteDocument, segment NoteDocumentSegment, blocks []common.NoteBlock) common.NoteDocument {
	updated := CloneNoteDocument(document)
	if segment.Start == segment.End && len(updated.Blocks) == 0 {
		updated.Blocks = blocks
		return updated
	}
	updated.Blocks = slices.Replace(updated.Blocks, segment.Start, min(segment.End, len(updated.Blocks)), blocks...)
	if len(updated.Blocks) == 0 {
		updated.Blocks = []common.NoteBlock{{ID: uuid.NewString(), Type: common.NoteBlockParagraph}}
	}
	return updated
}
