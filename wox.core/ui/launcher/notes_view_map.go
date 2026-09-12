package launcher

import (
	"strings"
	"unicode/utf8"

	"wox/common"
	notesplugin "wox/plugin/system/notes"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
)

// noteVisualCaret keeps a wrapped-line caret on the same screen position after a view toggle.
func (c *notesWindowController) noteVisualCaret(source string, offset int, sourceRuns []woxcomponent.TextFieldRichRun, dest string, destRuns []woxcomponent.TextFieldRichRun) (int, bool) {
	window := c.noteMeasureWindow()
	width := c.noteEditorInnerWidth()
	if window == nil || width <= 0 || !noteSourceLooksWrapped(source, offset) {
		return 0, false
	}
	lineHeight := 24 * c.zoom
	style := c.editorStyle()
	layout, ok := woxcomponent.TextFieldCaretLayoutAt(source, offset, window, style, width, lineHeight, sourceRuns)
	if !ok || !layout.AtLineEnd || layout.LineWidth >= width*0.7 {
		return 0, false
	}
	point := woxui.Point{X: max(layout.Point.X, width*0.45), Y: layout.Point.Y}
	return woxcomponent.TextFieldOffsetAtContentPoint(dest, point, window, style, width, lineHeight, destRuns), true
}

// noteEditorInnerWidth is the text field width after Notes chrome insets.
func (c *notesWindowController) noteEditorInnerWidth() float32 {
	width := c.lastFrame.Width
	if width <= 0 {
		width = notesDefaultWidth
	}
	padding := notesEditorPadding()
	return max(float32(1), width-padding.Left-padding.Right)
}

// noteMeasureWindow is the live window used to wrap editor lines.
func (c *notesWindowController) noteMeasureWindow() *woxui.Window {
	if c == nil || c.managed == nil {
		return nil
	}
	return c.managed.Window()
}

// noteSourceLooksWrapped is a mid-block hard wrap, not a paragraph gap.
func noteSourceLooksWrapped(source string, offset int) bool {
	runes := []rune(source)
	if offset < 0 || offset >= len(runes) || runes[offset] != '\n' {
		return false
	}
	return offset+1 >= len(runes) || runes[offset+1] != '\n'
}

// alignMarkdownOffset maps a caret from live source onto canonical ToMarkdown text.
func alignMarkdownOffset(source, canonical string, offset int) int {
	limit := utf8.RuneCountInString(canonical)
	if source == canonical {
		return min(limit, max(0, offset))
	}
	runes := []rune(source)
	if offset < 0 {
		offset = 0
	}
	if offset > len(runes) {
		offset = len(runes)
	}
	start := max(0, offset-16)
	needle := string(runes[start:offset])
	guess := 0
	if len(runes) > 0 && limit > 0 {
		guess = offset * limit / len(runes)
	}
	if needle == "" {
		return min(guess, limit)
	}
	if index := strings.LastIndex(canonical, needle); index >= 0 {
		return min(limit, utf8.RuneCountInString(canonical[:index])+utf8.RuneCountInString(needle))
	}
	return min(guess, limit)
}

// notePreviewSelectionToMarkdown maps a preview caret onto the raw Markdown source.
func notePreviewSelectionToMarkdown(document common.NoteDocument, ranges []noteBlockRange, selection woxui.TextSelection, imageBlock, tableBlock int, mapped notesplugin.MarkdownCaretMap) woxui.TextSelection {
	limit := utf8.RuneCountInString(mapped.Markdown)
	return woxui.TextSelection{
		Anchor: notePreviewOffsetToMarkdown(document, ranges, selection.Anchor, imageBlock, tableBlock, mapped, limit),
		Focus:  notePreviewOffsetToMarkdown(document, ranges, selection.Focus, imageBlock, tableBlock, mapped, limit),
	}
}

// notePreviewOffsetToMarkdown maps one preview rune offset onto Markdown.
func notePreviewOffsetToMarkdown(document common.NoteDocument, ranges []noteBlockRange, offset, imageBlock, tableBlock int, mapped notesplugin.MarkdownCaretMap, limit int) int {
	block := imageBlock
	if block < 0 {
		block = tableBlock
	}
	textOff := 0
	if block < 0 {
		block = noteBlockAt(ranges, offset)
		for _, item := range ranges {
			if item.Block == block {
				textOff = max(0, offset-item.TextStart)
				break
			}
		}
	}
	for _, span := range mapped.Blocks {
		if span.Index != block {
			continue
		}
		if block >= 0 && block < len(document.Blocks) {
			textOff = notesplugin.MarkdownSourceOffset(document.Blocks[block], textOff)
		}
		return min(limit, max(span.Start, span.TextStart+textOff))
	}
	return min(limit, max(0, offset))
}

// noteMarkdownCaretTarget finds the preview block and intra-block offset for a Markdown caret.
func noteMarkdownCaretTarget(document common.NoteDocument, mapped notesplugin.MarkdownCaretMap, offset int) (block, imageBlock, tableBlock, textOff int) {
	span := noteMarkdownBlockAt(mapped, offset)
	block = span.Index
	if block < 0 || block >= len(document.Blocks) {
		return 0, -1, -1, 0
	}
	textOff = notesplugin.MarkdownTextOffset(document.Blocks[block], max(0, offset-span.TextStart))
	switch document.Blocks[block].Type {
	case common.NoteBlockImage:
		return block, block, -1, 0
	case common.NoteBlockTable:
		return block, -1, block, 0
	default:
		return block, -1, -1, textOff
	}
}

// noteMarkdownBlockAt returns the Markdown span that contains offset.
func noteMarkdownBlockAt(mapped notesplugin.MarkdownCaretMap, offset int) notesplugin.MarkdownCaretBlock {
	for _, block := range mapped.Blocks {
		if offset <= block.End {
			return block
		}
	}
	if len(mapped.Blocks) == 0 {
		return notesplugin.MarkdownCaretBlock{}
	}
	return mapped.Blocks[len(mapped.Blocks)-1]
}

// notePreviewOffsetForBlock converts a block-local text offset into the preview field.
func notePreviewOffsetForBlock(ranges []noteBlockRange, block, textOff int) int {
	for _, item := range ranges {
		if item.Block != block {
			continue
		}
		return min(item.End, max(item.TextStart, item.TextStart+textOff))
	}
	if len(ranges) == 0 {
		return 0
	}
	return ranges[len(ranges)-1].End
}

// noteSegmentForCaretBlock picks the editable text run that should own a restored caret.
func noteSegmentForCaretBlock(document common.NoteDocument, block int) woxcomponent.NoteDocumentSegment {
	segment := woxcomponent.NoteSegmentAtBlock(document, block)
	if !segment.Structural() {
		return segment
	}
	for _, candidate := range woxcomponent.NoteDocumentSegments(document) {
		if !candidate.Structural() {
			return candidate
		}
	}
	return segment
}
