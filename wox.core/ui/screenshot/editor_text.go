package screenshot

import (
	"time"

	"wox/util/clipboard"
)

const (
	screenshotEditorTextMultiTapInterval = 500 * time.Millisecond
	screenshotEditorTextMultiTapDistance = float32(4)
)

// resetTextEditorLocked replaces the in-progress label editor and clears tap/drag selection.
func (state *screenshotEditorOverlayState) resetTextEditorLocked(text string) {
	state.textEditor = NewTextEditor(text)
	state.textDraft = text
	state.textMarked = ""
	state.textCaret = len([]rune(text))
	if text == "" {
		state.textCaret = 0
	}
	state.textSelecting = false
	state.textTapCount = 0
	state.textTapAt = time.Time{}
	state.textTapPoint = Point{}
}

func (state *screenshotEditorOverlayState) ensureTextEditorLocked() *TextEditor {
	if state.textEditor == nil {
		state.resetTextEditorLocked(state.textDraft)
		if state.textCaret > 0 {
			state.textEditor.SetCaret(state.textCaret)
		}
	}
	return state.textEditor
}

func (state *screenshotEditorOverlayState) syncTextEditorLocked() {
	if state.textEditor == nil {
		return
	}
	snapshot := state.textEditor.State()
	state.textDraft = snapshot.Text
	state.textMarked = snapshot.Composition
	state.textCaret = snapshot.Selection.Focus
}

func (state *screenshotEditorOverlayState) editingTextAnnotationLocked() screenshotEditorAnnotation {
	preview, _ := screenshotEditorTextEditingValue(state.textDraft, state.textMarked, state.textCaret)
	if state.textEditor != nil {
		preview, _ = screenshotEditorTextEditingPreview(state.textEditor.State())
	}
	return screenshotEditorAnnotation{
		tool: screenshotEditorToolText, start: state.textPosition, text: preview,
		color: state.annotationColor, fontSize: state.textFontSize,
	}
}

// editingTextContainsLocked reports whether the pointer is still inside the label being typed.
func (state *screenshotEditorOverlayState) editingTextContainsLocked(point Point) bool {
	if !state.textEditing {
		return false
	}
	annotation := state.editingTextAnnotationLocked()
	return screenshotEditorAnnotationContains(annotation, point, state.uiScale)
}

func (state *screenshotEditorOverlayState) noteTextTapLocked(point Point) int {
	now := time.Now()
	if !state.textTapAt.IsZero() && now.Sub(state.textTapAt) <= screenshotEditorTextMultiTapInterval {
		dx, dy := point.X-state.textTapPoint.X, point.Y-state.textTapPoint.Y
		if dx*dx+dy*dy <= screenshotEditorTextMultiTapDistance*screenshotEditorTextMultiTapDistance {
			state.textTapCount++
			if state.textTapCount > 3 {
				state.textTapCount = 1
			}
			state.textTapAt = now
			state.textTapPoint = point
			return state.textTapCount
		}
	}
	state.textTapCount = 1
	state.textTapAt = now
	state.textTapPoint = point
	return 1
}

// beginTextSelectionAtLocked applies caret, word, or line selection for consecutive clicks on a label.
func (state *screenshotEditorOverlayState) beginTextSelectionAtLocked(point Point) {
	editor := state.ensureTextEditorLocked()
	annotation := state.editingTextAnnotationLocked()
	offset := screenshotEditorTextCaretIndex(
		annotation.text,
		point.X-annotation.start.X,
		screenshotEditorAnnotationRenderedFontSize(annotation, state.uiScale),
	)
	switch state.noteTextTapLocked(point) {
	case 2:
		editor.SelectWordAt(offset)
		state.textSelecting = false
	case 3:
		editor.SelectLineAt(offset)
		state.textSelecting = false
	default:
		editor.SetCaret(offset)
		state.textSelecting = true
	}
	state.syncTextEditorLocked()
	state.showCaretLocked()
	state.pointerCursor = PointerCursorText
}

