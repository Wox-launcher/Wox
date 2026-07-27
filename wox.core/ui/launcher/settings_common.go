package launcher

import (
	"sync"

	woxui "wox/ui/runtime"
)

// CommonDeps bundles the minimal cross-cutting dependencies every settings controller needs.
// Controllers never receive *App; they only receive what they need to invalidate, translate, and read theme.
type CommonDeps struct {
	Invalidate func()
	Translate  func(string) string
	IsDev      bool
	Palette    func() uiPalette
}

// sharedEditState holds the single active built-in setting editor session.
// Only one settings domain edits at a time; Begin/End enforce mutual exclusion.
type sharedEditState struct {
	mu           sync.Mutex
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.owner != "" && s.owner != owner {
		return false
	}
	s.owner = owner
	s.editKey = key
	return true
}

// End releases the editor back to idle.
func (s *sharedEditState) End() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.owner = ""
	s.editKey = ""
	if s.editor != nil {
		s.editor.SetText("", false)
	}
	s.choicePicker = nil
}

// State returns a snapshot of the current edit session (zero-valued when idle).
func (s *sharedEditState) State() (editKey string, editing woxui.TextEditingState, choicePicker *settingChoicePickerSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
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
