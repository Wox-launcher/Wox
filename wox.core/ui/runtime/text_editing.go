package woxui

import (
	"runtime"
	"time"
)

const (
	textEditorHistoryLimit    = 100
	textEditorUndoMergeWindow = time.Second
)

type textEditorUndoKind uint8

const (
	textEditorUndoNone textEditorUndoKind = iota
	textEditorUndoInsert
	textEditorUndoDelete
	textEditorUndoReplace
)

// TextSelection stores anchor and focus as rune offsets so UTF-8 editing stays deterministic.
// Movement and deletion snap those offsets onto Unicode grapheme boundaries.
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
	state        TextEditingState
	undo         []TextEditingState
	redo         []TextEditingState
	lastUndoKind textEditorUndoKind
	lastUndoAt   time.Time
	// mergeInsertAt is the caret offset expected for the next typing insert that may merge.
	mergeInsertAt      int
	hasMergeInsert     bool
	preferredX         float32
	hasPreferredColumn bool
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
	e.endUndoMerge()
	e.clearPreferredColumn()
}

// SelectAll selects the complete committed value.
func (e *TextEditor) SelectAll() {
	e.state.Selection = TextSelection{Anchor: 0, Focus: len([]rune(e.state.Text))}
	e.state.Composition = ""
	e.clearPreferredColumn()
}

// SetCaret moves the caret to a clamped grapheme-safe rune offset.
func (e *TextEditor) SetCaret(offset int) {
	offset = e.clampGraphemeOffset(offset, true)
	e.state.Selection = TextSelection{Anchor: offset, Focus: offset}
	e.state.Composition = ""
	e.endUndoMerge()
	e.clearPreferredColumn()
}

// SetSelection replaces the current anchor and focus with clamped grapheme-safe rune offsets.
func (e *TextEditor) SetSelection(anchor, focus int) {
	e.state.Selection = TextSelection{
		Anchor: e.clampGraphemeOffset(anchor, true),
		Focus:  e.clampGraphemeOffset(focus, true),
	}
	e.state.Composition = ""
	e.endUndoMerge()
	e.clearPreferredColumn()
}

