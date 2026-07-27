package launcher

import (
	"log"

	woxui "wox/ui/runtime"
)

// CommonDeps bundles the minimal cross-cutting dependencies every settings controller needs.
// Controllers never receive *App; they only receive what they need to invalidate, translate, and read theme.
type CommonDeps struct {
	Invalidate func()
	Translate  func(string) string
	IsDev      bool
	Palette    func() uiPalette
	RunOnUI    func(string, func()) error
}

// OnUI applies one controller state transition through the shared UI single-writer boundary.
func (d CommonDeps) OnUI(operation string, fn func()) bool {
	if fn == nil {
		return true
	}
	if d.RunOnUI == nil {
		fn()
		return true
	}
	if err := d.RunOnUI(operation, fn); err != nil {
		log.Printf("dispatch settings controller operation %q: %v", operation, err)
		return false
	}
	return true
}

// sharedEditState holds the single active built-in setting editor session.
// Only one settings domain edits at a time; Begin/End enforce mutual exclusion.
type sharedEditState struct {
	editKey      string
	editor       *woxui.TextEditor
	choicePicker *settingChoicePickerState
	owner        string // controller name currently editing; "" when idle
}

func newSharedEditState() *sharedEditState {
	return &sharedEditState{editor: woxui.NewTextEditor("")}
}

// Begin claims the editor for one owner/key. Returns whether the claim succeeded.
func (s *sharedEditState) Begin(owner, key string) bool {
	if s.owner != "" && s.owner != owner {
		return false
	}
	s.owner = owner
	s.editKey = key
	return true
}

// End releases the editor back to idle.
func (s *sharedEditState) End() {
	s.owner = ""
	s.editKey = ""
	if s.editor != nil {
		s.editor.SetText("", false)
	}
	s.choicePicker = nil
}

// State returns a snapshot of the current edit session (zero-valued when idle).
func (s *sharedEditState) State() (editKey string, editing woxui.TextEditingState, choicePicker *settingChoicePickerSnapshot) {
	editing = woxui.TextEditingState{}
	if s.editor != nil {
		editing = s.editor.State()
	}
	return s.editKey, editing, snapshotSettingChoicePickerLocked(s.choicePicker)
}

// searchResult is one match returned by a controller's Search method.
type searchResult struct {
	ID    string
	Title string
	Tab   string
	Row   int
}

// Searchable is implemented by every settings controller that contributes to global settings search.
type Searchable interface {
	Search(query string) []searchResult
}