func (state *screenshotEditorOverlayState) extendTextSelectionLocked(point Point) {
	editor := state.ensureTextEditorLocked()
	annotation := state.editingTextAnnotationLocked()
	focus := screenshotEditorTextCaretIndex(
		annotation.text,
		point.X-annotation.start.X,
		screenshotEditorAnnotationRenderedFontSize(annotation, state.uiScale),
	)
	editor.SetSelection(editor.State().Selection.Anchor, focus)
	state.syncTextEditorLocked()
	state.showCaretLocked()
}

// handleTextEditingKeyLocked applies portable text commands while a label is being edited.
func (state *screenshotEditorOverlayState) handleTextEditingKeyLocked(event KeyEvent) (handled bool, clipboardWrite string, clipboardRead bool) {
	if !state.textEditing || event.Composing {
		return false, "", false
	}
	editor := state.ensureTextEditorLocked()
	if event.Modifiers.HasPrimary() {
		switch event.Key {
		case Key("c"):
			if selected := editor.SelectedText(); selected != "" {
				return true, selected, false
			}
			return true, "", false
		case Key("x"):
			selected := editor.SelectedText()
			if selected != "" && editor.DeleteSelection() {
				state.syncTextEditorLocked()
				state.showCaretLocked()
				return true, selected, false
			}
			return true, "", false
		case Key("v"):
			return true, "", true
		}
	}
	handled, _ = editor.HandleKey(event)
	if handled {
		state.syncTextEditorLocked()
		state.showCaretLocked()
	}
	return handled, "", false
}

func (state *screenshotEditorOverlayState) pasteTextEditing(text string) bool {
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.textEditing {
		return false
	}
	text = FilterSingleLineNewlines(text)
	if text == "" {
		return true
	}
	if state.ensureTextEditorLocked().InsertTextSeparate(text) {
		state.syncTextEditorLocked()
		state.showCaretLocked()
	}
	return true
}

func (state *screenshotEditorOverlayState) writeTextEditingClipboard(text string) {
	if text == "" {
		return
	}
	if state.writeClipboardText != nil {
		_ = state.writeClipboardText(text)
		return
	}
	_ = clipboard.WriteText(text)
}

func (state *screenshotEditorOverlayState) readTextEditingClipboard() string {
	if state.readClipboardText != nil {
		text, err := state.readClipboardText()
		if err != nil {
			return ""
		}
		return text
	}
	text, err := clipboard.ReadText()
	if err != nil {
		return ""
	}
	return text
}

// screenshotEditorTextEditingPreview inserts IME composition at the caret or over the active selection.
func screenshotEditorTextEditingPreview(state TextEditingState) (string, string) {
	runes := []rune(state.Text)
	start := min(max(0, state.Selection.Start()), len(runes))
	end := min(max(start, state.Selection.End()), len(runes))
	focus := min(max(0, state.Selection.Focus), len(runes))
	if state.Composition == "" {
		return state.Text, string(runes[:focus])
	}
	prefix := string(runes[:start]) + state.Composition
	return prefix + string(runes[end:]), prefix
}

func drawScreenshotEditorTextSelection(displayList *DisplayList, origin Point, text string, selection TextSelection, fontSize float32, color Color, uiScale float32) {
	runes := []rune(text)
	start := min(max(0, selection.Start()), len(runes))
	end := min(max(start, selection.End()), len(runes))
	if start >= end {
		return
	}
	left := origin.X + screenshotEditorEstimatedTextWidth(string(runes[:start]), fontSize)
	right := origin.X + screenshotEditorEstimatedTextWidth(string(runes[:end]), fontSize)
	highlight := color
	highlight.A = 70
	displayList.FillRect(Rect{
		X: left, Y: origin.Y,
		Width: max(float32(1), right-left), Height: fontSize + 4*max(float32(1), uiScale),
	}, highlight)
}
