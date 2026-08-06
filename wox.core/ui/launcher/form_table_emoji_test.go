package launcher

import (
	"encoding/json"
	"testing"
)

func TestOpenFormTableEmojiPickerTargetsIconField(t *testing.T) {
	app := trayQueryEditorTestApp(t)
	state := app.settingsTableEditor

	app.openFormTableEmojiPicker(0)
	if state.emojiPicker == nil {
		t.Fatal("emoji picker did not open for the icon field")
	}
	if state.emojiPicker.fieldIndex != 0 {
		t.Fatalf("emoji picker field index = %d, want 0", state.emojiPicker.fieldIndex)
	}
	if state.emojiPicker.initialEmoji != "📋" {
		t.Fatalf("emoji picker initial emoji = %q, want 📋", state.emojiPicker.initialEmoji)
	}

	app.closeFormTableEmojiPicker()
	if state.emojiPicker != nil {
		t.Fatal("emoji picker should close")
	}

	app.openFormTableEmojiPicker(1)
	if state.emojiPicker != nil {
		t.Fatal("emoji picker must not open for a non-woxImage field")
	}
}

func TestChooseFormTableEmojiCommitsAndCloses(t *testing.T) {
	app := trayQueryEditorTestApp(t)
	state := app.settingsTableEditor
	app.openFormTableEmojiPicker(0)

	app.chooseFormTableEmoji("🤖")
	if state.emojiPicker != nil {
		t.Fatal("emoji picker should close after choosing")
	}
	if value := state.rowForm.values["Icon"]; value != "🤖" {
		t.Fatalf("icon after choosing = %q, want 🤖", value)
	}

	app.openFormTableEmojiPicker(0)
	app.chooseFormTableEmoji("")
	if state.emojiPicker == nil {
		t.Fatal("empty emoji should not commit")
	}
}

func TestWoxImageRowFieldIsNotTextEditable(t *testing.T) {
	if formDefinitionTextEditable(formDefinition{Type: "woxImage"}) {
		t.Fatal("woxImage fields must not be text-editable")
	}
}

func TestTrayQueryRowEditorFocusesQueryNotIcon(t *testing.T) {
	app := trayQueryEditorTestApp(t)
	state := app.settingsTableEditor
	if state.rowForm == nil {
		t.Fatal("tray query row editor did not open")
	}
	if state.rowForm.focused == 0 {
		t.Fatal("the icon field must not hold text focus")
	}
	if state.rowForm.editor == nil {
		t.Fatal("the query field should own the row editor text input")
	}
	if definition := state.rowForm.definitions[state.rowForm.focused]; definition.Value.Key != "Query" {
		t.Fatalf("focused row field = %q, want Query", definition.Value.Key)
	}
}

// trayQueryEditorTestApp opens the tray query row editor for the first tray query.
func trayQueryEditorTestApp(t *testing.T) *App {
	t.Helper()
	deps := CommonDeps{}
	form := newHotkeySettingsForm(settingsData{
		MainHotkey:            "Alt+Space",
		SelectionHotkey:       "Alt+Shift+Space",
		TrayQueries:           json.RawMessage(`[{"Icon":{"ImageType":"emoji","ImageData":"📋"},"Query":"clipboard"}]`),
		IsLinuxWaylandSession: false,
	})
	hotkeys := newHotkeySettingsController(deps)
	hotkeys.SetForm(&form)
	app := &App{
		settingsOpen:   true,
		settingTab:     "general",
		hotkeySettings: hotkeys,
		aiSettings:     newAISettingsController(deps),
		pluginSettings: newPluginSettingsController(deps),
		settingsSearch: newSettingsSearchController(deps),
		themeSettings:  newThemeSettingsController(deps),
		sharedEdit:     newSharedEditState(),
	}
	app.openTrayQueryEditor(0)
	if app.settingsTableEditor == nil || app.settingsTableEditor.rowForm == nil {
		t.Fatal("tray query row editor did not open")
	}
	return app
}
