package widget

import (
	"sync"

	woxui "wox/ui/runtime"
)

// TextEditingController retains one portable editor across immutable widget rebuilds.
type TextEditingController struct {
	mu     sync.Mutex
	editor *woxui.TextEditor
}

// NewTextEditingController creates a controller with its caret at the end of the initial text.
func NewTextEditingController(text string) *TextEditingController {
	return &TextEditingController{editor: woxui.NewTextEditor(text)}
}

// State returns an immutable snapshot of text, selection, and composition.
func (c *TextEditingController) State() woxui.TextEditingState {
	if c == nil {
		return woxui.TextEditingState{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.editor.State()
}

// Text returns the committed text value.
func (c *TextEditingController) Text() string {
	return c.State().Text
}

// SetText replaces the value and updates its selection.
func (c *TextEditingController) SetText(text string, selectAll bool) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.editor.SetText(text, selectAll)
	c.mu.Unlock()
}

// SetCaret moves the caret to a clamped rune offset.
func (c *TextEditingController) SetCaret(offset int) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.editor.SetCaret(offset)
	c.mu.Unlock()
}

// SetSelection replaces the current anchor and focus.
func (c *TextEditingController) SetSelection(anchor, focus int) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.editor.SetSelection(anchor, focus)
	c.mu.Unlock()
}

// SelectAll selects the controller's complete committed value.
func (c *TextEditingController) SelectAll() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.editor.SelectAll()
	c.mu.Unlock()
}

// SelectWordAt selects the Unicode word containing the rune offset.
func (c *TextEditingController) SelectWordAt(offset int) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.editor.SelectWordAt(offset)
	c.mu.Unlock()
}

// SelectLineAt selects the newline-delimited line containing the rune offset.
func (c *TextEditingController) SelectLineAt(offset int) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.editor.SelectLineAt(offset)
	c.mu.Unlock()
}

// InsertText replaces the selection with committed text.
func (c *TextEditingController) InsertText(text string) bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.editor.InsertText(text)
}

// InsertTextSeparate replaces the selection without merging into the previous typing undo batch.
func (c *TextEditingController) InsertTextSeparate(text string) bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.editor.InsertTextSeparate(text)
}

// DeleteSelection removes the active selection range and collapses the caret to its start.
func (c *TextEditingController) DeleteSelection() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.editor.DeleteSelection()
}

// Undo restores the controller's previous committed editing state.
func (c *TextEditingController) Undo() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.editor.Undo()
}

// Redo reapplies the controller's most recently undone editing state.
func (c *TextEditingController) Redo() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.editor.Redo()
}

// SelectedText returns the currently selected substring, or empty when collapsed.
func (c *TextEditingController) SelectedText() string {
	if c == nil {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.editor.SelectedText()
}

// HandleKey applies one portable editing command.
func (c *TextEditingController) HandleKey(event woxui.KeyEvent) (bool, bool) {
	if c == nil {
		return false, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.editor.HandleKey(event)
}

// HandleTextInput applies one committed or composing native input event.
func (c *TextEditingController) HandleTextInput(event woxui.TextInputEvent) bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.editor.HandleTextInput(event)
}

// PreferredX returns the sticky measured horizontal caret X used by vertical navigation, when set.
func (c *TextEditingController) PreferredX() (float32, bool) {
	if c == nil {
		return 0, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.editor.PreferredX()
}

// SetPreferredX remembers the measured horizontal caret X for up/down and page navigation.
func (c *TextEditingController) SetPreferredX(x float32) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.editor.SetPreferredX(x)
	c.mu.Unlock()
}

// ClearPreferredColumn drops any sticky vertical-navigation column.
func (c *TextEditingController) ClearPreferredColumn() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.editor.ClearPreferredColumn()
	c.mu.Unlock()
}
