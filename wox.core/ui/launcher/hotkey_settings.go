package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

type ignoredHotkeyApp struct {
	Name     string
	Identity string
	Path     string
	Icon     woxImage
}

// buildHotkeySettingsPage prepares shared form fields for the pure settings page.
func (a *App) buildHotkeySettingsPage(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	if snapshot.hotkey.Form == nil {
		return launcherview.HotkeySettingsView(launcherview.HotkeySettingsProps{Width: width, Height: height, Theme: snapshot.palette.componentTheme()})
	}
	innerWidth := max(float32(0), width-72)
	callbacks := formFieldCallbacks{
		idPrefix: "hotkey-settings", focus: a.focusHotkeySettingsField, openTable: a.openHotkeySettingsTable, recordKey: a.recordHotkeySettingsField,
	}
	rows := make([]woxwidget.Widget, 0, len(snapshot.hotkey.Form.definitions))
	for index, definition := range snapshot.hotkey.Form.definitions {
		rows = append(rows, woxwidget.Keyed{Key: formFieldRowKey("hotkey-settings", index), Child: a.buildFormField(*snapshot.hotkey.Form, callbacks, snapshot.palette, index, definition, innerWidth, 0)})
	}
	return launcherview.HotkeySettingsView(launcherview.HotkeySettingsProps{
		Width: width, Height: height, Theme: snapshot.palette.componentTheme(), Available: true,
		Rows: rows, KeepVisibleKey: formFieldsKeepVisibleKey("hotkey-settings", *snapshot.hotkey.Form),
	})
}

