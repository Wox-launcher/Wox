package launcher

import (
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"wox/common"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
)

type noteBlockRange struct {
	Block     int
	Start     int
	Marker    int
	TextStart int
	TextEnd   int
	End       int
}

type noteInlineStyle struct {
	bold, italic, underline, strike, code bool
	link                                  string
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

// projectNoteDocument creates the editor's plain-text backing value and its non-overlapping visual runs.
func projectNoteDocument(document common.NoteDocument, base woxui.TextStyle, theme woxcomponent.Theme) (string, []woxcomponent.TextFieldRichRun, []noteBlockRange) {
	var output strings.Builder
	runs := make([]woxcomponent.TextFieldRichRun, 0)
	ranges := make([]noteBlockRange, 0, len(document.Blocks))
	ordered := [common.NoteMaximumIndent + 1]int{1, 1, 1}
	offset := 0
	codeBackground := theme.QueryBackground
	codeBackground.A = min(uint8(150), codeBackground.A)
	for index, block := range document.Blocks {
		if index > 0 {
			output.WriteRune('\n')
			offset++
		}
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
		ranges = append(ranges, noteBlockRange{Block: index, Start: start, Marker: marker, TextStart: textStart, TextEnd: textEnd, End: textEnd})
		if block.Type == common.NoteBlockTask {
			runs = append(runs, woxcomponent.TextFieldRichRun{Start: marker, End: marker + 1, Style: base, Color: woxcomponent.DocumentListMarkerColor, Checkbox: true, Checked: block.Checked})
		} else if block.Type == common.NoteBlockBullet || block.Type == common.NoteBlockOrdered {
			runs = append(runs, woxcomponent.TextFieldRichRun{Start: marker, End: textStart, Style: base, Color: woxcomponent.DocumentListMarkerColor})
		}
		if block.Type == common.NoteBlockQuote {
			runs = append(runs, woxcomponent.TextFieldRichRun{Start: start, End: textEnd, Style: base, Color: woxcomponent.DocumentListMarkerColor, LeadingBar: true})
		}
		if block.Type == common.NoteBlockDivider {
			runs = append(runs, woxcomponent.TextFieldRichRun{Start: textStart, End: textEnd, Style: base, Color: theme.PreviewSplit, HorizontalRule: true})
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
			link := noteOpenableLink(inline.link)
			if link != "" && color == (woxui.Color{}) {
				color = theme.Cursor
			}
			runs = append(runs, woxcomponent.TextFieldRichRun{
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

// noteBlockStyles expands overlapping spans into one style value per rune.
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

// documentFromEditor applies Markdown input rules and preserves existing inline styles across local edits.
func documentFromEditor(value string, previous common.NoteDocument) common.NoteDocument {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	blocks := make([]common.NoteBlock, 0, len(lines))
	inCodeFence := false
	for _, raw := range lines {
		if strings.HasPrefix(strings.TrimSpace(raw), "```") {
			inCodeFence = !inCodeFence
			continue
		}
		old := common.NoteBlock{ID: newID(), Type: common.NoteBlockParagraph}
		index := len(blocks)
		if index < len(previous.Blocks) {
			old = previous.Blocks[index]
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
		blocks = append(blocks, common.NoteBlock{ID: newID(), Type: common.NoteBlockParagraph})
	}
	return common.NoteDocument{Version: 1, Blocks: blocks}
}

// parseNoteBlock applies line-start Markdown rules without exposing their markers.
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
	if previous == common.NoteBlockBullet || previous == common.NoteBlockOrdered || previous == common.NoteBlockTask || previous == common.NoteBlockQuote || previous == common.NoteBlockDivider {
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

// remapNoteSpans preserves styles around the smallest changed rune range.
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

// parseNoteInlineMarkdown removes closed markers and records their resulting styles.
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

// compactNoteSpans merges the expanded style map into non-overlapping persisted spans.
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
	result := make([]common.NoteSpan, 0)
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
		result = append(result, common.NoteSpan{Start: start, End: end, Bold: style.bold, Italic: style.italic, Underline: style.underline, Strike: style.strike, Code: style.code, Link: style.link})
		start = end
	}
	return result
}

// toggleNoteInline applies or removes one style across every selected block range.
func toggleNoteInline(document common.NoteDocument, ranges []noteBlockRange, selection woxui.TextSelection, kind string, link string) common.NoteDocument {
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

// noteActiveFormats reports which format-bar controls match the caret or selection.
func noteActiveFormats(document common.NoteDocument, ranges []noteBlockRange, selection woxui.TextSelection) map[string]bool {
	active := map[string]bool{}
	if len(document.Blocks) == 0 || len(ranges) == 0 {
		return active
	}
	index := noteBlockAt(ranges, selection.Focus)
	if index >= 0 && index < len(document.Blocks) {
		switch document.Blocks[index].Type {
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
		}
	}
	inline := noteInlineStyleAt(document, ranges, selection)
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
	return active
}

// noteInlineStyleAt reads the style under a collapsed caret, or the styles shared by a range.
func noteInlineStyleAt(document common.NoteDocument, ranges []noteBlockRange, selection woxui.TextSelection) noteInlineStyle {
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

// noteInlineStyleAtOffset uses the rune at the caret, or the previous rune when the caret is at a block end.
func noteInlineStyleAtOffset(document common.NoteDocument, ranges []noteBlockRange, offset int) noteInlineStyle {
	index := noteBlockAt(ranges, offset)
	if index < 0 || index >= len(document.Blocks) {
		return noteInlineStyle{}
	}
	blockRange := ranges[index]
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

// spansFromNoteStyles compacts per-rune editor styles after a format operation.
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

func noteBlockAt(ranges []noteBlockRange, offset int) int {
	for _, blockRange := range ranges {
		if offset <= blockRange.End {
			return blockRange.Block
		}
	}
	return max(0, len(ranges)-1)
}

// noteOpenableLink keeps pointer activation on the same schemes Window.OpenExternalURL accepts.
func noteOpenableLink(target string) string {
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

// noteLinkAtOffset returns the openable URL under a projected editor offset.
func noteLinkAtOffset(document common.NoteDocument, ranges []noteBlockRange, offset int) string {
	if len(document.Blocks) == 0 || len(ranges) == 0 {
		return ""
	}
	index := noteBlockAt(ranges, offset)
	blockRange := ranges[index]
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
		if link := noteOpenableLink(styles[runeOffset].link); link != "" {
			return link
		}
	}
	// Hit-testing snaps to the caret after a glyph's midpoint, so the exclusive span end still belongs to the link.
	if runeOffset > 0 {
		return noteOpenableLink(styles[runeOffset-1].link)
	}
	return ""
}

// noteTaskAtOffset limits task activation to the rendered checkbox prefix.
func noteTaskAtOffset(document common.NoteDocument, ranges []noteBlockRange, offset int) (int, bool) {
	if len(document.Blocks) == 0 || len(ranges) == 0 {
		return 0, false
	}
	index := noteBlockAt(ranges, offset)
	blockRange := ranges[index]
	return index, document.Blocks[index].Type == common.NoteBlockTask && offset >= blockRange.Marker && offset <= blockRange.TextStart
}

// noteDividerAtOffset treats a horizontal rule as an atomic visual block rather than selectable placeholder text.
func noteDividerAtOffset(document common.NoteDocument, ranges []noteBlockRange, offset int) bool {
	if len(document.Blocks) == 0 || len(ranges) == 0 {
		return false
	}
	index := noteBlockAt(ranges, offset)
	blockRange := ranges[index]
	return document.Blocks[index].Type == common.NoteBlockDivider && offset >= blockRange.Start && offset <= blockRange.End
}

// continueNoteBlock splits a list or quote and keeps the same marker on the new block.
func continueNoteBlock(document common.NoteDocument, ranges []noteBlockRange, selection woxui.TextSelection) (common.NoteDocument, int, bool) {
	if !selection.Collapsed() || len(document.Blocks) == 0 || len(ranges) == 0 {
		return document, 0, false
	}
	index := noteBlockAt(ranges, selection.Focus)
	block := document.Blocks[index]
	switch block.Type {
	case common.NoteBlockBullet, common.NoteBlockOrdered, common.NoteBlockTask, common.NoteBlockQuote:
	default:
		return document, 0, false
	}

	updated := cloneNoteDocument(document)
	if block.Text == "" {
		updated.Blocks[index].Type = common.NoteBlockParagraph
		updated.Blocks[index].Checked = false
		updated.Blocks[index].Indent = 0
		return updated, index, true
	}

	runes := []rune(block.Text)
	offset := max(0, min(len(runes), selection.Focus-ranges[index].TextStart))
	styles := noteBlockStyles(block, block.Text)
	updated.Blocks[index].Text = string(runes[:offset])
	updated.Blocks[index].Spans = spansFromNoteStyles(styles[:offset])
	next := common.NoteBlock{ID: newID(), Type: block.Type, Text: string(runes[offset:]), Indent: block.Indent, Spans: spansFromNoteStyles(styles[offset:])}
	updated.Blocks = slices.Insert(updated.Blocks, index+1, next)
	return updated, index + 1, true
}

// adjustNoteListIndent changes the active list item without exposing indentation as document text.
func adjustNoteListIndent(document common.NoteDocument, ranges []noteBlockRange, selection woxui.TextSelection, delta int) (common.NoteDocument, int, bool, bool) {
	if !selection.Collapsed() || len(document.Blocks) == 0 || len(ranges) == 0 {
		return document, 0, false, false
	}
	index := noteBlockAt(ranges, selection.Focus)
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
	updated := cloneNoteDocument(document)
	updated.Blocks[index].Indent = next
	return updated, index, true, true
}
