package woxui

import "unicode"

const textEditorHistoryLimit = 100

// TextSelection stores anchor and focus as rune offsets so UTF-8 editing stays deterministic.
type TextSelection struct {
	Anchor int
	Focus  int
}

// Start returns the lower normalized selection boundary.
func (s TextSelection) Start() int {
	return min(s.Anchor, s.Focus)
}

// End returns the upper normalized selection boundary.
func (s TextSelection) End() int {
	return max(s.Anchor, s.Focus)
}

// Collapsed reports whether the selection is only a caret.
func (s TextSelection) Collapsed() bool {
	return s.Anchor == s.Focus
}

// TextEditingState is an immutable snapshot of committed text, selection, and marked text.
type TextEditingState struct {
	Text        string
	Selection   TextSelection
	Composition string
}

// TextEditor applies portable key and IME events to one UTF-8 value.
type TextEditor struct {
	state TextEditingState
	undo  []TextEditingState
	redo  []TextEditingState
}

// NewTextEditor creates an editor with its caret at the end of text.
func NewTextEditor(text string) *TextEditor {
	editor := &TextEditor{}
	editor.SetText(text, false)
	return editor
}

// State returns a copy of the current editing state.
func (e *TextEditor) State() TextEditingState {
	if e == nil {
		return TextEditingState{}
	}
	return e.state
}

// SetText replaces the value and either selects it or moves the caret to its end.
func (e *TextEditor) SetText(text string, selectAll bool) {
	length := len([]rune(text))
	selection := TextSelection{Anchor: length, Focus: length}
	if selectAll {
		selection.Anchor = 0
	}
	e.state = TextEditingState{Text: text, Selection: selection}
	e.undo = nil
	e.redo = nil
}

// SelectAll selects the complete committed value.
func (e *TextEditor) SelectAll() {
	e.state.Selection = TextSelection{Anchor: 0, Focus: len([]rune(e.state.Text))}
	e.state.Composition = ""
}

// SetCaret moves the caret to a clamped rune offset.
func (e *TextEditor) SetCaret(offset int) {
	offset = max(0, min(len([]rune(e.state.Text)), offset))
	e.state.Selection = TextSelection{Anchor: offset, Focus: offset}
	e.state.Composition = ""
}

// SetSelection replaces the current anchor and focus with clamped rune offsets.
func (e *TextEditor) SetSelection(anchor, focus int) {
	length := len([]rune(e.state.Text))
	e.state.Selection = TextSelection{Anchor: max(0, min(length, anchor)), Focus: max(0, min(length, focus))}
	e.state.Composition = ""
}

// SelectWordAt selects the Unicode word containing the rune offset.
func (e *TextEditor) SelectWordAt(offset int) {
	runes := []rune(e.state.Text)
	if len(runes) == 0 {
		e.SetCaret(0)
		return
	}
	offset = min(max(0, offset), len(runes)-1)
	start, end := offset, offset+1
	if isTextWordRune(runes[offset]) {
		for start > 0 && isTextWordRune(runes[start-1]) {
			start--
		}
		for end < len(runes) && isTextWordRune(runes[end]) {
			end++
		}
	}
	e.SetSelection(start, end)
}

// SelectLineAt selects the newline-delimited line containing the rune offset.
func (e *TextEditor) SelectLineAt(offset int) {
	runes := []rune(e.state.Text)
	offset = min(max(0, offset), len(runes))
	start, end := offset, offset
	for start > 0 && runes[start-1] != '\n' {
		start--
	}
	for end < len(runes) && runes[end] != '\n' {
		end++
	}
	e.SetSelection(start, end)
}

// InsertText replaces the current selection with committed text.
func (e *TextEditor) InsertText(text string) bool {
	if e == nil || text == "" {
		return false
	}
	runes := []rune(e.state.Text)
	start, end := e.selectionBounds(len(runes))
	inserted := []rune(text)
	next := make([]rune, 0, len(runes)-(end-start)+len(inserted))
	next = append(next, runes[:start]...)
	next = append(next, inserted...)
	next = append(next, runes[end:]...)
	caret := start + len(inserted)
	e.rememberUndoState()
	e.state = TextEditingState{Text: string(next), Selection: TextSelection{Anchor: caret, Focus: caret}}
	return true
}

// DeleteSelection removes the active selection range, collapsing the caret to its start.
// Returns false when the selection is collapsed or the editor is nil.
func (e *TextEditor) DeleteSelection() bool {
	if e == nil {
		return false
	}
	runes := []rune(e.state.Text)
	start, end := e.selectionBounds(len(runes))
	if start >= end {
		return false
	}
	next := append(append(make([]rune, 0, len(runes)-(end-start)), runes[:start]...), runes[end:]...)
	e.rememberUndoState()
	e.state = TextEditingState{Text: string(next), Selection: TextSelection{Anchor: start, Focus: start}}
	return true
}

// Undo restores the previous committed text and selection state.
func (e *TextEditor) Undo() bool {
	if e == nil || len(e.undo) == 0 {
		return false
	}
	previous := e.undo[len(e.undo)-1]
	e.undo = e.undo[:len(e.undo)-1]
	e.redo = appendTextEditorHistory(e.redo, e.state)
	e.state = previous
	e.state.Composition = ""
	return true
}