// newHotkeySettingsForm maps global bindings and query launchers onto the shared form/table engine.
func newHotkeySettingsForm(data settingsData) formFieldsState {
	definitions := []formDefinition{
		{Type: "hotkey", Value: formDefinitionValue{Key: "MainHotkey", Label: "i18n:ui_hotkey", Tooltip: "i18n:ui_hotkey_tips"}},
	}
	if !data.IsLinuxWaylandSession {
		definitions = append(definitions,
			formDefinition{Type: "hotkey", Value: formDefinitionValue{Key: "SelectionHotkey", Label: "i18n:ui_selection_hotkey", Tooltip: "i18n:ui_selection_hotkey_tips"}},
			formDefinition{Type: "table", Value: formDefinitionValue{
				Key: "IgnoredHotkeyApps", Title: "i18n:ui_hotkey_ignore_apps", Tooltip: "i18n:ui_hotkey_ignore_apps_tips", MaxHeight: 220, InlineTable: true,
				Columns: []formTableColumn{{Key: "App", Label: "i18n:ui_hotkey_ignore_apps_app", Tooltip: "i18n:ui_hotkey_ignore_apps_tips", Width: 420, Type: "app", Validators: []formValidator{{Type: "not_empty"}}}},
			}},
		)
	}
	definitions = append(definitions,
		formDefinition{Type: "table", Value: formDefinitionValue{
			Key: "QueryHotkeys", Title: "i18n:ui_query_hotkeys", Tooltip: "i18n:ui_query_hotkeys_tips", SortColumnKey: "Query", InlineTable: true, UpdateDialogWidth: 700,
			Columns: []formTableColumn{
				{Key: "Name", Label: "i18n:ui_query_hotkeys_name", Tooltip: "i18n:ui_query_hotkeys_name_tooltip", Width: 140, Type: "text"},
				{Key: "Hotkey", Label: "i18n:ui_query_hotkeys_hotkey", Tooltip: "i18n:ui_query_hotkeys_hotkey_tooltip", Width: 120, Type: "hotkey", Validators: []formValidator{{Type: "not_empty"}}},
				{Key: "Query", Label: "i18n:ui_query_hotkeys_query", Tooltip: "i18n:ui_query_hotkeys_query_tooltip", Type: "queryHotkeyQuery", Validators: []formValidator{{Type: "not_empty"}}},
				{Key: "Position", Label: "i18n:ui_query_hotkeys_position", Tooltip: "i18n:ui_query_hotkeys_position_tooltip", Width: 120, Type: "select", HideInTable: true, SelectOptions: queryHotkeyPositionOptions()},
				{Key: "HideQueryBox", Label: "i18n:ui_query_hotkeys_hide_query_box", Tooltip: "i18n:ui_query_hotkeys_hide_query_box_tooltip", Width: 80, Type: "checkbox", HideInTable: true},
				{Key: "HideToolbar", Label: "i18n:ui_query_hotkeys_hide_toolbar", Tooltip: "i18n:ui_query_hotkeys_hide_toolbar_tooltip", Width: 80, Type: "checkbox", HideInTable: true},
				{Key: "Width", Label: "i18n:ui_query_hotkeys_width", Tooltip: "i18n:ui_query_hotkeys_width_tooltip", Width: 50, Type: "text", HideInTable: true, EmptyAsZero: true, Validators: optionalIntegerValidators(false, 0, 0, "")},
				{Key: "MaxResultCount", Label: "i18n:ui_query_hotkeys_max_result_count", Tooltip: "i18n:ui_query_hotkeys_max_result_count_tooltip", Width: 90, Type: "text", HideInTable: true, EmptyAsZero: true, Validators: optionalIntegerValidators(true, 5, 15, "i18n:ui_query_hotkeys_max_result_count_range_error")},
				{Key: "IsSilentExecution", Label: "i18n:ui_query_hotkeys_silent", Tooltip: "i18n:ui_query_hotkeys_silent_tooltip", Width: 40, Type: "checkbox", HideInTable: true},
				{Key: "Disabled", Label: "i18n:ui_disabled", Tooltip: "i18n:ui_disabled_tooltip", Width: 60, Type: "checkbox"},
			},
		}},
		formDefinition{Type: "table", Value: formDefinitionValue{
			Key: "QueryShortcuts", Title: "i18n:ui_query_shortcuts", Tooltip: "i18n:ui_query_shortcuts_tips", SortColumnKey: "Query", InlineTable: true,
			Columns: []formTableColumn{
				{Key: "Shortcut", Label: "i18n:ui_query_shortcuts_shortcut", Tooltip: "i18n:ui_query_shortcuts_shortcut_tooltip", Width: 120, Type: "text", Validators: []formValidator{{Type: "not_empty"}}},
				{Key: "Query", Label: "i18n:ui_query_shortcuts_query", Tooltip: "i18n:ui_query_shortcuts_query_tooltip", Type: "text", Validators: []formValidator{{Type: "not_empty"}}},
				{Key: "Disabled", Label: "i18n:ui_disabled", Tooltip: "i18n:ui_disabled_tooltip", Width: 60, Type: "checkbox"},
			},
		}},
	)
	if !data.IsLinuxWaylandSession {
		definitions = append(definitions, formDefinition{Type: "table", Value: formDefinitionValue{
			Key: "TrayQueries", Title: "i18n:ui_tray_queries", Tooltip: "i18n:ui_tray_queries_tips", InlineTable: true,
			Columns: []formTableColumn{
				{Key: "Icon", Label: "i18n:ui_tray_queries_icon", Tooltip: "i18n:ui_tray_queries_icon_tooltip", Width: 40, Type: "woxImage"},
				{Key: "Query", Label: "i18n:ui_tray_queries_query", Tooltip: "i18n:ui_tray_queries_query_tooltip", Type: "text", Validators: []formValidator{{Type: "not_empty"}}},
				{Key: "HideQueryBox", Label: "i18n:ui_tray_queries_hide_query_box", Tooltip: "i18n:ui_tray_queries_hide_query_box_tooltip", Width: 80, Type: "checkbox", HideInTable: true},
				{Key: "HideToolbar", Label: "i18n:ui_tray_queries_hide_toolbar", Tooltip: "i18n:ui_tray_queries_hide_toolbar_tooltip", Width: 80, Type: "checkbox", HideInTable: true},
				{Key: "Width", Label: "i18n:ui_tray_queries_width", Tooltip: "i18n:ui_tray_queries_width_tooltip", Width: 40, Type: "text", HideInTable: true, EmptyAsZero: true, Validators: optionalIntegerValidators(false, 0, 0, "")},
				{Key: "MaxResultCount", Label: "i18n:ui_tray_queries_max_result_count", Tooltip: "i18n:ui_tray_queries_max_result_count_tooltip", Width: 90, Type: "text", HideInTable: true, EmptyAsZero: true, Validators: optionalIntegerValidators(true, 5, 15, "i18n:ui_query_hotkeys_max_result_count_range_error")},
				{Key: "Disabled", Label: "i18n:ui_disabled", Tooltip: "i18n:ui_disabled_tooltip", Width: 50, Type: "checkbox"},
			},
		}})
	}
	values := map[string]string{
		"MainHotkey":        data.MainHotkey,
		"SelectionHotkey":   data.SelectionHotkey,
		"IgnoredHotkeyApps": settingsIgnoredHotkeyAppRowsJSON(data.IgnoredHotkeyApps),
		"QueryHotkeys":      settingsRowsJSON(data.QueryHotkeys),
		"QueryShortcuts":    settingsRowsJSON(data.QueryShortcuts),
		"TrayQueries":       settingsJSONArray(data.TrayQueries),
	}
	return newFormFieldsState(definitions, values, true)
}

