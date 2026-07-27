package launcher

import (
	"testing"

	woxui "wox/ui/runtime"
)

func newGeneralControllerForTest() *generalSettingsController {
	deps := CommonDeps{
		Invalidate: func() {},
		Translate:  func(s string) string { return s },
	}
	shared := newSharedEditState()
	return newGeneralSettingsController(deps, shared)
}

func TestGeneralControllerApplyData(t *testing.T) {
	c := newGeneralControllerForTest()
	c.ApplyData(settingsData{UsePinYin: true, LangCode: "en"})
	data := c.Data()
	if !data.UsePinYin {
		t.Fatalf("expected UsePinYin=true after ApplyData, got false")
	}
	if data.LangCode != "en" {
		t.Fatalf("expected LangCode=en after ApplyData, got %q", data.LangCode)
	}

	// Re-apply replaces the whole struct.
	c.ApplyData(settingsData{UsePinYin: false, LangCode: "zh"})
	data = c.Data()
	if data.UsePinYin {
		t.Fatalf("expected UsePinYin=false after second ApplyData, got true")
	}
	if data.LangCode != "zh" {
		t.Fatalf("expected LangCode=zh after second ApplyData, got %q", data.LangCode)
	}
}

func TestGeneralControllerSharedEditBeginEnd(t *testing.T) {
	c := newGeneralControllerForTest()
	if !c.BeginEdit("general", "LangCode") {
		t.Fatalf("first BeginEdit(general, LangCode) should succeed")
	}
	// A different owner cannot claim while general owns the session.
	if c.BeginEdit("plugins", "X") {
		t.Fatalf("BeginEdit(plugins, X) should fail while general owns the session")
	}
	// Same owner reclaiming is allowed and updates the key.
	if !c.BeginEdit("general", "UsePinYin") {
		t.Fatalf("BeginEdit(general, UsePinYin) re-claim should succeed")
	}
	c.EndEdit()
	// After End the session is free for a different owner.
	if !c.BeginEdit("plugins", "X") {
		t.Fatalf("BeginEdit(plugins, X) should succeed after EndEdit")
	}
	c.EndEdit()
}

func TestGeneralControllerEditState(t *testing.T) {
	c := newGeneralControllerForTest()
	if !c.BeginEdit("general", "LangCode") {
		t.Fatalf("BeginEdit(general, LangCode) should succeed")
	}
	if got := c.EditKey(); got != "LangCode" {
		t.Fatalf("EditKey() = %q, want LangCode", got)
	}
	if editor := c.Editor(); editor == nil {
		t.Fatalf("Editor() should not be nil while editing")
	}
	key, _, choicePicker := c.EditState()
	if key != "LangCode" {
		t.Fatalf("EditState key = %q, want LangCode", key)
	}
	if choicePicker != nil {
		t.Fatalf("EditState choicePicker should be nil when no picker is open")
	}
	c.EndEdit()
	if got := c.EditKey(); got != "" {
		t.Fatalf("EditKey() after EndEdit = %q, want empty", got)
	}
}

func TestGeneralControllerLanguages(t *testing.T) {
	c := newGeneralControllerForTest()
	c.SetLanguages([]settingChoice{{value: "en", label: "English"}, {value: "zh", label: "Chinese"}})
	got := c.Languages()
	if len(got) != 2 {
		t.Fatalf("Languages() len = %d, want 2", len(got))
	}
	if got[0].value != "en" || got[1].value != "zh" {
		t.Fatalf("Languages() = %+v", got)
	}

	// Mutating the returned slice must not affect the controller state.
	got[0].value = "mutated"
	again := c.Languages()
	if again[0].value != "en" {
		t.Fatalf("Languages() returned slice was not a copy; controller state mutated to %q", again[0].value)
	}
}

func TestGeneralControllerSetChoicePicker(t *testing.T) {
	c := newGeneralControllerForTest()
	state := &settingChoicePickerState{item: settingItem{key: "LangCode"}, anchor: woxui.Rect{X: 1, Y: 2}}
	c.SetChoicePicker(state)
	if got := c.ChoicePicker(); got != state {
		t.Fatalf("ChoicePicker() returned %p, want the same instance %p", got, state)
	}

	// SetChoicePicker(nil) clears the picker.
	c.SetChoicePicker(nil)
	if got := c.ChoicePicker(); got != nil {
		t.Fatalf("ChoicePicker() after SetChoicePicker(nil) should be nil, got %p", got)
	}
}