// SelectWordAt selects the Unicode word containing the rune offset.
func (e *TextEditor) SelectWordAt(offset int) {
	runes := []rune(e.state.Text)
	if len(runes) == 0 {
		e.SetCaret(0)
		return
	}
	offset = e.clampGraphemeOffset(min(max(0, offset), len(runes)-1), true)
	if offset >= len(runes) {
		offset = len(runes) - 1
	}
	start, end := offset, nextGraphemeBoundary(e.state.Text, offset)
	if isTextWordRune(runes[offset]) {
		for start > 0 && isTextWordRune(runes[start-1]) {
			start = previousGraphemeBoundary(e.state.Text, start)
		}
		for end < len(runes) && isTextWordRune(runes[end]) {
			end = nextGraphemeBoundary(e.state.Text, end)
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

// InsertText replaces the current selection with committed text and merges adjacent typing undos.
func (e *TextEditor) InsertText(text string) bool {
	return e.insertText(text, true)
}

// InsertTextSeparate replaces the selection without merging into the previous typing undo batch.
func (e *TextEditor) InsertTextSeparate(text string) bool {
	return e.insertText(text, false)
}

func (e *TextEditor) insertText(text string, mergeTyping bool) bool {
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
	collapsedInsert := start == end
	// Merge only contiguous collapsed typing at the expected caret; cursor moves break the batch.
	canMerge := mergeTyping && collapsedInsert && e.lastUndoKind == textEditorUndoInsert && e.hasMergeInsert &&
		start == e.mergeInsertAt && time.Since(e.lastUndoAt) <= textEditorUndoMergeWindow
	if canMerge {
		e.lastUndoAt = time.Now()
		e.mergeInsertAt = caret
	} else if collapsedInsert && mergeTyping {
		e.rememberUndoState(textEditorUndoInsert)
		e.mergeInsertAt = caret
		e.hasMergeInsert = true
	} else {
		e.rememberUndoState(textEditorUndoReplace)
		e.endUndoMerge()
	}
	e.state = TextEditingState{Text: string(next), Selection: TextSelection{Anchor: caret, Focus: caret}}
	e.clearPreferredColumn()
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
	e.rememberUndoState(textEditorUndoDelete)
	e.endUndoMerge()
	e.state = TextEditingState{Text: string(next), Selection: TextSelection{Anchor: start, Focus: start}}
	e.clearPreferredColumn()
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
	e.endUndoMerge()
	e.clearPreferredColumn()
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
	e.endUndoMerge()
	e.clearPreferredColumn()
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

// PreferredX returns the sticky measured horizontal caret X used by vertical navigation, when set.
func (e *TextEditor) PreferredX() (float32, bool) {
	if e == nil || !e.hasPreferredColumn {
		return 0, false
	}
	return e.preferredX, true
}

// SetPreferredX remembers the measured horizontal caret X for up/down and page navigation.
func (e *TextEditor) SetPreferredX(x float32) {
	if e == nil {
		return
	}
	e.preferredX = max(float32(0), x)
	e.hasPreferredColumn = true
}

// ClearPreferredColumn drops any sticky vertical-navigation column.
func (e *TextEditor) ClearPreferredColumn() {
	if e == nil {
		return
	}
	e.clearPreferredColumn()
}

// HandleKey applies editing commands and reports whether the event was handled and changed text.
func (e *TextEditor) HandleKey(event KeyEvent) (handled bool, textChanged bool) {
	if e == nil || !event.Down || event.Composing {
		return false, false
	}
	extend := event.Modifiers&KeyModifierShift != 0
	word := event.Modifiers.HasWordModifier()
	line := event.Modifiers.HasLineModifier()
	primary := event.Modifiers.HasPrimary()

	if primary {
		switch event.Key {
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
		case KeyBackspace:
			// macOS Cmd+Backspace deletes to the start of the current hard line.
			if runtime.GOOS == "darwin" {
				return true, e.deleteToLineStart()
			}
		case KeyArrowLeft:
			if runtime.GOOS == "darwin" {
				e.moveToLineBoundary(true, extend)
				return true, false
			}
		case KeyArrowRight:
			if runtime.GOOS == "darwin" {
				e.moveToLineBoundary(false, extend)
				return true, false
			}
		case KeyArrowUp:
			if runtime.GOOS == "darwin" {
				e.moveCaretTo(0, extend)
				return true, false
			}
		case KeyArrowDown:
			if runtime.GOOS == "darwin" {
				e.moveCaretTo(len([]rune(e.state.Text)), extend)
				return true, false
			}
		}
	}

	if word {
		switch event.Key {
		case KeyBackspace:
			return true, e.deleteWordBackward()
		case KeyDelete:
			return true, e.deleteWordForward()
		case KeyArrowLeft:
			e.moveCaretByWord(-1, extend)
			return true, false
		case KeyArrowRight:
			e.moveCaretByWord(1, extend)
			return true, false
		}
	}

	if line {
		switch event.Key {
		case KeyArrowLeft, KeyHome:
			e.moveToLineBoundary(true, extend)
			return true, false
		case KeyArrowRight, KeyEnd:
			e.moveToLineBoundary(false, extend)
			return true, false
		}
	}

	switch event.Key {
	case KeyBackspace:
		return true, e.deleteBackward()
	case KeyDelete:
		return true, e.deleteForward()
	case KeyArrowLeft:
		e.moveCaretByGrapheme(-1, extend)
		return true, false
	case KeyArrowRight:
		e.moveCaretByGrapheme(1, extend)
		return true, false
	case KeyHome:
		// Document start/end; multiline visual-line Home/End is handled by WoxTextField.
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
		start = previousGraphemeBoundary(e.state.Text, start)
	}
	e.replaceRange(runes, start, end, textEditorUndoDelete)
	return true
}

// deleteWordBackward removes the selection or the text segment before the caret.
func (e *TextEditor) deleteWordBackward() bool {
	runes := []rune(e.state.Text)
	start, end := e.selectionBounds(len(runes))
	if start != end {
		e.replaceRange(runes, start, end, textEditorUndoDelete)
		return true
	}
	if start == 0 {
		return false
	}
	start = wordBoundaryBefore(e.state.Text, start)
	e.replaceRange(runes, start, end, textEditorUndoDelete)
	return true
}

// deleteWordForward removes the selection or the text segment after the caret.
func (e *TextEditor) deleteWordForward() bool {
	runes := []rune(e.state.Text)
	start, end := e.selectionBounds(len(runes))
	if start != end {
		e.replaceRange(runes, start, end, textEditorUndoDelete)
		return true
	}
	if end == len(runes) {
		return false
	}
	end = wordBoundaryAfter(e.state.Text, end)
	e.replaceRange(runes, start, end, textEditorUndoDelete)
	return true
}

func (e *TextEditor) deleteForward() bool {
	runes := []rune(e.state.Text)
	start, end := e.selectionBounds(len(runes))
	if start == end {
		if end == len(runes) {
			return false
		}
		end = nextGraphemeBoundary(e.state.Text, end)
	}
	e.replaceRange(runes, start, end, textEditorUndoDelete)
	return true
}

func (e *TextEditor) deleteToLineStart() bool {
	runes := []rune(e.state.Text)
	start, end := e.selectionBounds(len(runes))
	if start != end {
		e.replaceRange(runes, start, end, textEditorUndoDelete)
		return true
	}
	lineStart := hardLineStart(runes, start)
	if lineStart == start {
		return false
	}
	e.replaceRange(runes, lineStart, start, textEditorUndoDelete)
	return true
}

func (e *TextEditor) replaceRange(runes []rune, start, end int, kind textEditorUndoKind) {
	next := append(append(make([]rune, 0, len(runes)-(end-start)), runes[:start]...), runes[end:]...)
	e.rememberUndoState(kind)
	e.endUndoMerge()
	e.state = TextEditingState{Text: string(next), Selection: TextSelection{Anchor: start, Focus: start}}
	e.clearPreferredColumn()
}

func (e *TextEditor) rememberUndoState(kind textEditorUndoKind) {
	if e == nil {
		return
	}
	e.undo = appendTextEditorHistory(e.undo, e.state)
	e.redo = nil
	e.lastUndoKind = kind
	e.lastUndoAt = time.Now()
}

func (e *TextEditor) endUndoMerge() {
	e.lastUndoKind = textEditorUndoNone
	e.hasMergeInsert = false
	e.mergeInsertAt = 0
}

func appendTextEditorHistory(history []TextEditingState, state TextEditingState) []TextEditingState {
	state.Composition = ""
	if len(history) == textEditorHistoryLimit {
		copy(history, history[1:])
		history = history[:textEditorHistoryLimit-1]
	}
	return append(history, state)
}

func (e *TextEditor) moveCaretByGrapheme(delta int, extend bool) {
	selection := e.state.Selection
	if !extend && !selection.Collapsed() {
		if delta < 0 {
			e.moveCaretTo(selection.Start(), false)
		} else {
			e.moveCaretTo(selection.End(), false)
		}
		return
	}
	focus := selection.Focus
	if delta < 0 {
		focus = previousGraphemeBoundary(e.state.Text, focus)
	} else {
		focus = nextGraphemeBoundary(e.state.Text, focus)
	}
	e.moveCaretTo(focus, extend)
}

func (e *TextEditor) moveCaretByWord(delta int, extend bool) {
	selection := e.state.Selection
	if !extend && !selection.Collapsed() {
		if delta < 0 {
			e.moveCaretTo(selection.Start(), false)
		} else {
			e.moveCaretTo(selection.End(), false)
		}
		return
	}
	focus := selection.Focus
	if delta < 0 {
		focus = wordMoveBefore(e.state.Text, focus)
	} else {
		focus = wordMoveAfter(e.state.Text, focus)
	}
	e.moveCaretTo(focus, extend)
}

func (e *TextEditor) moveToLineBoundary(toStart, extend bool) {
	runes := []rune(e.state.Text)
	focus := e.state.Selection.Focus
	if toStart {
		e.moveCaretTo(hardLineStart(runes, focus), extend)
		return
	}
	e.moveCaretTo(hardLineEnd(runes, focus), extend)
}

func (e *TextEditor) moveCaretTo(offset int, extend bool) {
	offset = e.clampGraphemeOffset(offset, true)
	if extend {
		e.state.Selection.Focus = offset
	} else {
		e.state.Selection = TextSelection{Anchor: offset, Focus: offset}
	}
	e.state.Composition = ""
	e.endUndoMerge()
	e.clearPreferredColumn()
}

func (e *TextEditor) selectionBounds(length int) (int, int) {
	start := max(0, min(length, e.state.Selection.Start()))
	end := max(start, min(length, e.state.Selection.End()))
	return start, end
}

func (e *TextEditor) clampGraphemeOffset(offset int, biasBackward bool) int {
	return snapGraphemeBoundary(graphemeBoundaries(e.state.Text), offset, biasBackward)
}

func (e *TextEditor) clearPreferredColumn() {
	e.hasPreferredColumn = false
	e.preferredX = 0
}

func hardLineStart(runes []rune, offset int) int {
	offset = max(0, min(len(runes), offset))
	for offset > 0 && runes[offset-1] != '\n' {
		offset--
	}
	return offset
}

func hardLineEnd(runes []rune, offset int) int {
	offset = max(0, min(len(runes), offset))
	for offset < len(runes) && runes[offset] != '\n' {
		offset++
	}
	return offset
}

// FilterSingleLineNewlines strips carriage returns and newlines for single-line editors.
func FilterSingleLineNewlines(text string) string {
	if text == "" {
		return text
	}
	buf := make([]rune, 0, len([]rune(text)))
	for _, current := range text {
		if current == '\n' || current == '\r' {
			continue
		}
		buf = append(buf, current)
	}
	return string(buf)
}

// MaskProtectedText replaces each user-perceived character with a bullet for password display.
func MaskProtectedText(text string) string {
	if text == "" {
		return ""
	}
	return repeatBullets(graphemeCount(text))
}

// MapSelectionToProtectedDisplay remaps rune selection offsets onto the bullet-masked display.
func MapSelectionToProtectedDisplay(text string, selection TextSelection) TextSelection {
	return TextSelection{
		Anchor: runeOffsetToGraphemeIndex(text, selection.Anchor),
		Focus:  runeOffsetToGraphemeIndex(text, selection.Focus),
	}
}

// MapProtectedDisplayOffsetToRune maps a bullet-display grapheme index back onto the real text.
func MapProtectedDisplayOffsetToRune(text string, displayOffset int) int {
	return graphemeIndexToRuneOffset(text, displayOffset)
}

func repeatBullets(count int) string {
	if count <= 0 {
		return ""
	}
	bullets := make([]rune, count)
	for index := range bullets {
		bullets[index] = '•'
	}
	return string(bullets)
}