func settingsIgnoredHotkeyAppRowsJSON(raw json.RawMessage) string {
	var apps []ignoredHotkeyApp
	if len(raw) > 0 && json.Unmarshal(raw, &apps) != nil {
		return "[]"
	}
	rows := make([]map[string]any, 0, len(apps))
	for _, app := range apps {
		rows = append(rows, map[string]any{"App": app})
	}
	return settingsRowsJSON(rows)
}

func settingsIgnoredHotkeyAppsCoreJSON(value string) (string, error) {
	rows, err := decodeFormTableRows(value)
	if err != nil {
		return "", err
	}
	apps := make([]any, 0, len(rows))
	for _, row := range rows {
		app, exists := row["App"]
		if !exists {
			continue
		}
		apps = append(apps, app)
	}
	encoded, err := json.Marshal(apps)
	if err != nil {
		return "", fmt.Errorf("encode ignored hotkey apps: %w", err)
	}
	return string(encoded), nil
}

// loadHotkeyAppCandidates asks core for platform-specific identities and keeps the picker itself platform-neutral.
// Delegates to the hotkey controller which owns the candidate cache and load status.
func (a *App) loadHotkeyAppCandidates() {
	a.hotkeySettings.ReloadAppCandidates(context.Background(), a.services, a.sessionID)
}

func settingsRowsJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

func queryHotkeyPositionOptions() []formOption {
	return []formOption{
		{Label: "System default", Value: "system_default"},
		{Label: "Top left", Value: "top_left"},
		{Label: "Top center", Value: "top_center"},
		{Label: "Top right", Value: "top_right"},
		{Label: "Center", Value: "center"},
		{Label: "Bottom left", Value: "bottom_left"},
		{Label: "Bottom center", Value: "bottom_center"},
		{Label: "Bottom right", Value: "bottom_right"},
	}
}

// optionalIntegerValidators describes blank-or-integer fields, optionally with an inclusive range.
func optionalIntegerValidators(hasRange bool, min, max int, errorKey string) []formValidator {
	return []formValidator{{
		Type: "is_number",
		Value: formValidatorValue{
			IsInteger: true, Optional: true, HasRange: hasRange, Min: min, Max: max, ErrorKey: errorKey,
		},
	}}
}

// onHotkeySettingsKey moves between shared fields without stealing keys from an active recorder.
func (a *App) onHotkeySettingsKey(event woxui.KeyEvent) bool {
	active := a.settingsOpen && a.settingTab == "general" && a.hotkeySettings.Focused() && a.hotkeySettings.Form() != nil && a.settingsTableEditor == nil
	if !active {
		return false
	}
	switch event.Key {
	case woxui.KeyArrowUp:
		a.moveHotkeySettingsFocus(-1)
	case woxui.KeyArrowDown:
		a.moveHotkeySettingsFocus(1)
	case woxui.KeyEnter, woxui.KeySpace, woxui.KeyArrowRight:
		a.activateHotkeySettingsField()
	default:
		return false
	}
	return true
}