// Redo reapplies the most recently undone committed text and selection state.
func (e *TextEditor) Redo() bool {
	if e == nil || len(e.redo) == 0 {
		return false
	}
	next := e.redo[len(e.redo)-1]
	e.redo = e.redo[:len(e.redo)-1]
	e.undo = appendTextEditorHistory(e.undo, e.state)
	e.state = next
	e.state.Composition = ""
	return true
}

// SelectedText returns the currently selected substring, or empty when collapsed.
func (e *TextEditor) SelectedText() string {
	if e == nil {
		return ""
	}
	runes := []rune(e.state.Text)
	start, end := e.selectionBounds(len(runes))
	if start >= end {
		return ""
	}
	return string(runes[start:end])
}

// HandleKey applies editing commands and reports whether the event was handled and changed text.
func (e *TextEditor) HandleKey(event KeyEvent) (handled bool, textChanged bool) {
	if e == nil || !event.Down || event.Composing {
		return false, false
	}
	if event.Modifiers.HasPrimary() {
		switch event.Key {
		case KeyBackspace:
			return true, e.deleteWordBackward()
		case Key("a"):
			e.SelectAll()
			return true, false
		case Key("z"):
			if event.Modifiers&KeyModifierShift != 0 {
				return true, e.Redo()
			}
			return true, e.Undo()
		case Key("y"):
			return true, e.Redo()
		}
	}
	extend := event.Modifiers&KeyModifierShift != 0
	switch event.Key {
	case KeyBackspace:
		return true, e.deleteBackward()
	case KeyDelete:
		return true, e.deleteForward()
	case KeyArrowLeft:
		e.moveCaret(-1, extend)
		return true, false
	case KeyArrowRight:
		e.moveCaret(1, extend)
		return true, false
	case KeyHome:
		e.moveCaretTo(0, extend)
		return true, false
	case KeyEnd:
		e.moveCaretTo(len([]rune(e.state.Text)), extend)
		return true, false
	default:
		return false, false
	}
}

// HandleTextInput applies committed or composing input and reports a committed text change.
func (e *TextEditor) HandleTextInput(event TextInputEvent) bool {
	if e == nil {
		return false
	}
	if event.Kind == TextInputCompose {
		e.state.Composition = event.Text
		return false
	}
	if event.Text == "" {
		e.state.Composition = ""
		return false
	}
	return e.InsertText(event.Text)
}

func (e *TextEditor) deleteBackward() bool {
	runes := []rune(e.state.Text)
	start, end := e.selectionBounds(len(runes))
	if start == end {
		if start == 0 {
			return false
		}
		start--
	}
	e.replaceRange(runes, start, end)
	return true
}

// deleteWordBackward removes the selection or the text segment before the caret.
func (e *TextEditor) deleteWordBackward() bool {
	runes := []rune(e.state.Text)
	start, end := e.selectionBounds(len(runes))
	if start != end {
		e.replaceRange(runes, start, end)
		return true
	}
	if start == 0 {
		return false
	}

	for start > 0 && unicode.IsSpace(runes[start-1]) {
		start--
	}
	if start > 0 {
		word := isTextWordRune(runes[start-1])
		for start > 0 && !unicode.IsSpace(runes[start-1]) && isTextWordRune(runes[start-1]) == word {
			start--
		}
	}
	e.replaceRange(runes, start, end)
	return true
}

func (e *TextEditor) deleteForward() bool {
	runes := []rune(e.state.Text)
	start, end := e.selectionBounds(len(runes))
	if start == end {
		if end == len(runes) {
			return false
		}
		end++
	}
	e.replaceRange(runes, start, end)
	return true
}

func (e *TextEditor) replaceRange(runes []rune, start, end int) {
	next := append(append(make([]rune, 0, len(runes)-(end-start)), runes[:start]...), runes[end:]...)
	e.rememberUndoState()
	e.state = TextEditingState{Text: string(next), Selection: TextSelection{Anchor: start, Focus: start}}
}

func (e *TextEditor) rememberUndoState() {
	if e == nil {
		return
	}
	e.undo = appendTextEditorHistory(e.undo, e.state)
	e.redo = nil
}

func appendTextEditorHistory(history []TextEditingState, state TextEditingState) []TextEditingState {
	state.Composition = ""
	if len(history) == textEditorHistoryLimit {
		copy(history, history[1:])
		history = history[:textEditorHistoryLimit-1]
	}
	return append(history, state)
}

func (e *TextEditor) moveCaret(delta int, extend bool) {
	selection := e.state.Selection
	if !extend && !selection.Collapsed() {
		if delta < 0 {
			e.moveCaretTo(selection.Start(), false)
		} else {
			e.moveCaretTo(selection.End(), false)
		}
		return
	}
	e.moveCaretTo(selection.Focus+delta, extend)
}

func (e *TextEditor) moveCaretTo(offset int, extend bool) {
	length := len([]rune(e.state.Text))
	offset = max(0, min(length, offset))
	if extend {
		e.state.Selection.Focus = offset
	} else {
		e.state.Selection = TextSelection{Anchor: offset, Focus: offset}
	}
	e.state.Composition = ""
}

func (e *TextEditor) selectionBounds(length int) (int, int) {
	start := max(0, min(length, e.state.Selection.Start()))
	end := max(start, min(length, e.state.Selection.End()))
	return start, end
}

func isTextWordRune(current rune) bool {
	return unicode.IsLetter(current) || unicode.IsDigit(current) || unicode.IsMark(current) || current == '_'
}
