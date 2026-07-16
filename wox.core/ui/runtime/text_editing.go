package woxui

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
	e.state = TextEditingState{Text: string(next), Selection: TextSelection{Anchor: caret, Focus: caret}}
	return true
}

// HandleKey applies editing commands and reports whether the event was handled and changed text.
func (e *TextEditor) HandleKey(event KeyEvent) (handled bool, textChanged bool) {
	if e == nil || !event.Down || event.Composing {
		return false, false
	}
	if event.Key == Key("a") && event.Modifiers.HasPrimary() {
		e.SelectAll()
		return true, false
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
	e.state = TextEditingState{Text: string(next), Selection: TextSelection{Anchor: start, Focus: start}}
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