func (a *App) moveHotkeySettingsFocus(delta int) {
	fields := a.hotkeySettings.Form()
	if fields == nil || len(fields.definitions) == 0 {
		return
	}
	index := fields.focused
	for step := 0; step < len(fields.definitions); step++ {
		index = (index + delta + len(fields.definitions)) % len(fields.definitions)
		if formDefinitionFocusable(fields.definitions[index]) {
			setFormFieldsFocusLocked(fields, index)
			a.settingRow = index
			break
		}
	}
	a.stopHotkeyRecordingForDifferentField(fields, index)
	a.invalidateSettingsWindow()
}

func (a *App) focusHotkeySettingsField(index int) {
	a.stopHotkeyRecordingForDifferentField(a.hotkeySettings.Form(), index)
	if fields := a.hotkeySettings.Form(); fields != nil && index >= 0 && index < len(fields.definitions) && formDefinitionFocusable(fields.definitions[index]) {
		setFormFieldsFocusLocked(fields, index)
		a.settingRow = index
		a.hotkeySettings.SetFocused(true)
	}
	a.invalidateSettingsWindow()
}

func (a *App) activateHotkeySettingsField() {
	fields := a.hotkeySettings.Form()
	if fields == nil || fields.focused < 0 || fields.focused >= len(fields.definitions) {
		return
	}
	index := fields.focused
	typeName := fields.definitions[index].Type
	if typeName == "hotkey" {
		a.recordHotkeySettingsField(index)
	} else if typeName == "table" {
		a.openHotkeySettingsTable(index)
	}
}

func (a *App) recordHotkeySettingsField(index int) {
	fields := a.hotkeySettings.Form()
	if fields == nil || index < 0 || index >= len(fields.definitions) {
		return
	}
	key := fields.definitions[index].Value.Key
	a.startHotkeyRecording("hotkey-settings", fields, index, key, nil)
}

func (a *App) openHotkeySettingsTable(index int) {
	if form := a.hotkeySettings.Form(); a.settingsOpen && a.settingTab == "general" && form != nil {
		a.settingRow = index
		a.openFormTableLocked(form, index)
	}
	a.finishOpeningFormTable()
}

// trayQueryRowIndexFromParam extracts a tray query row index from an open-settings param
// like "tray_queries:2", matching the tray icon context menu's edit target.
func trayQueryRowIndexFromParam(param string) (int, bool) {
	param = strings.TrimSpace(param)
	if !strings.HasPrefix(param, "tray_queries:") {
		return 0, false
	}
	index, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(param, "tray_queries:")))
	if err != nil || index < 0 {
		return 0, false
	}
	return index, true
}

// openTrayQueryEditor opens the settings form table and starts editing one tray query row,
// mirroring the inline table's edit button flow.
func (a *App) openTrayQueryEditor(rowIndex int) {
	fields := a.hotkeySettings.Form()
	if !a.settingsOpen || a.settingTab != "general" || fields == nil || rowIndex < 0 {
		return
	}
	index := -1
	for candidate, definition := range fields.definitions {
		if definition.Value.Key == "TrayQueries" {
			index = candidate
			break
		}
	}
	if index < 0 {
		return
	}
	// Focus the field first so the settings page keeps the TrayQueries table visible
	// while the row editor opens, mirroring the inline table's edit button flow.
	a.focusHotkeySettingsField(index)
	a.openHotkeySettingsTable(index)
	state := a.activeFormTableEditor()
	if state == nil || state.invalid || rowIndex >= len(state.rows) {
		return
	}
	a.selectFormTableRow(rowIndex)
	a.beginEditFormTableRowDirect()
}

func (a *App) applyHotkeySettingsRawLocked(key, value string) {
	raw := json.RawMessage(append([]byte(nil), value...))
	switch key {
	case "QueryHotkeys":
		a.generalSettings.Update(func(d *settingsData) { _ = json.Unmarshal(raw, &d.QueryHotkeys) })
	case "IgnoredHotkeyApps":
		a.generalSettings.Update(func(d *settingsData) { d.IgnoredHotkeyApps = raw })
	case "QueryShortcuts":
		a.generalSettings.Update(func(d *settingsData) { _ = json.Unmarshal(raw, &d.QueryShortcuts) })
	case "TrayQueries":
		a.generalSettings.Update(func(d *settingsData) { d.TrayQueries = raw })
	}
}
