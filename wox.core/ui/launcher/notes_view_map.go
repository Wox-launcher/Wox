package launcher

import (
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
	// Paragraph spacing exists only in the rich preview, so a raw Y from one view
	// does not identify the same line in the other.
	if noteRunsHaveParagraphGap(sourceRuns) != noteRunsHaveParagraphGap(destRuns) {
		return 0, false
	}
	lineHeight := notesEditorLineHeight * c.zoom
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
	width := c.noteContentWidth()
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

func noteRunsHaveParagraphGap(runs []woxcomponent.TextFieldRichRun) bool {
	for _, run := range runs {
		if run.BlockGap > 0 {
			return true
		}
	}
	return false
}

// noteSourceLooksWrapped is a mid-block hard wrap, not a paragraph gap.
func noteSourceLooksWrapped(source string, offset int) bool {
	runes := []rune(source)
	if offset < 0 || offset >= len(runes) || runes[offset] != '\n' {
		return false
	}
	return offset+1 >= len(runes) || runes[offset+1] != '\n'
}
