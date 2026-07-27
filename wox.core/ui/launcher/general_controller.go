package launcher

import (
	"sync"

	woxui "wox/ui/runtime"
)

// generalSettingsSnapshot is the immutable General tab state consumed by the view layer.
// It owns the full settingsData struct (general, appearance, network, data, cloud domains)
// plus the shared built-in editor session and the language choice list. Other domain
// controllers keep their own narrow mirrors (e.g. networkSettingsController.proxyEnabled)
// and read their slice via ApplyData calls driven by App.reloadSettings.
type generalSettingsSnapshot struct {
	EditKey      string
	Editing      woxui.TextEditingState
	ChoicePicker *settingChoicePickerSnapshot
	Languages    []settingChoice
	Data         settingsData
}

// generalSettingsController owns the General tab state and the single shared built-in
// settings editor session. The shared editor is owned here (not on App) so every
// settings domain can claim it via BeginEdit while mutual exclusion is enforced by
// sharedEditState. The full settingsData struct lives on this controller; other
// controllers that need their slice read it through ApplyData called from
// App.reloadSettings.
type generalSettingsController struct {
	deps   CommonDeps
	shared *sharedEditState
	// Background query, glance, and cloud work read the immutable settings snapshot.
	dataMu    sync.RWMutex
	data      settingsData
	languages []settingChoice
}

func newGeneralSettingsController(deps CommonDeps, shared *sharedEditState) *generalSettingsController {
	return &generalSettingsController{deps: deps, shared: shared}
}

// ApplyData replaces the full settingsData owned by the general controller. Called by
// App.reloadSettings after the full settings payload is fetched from core, on both
// initial load and after every saveSetting round-trip (which calls reloadSettings).
func (c *generalSettingsController) ApplyData(data settingsData) {
	c.dataMu.Lock()
	c.data = data
	c.dataMu.Unlock()
}

// Data returns a copy of the full settingsData. Cross-domain readers (query, glance,
// theme, data, cloud, ai) use this to read their slice without owning the struct.
func (c *generalSettingsController) Data() settingsData {
	c.dataMu.RLock()
	defer c.dataMu.RUnlock()
	return c.data
}

// SetLanguages replaces the language choice list. Called by App.reloadSettings after
// it fetches the available languages from core.
func (c *generalSettingsController) SetLanguages(languages []settingChoice) {
	c.languages = append([]settingChoice(nil), languages...)
}

// Languages returns a copy of the language choice list.
func (c *generalSettingsController) Languages() []settingChoice {
	return append([]settingChoice(nil), c.languages...)
}

// BeginEdit claims the shared built-in editor for one owner/key. Returns whether the
// claim succeeded. Only one settings domain edits at a time; the shared mutex enforces
// mutual exclusion across controllers.
func (c *generalSettingsController) BeginEdit(owner, key string) bool {
	return c.shared.Begin(owner, key)
}

// EndEdit releases the shared editor back to idle.
func (c *generalSettingsController) EndEdit() {
	c.shared.End()
}

// EditState returns a snapshot of the current shared edit session. When idle the
// returned editKey is empty and the editor state is zero-valued.
func (c *generalSettingsController) EditState() (string, woxui.TextEditingState, *settingChoicePickerSnapshot) {
	return c.shared.State()
}

// EditKey returns the currently editing key, or "" when idle. Convenience for readers
// that only need the key (e.g. focus checks in the settings adapter).
func (c *generalSettingsController) EditKey() string {
	return c.shared.editKey
}

// Editor returns the shared text editor. The editor is always non-nil (allocated in
// newSharedEditState), so callers can safely read its state even when idle.
func (c *generalSettingsController) Editor() *woxui.TextEditor {
	return c.shared.editor
}

// SetChoicePicker stores the active choice picker state inside the shared edit session.
// Passing nil clears an open picker. The choice picker lives on the shared state so
// EndEdit naturally clears it when the session ends.
func (c *generalSettingsController) SetChoicePicker(state *settingChoicePickerState) {
	c.shared.choicePicker = state
	if state == nil && c.shared.editor != nil {
		c.shared.editor.SetText("", false)
	}
}

// ChoicePicker returns the active choice picker state, or nil when none is open.
func (c *generalSettingsController) ChoicePicker() *settingChoicePickerState {
	return c.shared.choicePicker
}

// SetEditText forwards a new text value to the shared editor when the active key
// matches. Used by setBuiltInSettingEditValue and browseBuiltInSettingFile to keep
// the shared editor in sync without exposing the shared state directly.
func (c *generalSettingsController) SetEditText(key, value string) {
	if c.shared.editKey == key && c.shared.editor != nil {
		c.shared.editor.SetText(value, false)
	}
}

// StartEdit claims the editor for the general owner and seeds the editor with the
// given value. Returns whether the claim succeeded. When caret >= 0 the caret is
// moved to that offset after seeding.
func (c *generalSettingsController) StartEdit(key, value string, caret int) bool {
	if c.shared.owner != "" && c.shared.owner != "general" {
		return false
	}
	c.shared.owner = "general"
	c.shared.editKey = key
	if c.shared.editor != nil {
		c.shared.editor.SetText(value, false)
		if caret >= 0 {
			c.shared.editor.SetCaret(caret)
		}
	}
	c.shared.choicePicker = nil
	return true
}

// Update runs fn under the controller's lock so callers can mutate specific fields
// of the owned settingsData. Cross-domain writers (hotkey recording, AI raw apply,
// theme apply, hotkey settings raw apply) use this to patch their slice without
// taking ownership of the whole struct.
func (c *generalSettingsController) Update(fn func(data *settingsData)) {
	c.dataMu.Lock()
	defer c.dataMu.Unlock()
	fn(&c.data)
}

// Snapshot returns a copy of the general tab state for the view layer.
func (c *generalSettingsController) Snapshot() generalSettingsSnapshot {
	c.dataMu.RLock()
	defer c.dataMu.RUnlock()
	editKey, editing, choicePicker := c.shared.State()
	return generalSettingsSnapshot{
		EditKey:      editKey,
		Editing:      editing,
		ChoicePicker: choicePicker,
		Languages:    append([]settingChoice(nil), c.languages...),
		Data:         c.data,
	}
}
