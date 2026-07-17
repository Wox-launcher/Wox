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

// InsertText replaces the selection with committed text.
func (c *TextEditingController) InsertText(text string) bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.editor.InsertText(text)
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
