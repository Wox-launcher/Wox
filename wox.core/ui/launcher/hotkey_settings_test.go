package launcher

import (
	"encoding/json"
	"testing"
)

func TestTrayQueryRowIndexFromParam(t *testing.T) {
	if index, ok := trayQueryRowIndexFromParam("tray_queries:2"); !ok || index != 2 {
		t.Fatalf("tray_queries:2 = %d, %v; want 2, true", index, ok)
	}
	if _, ok := trayQueryRowIndexFromParam(""); ok {
		t.Fatal("empty param should not match")
	}
	if _, ok := trayQueryRowIndexFromParam("plugins:2"); ok {
		t.Fatal("unrelated param should not match")
	}
	if _, ok := trayQueryRowIndexFromParam("tray_queries:abc"); ok {
		t.Fatal("non-numeric index should not match")
	}
	if _, ok := trayQueryRowIndexFromParam("tray_queries:-1"); ok {
		t.Fatal("negative index should not match")
	}
}

func TestOpenTrayQueryEditorOpensSelectedRow(t *testing.T) {
	deps := CommonDeps{}
	form := newHotkeySettingsForm(settingsData{
		MainHotkey:            "Alt+Space",
		SelectionHotkey:       "Alt+Shift+Space",
		TrayQueries:           json.RawMessage(`[{"Icon":{"ImageType":"emoji","ImageData":"📋"},"Query":"clipboard"},{"Icon":{"ImageType":"emoji","ImageData":"🎵"},"Query":"music"}]`),
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

	app.openTrayQueryEditor(1)

	if app.settingsTableEditor == nil {
		t.Fatal("tray query editor did not open a form table")
	}
	state := app.settingsTableEditor
	if state.definition.Value.Key != "TrayQueries" {
		t.Fatalf("opened table = %q, want TrayQueries", state.definition.Value.Key)
	}
	if state.rowForm == nil {
		t.Fatal("tray query row editor did not open")
	}
	if state.rowIndex != 1 {
		t.Fatalf("editing row index = %d, want 1", state.rowIndex)
	}
	if !state.rowEditorOnly {
		t.Fatal("direct row editor should close the overlay when it exits")
	}
	if value := state.rowForm.values["Query"]; value != "music" {
		t.Fatalf("editing Query = %q, want music", value)
	}
	// The hotkey form must be marked focused so the settings page scrolls the
	// TrayQueries field into view while the row editor is open.
	if !app.hotkeySettings.Focused() {
		t.Fatal("hotkey settings should be focused so the page keeps the tray query table visible")
	}
	if form := app.hotkeySettings.Form(); form == nil || form.focused != trayQueryDefinitionIndex(form) {
		t.Fatalf("hotkey form focus = %d, want the TrayQueries field", form.focused)
	}
}

// trayQueryDefinitionIndex returns the form definition index of the TrayQueries table.
func trayQueryDefinitionIndex(form *formFieldsState) int {
	for index, definition := range form.definitions {
		if definition.Value.Key == "TrayQueries" {
			return index
		}
	}
	return -1
}

func TestOpenTrayQueryEditorIgnoresInvalidRow(t *testing.T) {
	deps := CommonDeps{}
	form := newHotkeySettingsForm(settingsData{
		MainHotkey:            "Alt+Space",
		SelectionHotkey:       "Alt+Shift+Space",
		TrayQueries:           json.RawMessage(`[{"Query":"clipboard"}]`),
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

	app.openTrayQueryEditor(5)

	if app.settingsTableEditor == nil {
		t.Fatal("tray query table should open so the row list is visible")
	}
	if app.settingsTableEditor.rowForm != nil {
		t.Fatal("out-of-range row should not open a row editor")
	}
}
