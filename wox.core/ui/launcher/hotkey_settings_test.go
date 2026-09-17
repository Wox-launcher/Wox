package launcher

import (
	"encoding/json"
	"strconv"
	"testing"

	"wox/setting"
	"wox/ui/contract"
	woxui "wox/ui/runtime"
	"wox/util"
)

func TestQueryHotkeyPositionOptionsUseLocalizedNineGridWithIcons(t *testing.T) {
	options := queryHotkeyPositionOptions()
	want := []struct {
		value setting.QueryHotkeyPosition
		label string
	}{
		{setting.QueryHotkeyPositionSystemDefault, "i18n:ui_query_position_system_default"},
		{setting.QueryHotkeyPositionTopLeft, "i18n:ui_query_position_top_left"},
		{setting.QueryHotkeyPositionTopCenter, "i18n:ui_query_position_top_center"},
		{setting.QueryHotkeyPositionTopRight, "i18n:ui_query_position_top_right"},
		{setting.QueryHotkeyPositionMiddleLeft, "i18n:ui_query_position_middle_left"},
		{setting.QueryHotkeyPositionCenter, "i18n:ui_query_position_center"},
		{setting.QueryHotkeyPositionMiddleRight, "i18n:ui_query_position_middle_right"},
		{setting.QueryHotkeyPositionBottomLeft, "i18n:ui_query_position_bottom_left"},
		{setting.QueryHotkeyPositionBottomCenter, "i18n:ui_query_position_bottom_center"},
		{setting.QueryHotkeyPositionBottomRight, "i18n:ui_query_position_bottom_right"},
	}
	if len(options) != len(want) {
		t.Fatalf("position option count = %d, want %d", len(options), len(want))
	}
	for index, expected := range want {
		option := options[index]
		if option.Value != string(expected.value) || option.Label != expected.label {
			t.Fatalf("position option %d = (%q, %q), want (%q, %q)", index, option.Value, option.Label, expected.value, expected.label)
		}
		if option.Icon.ImageType == "" || option.Icon.ImageData == "" {
			t.Fatalf("position option %q has no visual icon", option.Value)
		}
	}
}

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
	form := newGeneralQuerySettingsForm(settingsData{
		TrayQueries:           json.RawMessage(`[{"Icon":{"ImageType":"emoji","ImageData":"📋"},"Query":"clipboard"},{"Icon":{"ImageType":"emoji","ImageData":"🎵"},"Query":"music"}]`),
		IsLinuxWaylandSession: false,
	})
	general := newGeneralSettingsController(deps, newSharedEditState())
	general.SetForm(&form)
	app := &App{
		settingsOpen:    true,
		settingTab:      "general",
		generalSettings: general,
		hotkeySettings:  newHotkeySettingsController(deps),
		aiSettings:      newAISettingsController(deps),
		pluginSettings:  newPluginSettingsController(deps),
		settingsSearch:  newSettingsSearchController(deps),
		themeSettings:   newThemeSettingsController(deps),
		sharedEdit:      newSharedEditState(),
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
	// The General query form must be marked focused so the settings page scrolls the
	// TrayQueries field into view while the row editor is open.
	if !app.generalSettings.FormFocused() {
		t.Fatal("general query settings should be focused so the page keeps the tray query table visible")
	}
	if form := app.generalSettings.Form(); form == nil || form.focused != trayQueryDefinitionIndex(form) {
		t.Fatalf("general form focus = %d, want the TrayQueries field", form.focused)
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
	form := newGeneralQuerySettingsForm(settingsData{
		TrayQueries:           json.RawMessage(`[{"Query":"clipboard"}]`),
		IsLinuxWaylandSession: false,
	})
	general := newGeneralSettingsController(deps, newSharedEditState())
	general.SetForm(&form)
	app := &App{
		settingsOpen:    true,
		settingTab:      "general",
		generalSettings: general,
		hotkeySettings:  newHotkeySettingsController(deps),
		aiSettings:      newAISettingsController(deps),
		pluginSettings:  newPluginSettingsController(deps),
		settingsSearch:  newSettingsSearchController(deps),
		themeSettings:   newThemeSettingsController(deps),
		sharedEdit:      newSharedEditState(),
	}

	app.openTrayQueryEditor(5)

	if app.settingsTableEditor == nil {
		t.Fatal("tray query table should open so the row list is visible")
	}
	if app.settingsTableEditor.rowForm != nil {
		t.Fatal("out-of-range row should not open a row editor")
	}
}

func TestFullscreenHotkeySetting(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, supported := range []bool{false, true} {
			data, err := settingsDataFromContract(contract.GeneralSettings{
				IgnoreHotkeysOnFullscreen:    enabled,
				FullscreenDetectionSupported: supported,
			})
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, item := range settingItems("hotkey", data) {
				if item.key != "IgnoreHotkeysOnFullscreen" {
					continue
				}
				found = true
				if item.value != strconv.FormatBool(enabled) || item.disabled == supported || len(item.choices) != 2 {
					t.Fatalf("fullscreen switch lost its value or capability: %+v", item)
				}
			}
			if found == util.IsMacOS() {
				t.Fatalf("fullscreen setting visibility = %t, macOS = %t", found, util.IsMacOS())
			}
		}
	}
}

func TestGeneralQueryTablesKeyboardNavigation(t *testing.T) {
	app := newApp(false, nil, woxui.NewWindowManager(), newAppInstanceRegistry(), nil, true, "", launcherWindowID)
	defer app.cancel()
	app.settingsOpen = true
	app.settingTab = "general"
	form := newGeneralQuerySettingsForm(settingsData{})
	app.generalSettings.SetForm(&form)
	app.focusBuiltInSettingsSearchTarget("general", "QueryShortcuts")
	if !app.onSettingsKey(woxui.KeyEvent{Key: woxui.KeyArrowDown, Down: true}) || form.focused != 1 {
		t.Fatal("search-focused query table did not navigate to TrayQueries")
	}
	if !app.onSettingsKey(woxui.KeyEvent{Key: woxui.KeyEnter, Down: true}) || app.settingsTableEditor == nil || app.settingsTableEditor.definition.Value.Key != "TrayQueries" {
		t.Fatal("Enter did not open the focused query table")
	}
	app.settingsTableEditor = nil
	app.selectSettingRow(0)
	if app.generalSettings.FormFocused() || app.onGeneralQuerySettingsKey(woxui.KeyEvent{Key: woxui.KeyArrowDown, Down: true}) {
		t.Fatal("query table retained keyboard ownership after selecting a built-in row")
	}
}

func TestLauncherHotkeyUsesRecordedAliases(t *testing.T) {
	if !hotkeyMatches("win+k", woxui.KeyEvent{Key: "k", Modifiers: woxui.KeyModifierMeta, Down: true}) || hotkeyMatches("win+k", woxui.KeyEvent{Key: "k", Down: true}) {
		t.Fatal("Win modifier was lost when matching a recorded shortcut")
	}
	if !hotkeyMatches("ctrl+left", woxui.KeyEvent{Key: woxui.KeyArrowLeft, Modifiers: woxui.KeyModifierControl, Down: true}) {
		t.Fatal("recorded direction key did not match")
	}
}
