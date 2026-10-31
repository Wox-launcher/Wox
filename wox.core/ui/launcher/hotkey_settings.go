package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"wox/common/icons"
	"wox/setting"
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
func (c *hotkeySettingsController) buildHotkeySettingsPage(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	if snapshot.hotkey.Form == nil {
		return launcherview.HotkeySettingsView(launcherview.HotkeySettingsProps{Width: width, Height: height, Theme: snapshot.palette})
	}
	innerWidth := max(float32(0), width-72)
	callbacks := formFieldCallbacks{
		idPrefix: "hotkey-settings", focus: c.focusHotkeySettingsField, openTable: c.openHotkeySettingsTable, recordKey: c.recordHotkeySettingsField,
	}
	rows := make([]woxwidget.Widget, 0, len(snapshot.hotkey.Form.definitions))
	for index, definition := range snapshot.hotkey.Form.definitions {
		rows = append(rows, woxwidget.Keyed{Key: formFieldRowKey("hotkey-settings", index), Child: c.buildFormField(*snapshot.hotkey.Form, callbacks, snapshot.palette, index, definition, innerWidth, 0)})
	}
	return launcherview.HotkeySettingsView(launcherview.HotkeySettingsProps{
		Width: width, Height: height, Theme: snapshot.palette, Available: true,
		Rows: rows, KeepVisibleKey: formFieldsKeepVisibleKey("hotkey-settings", *snapshot.hotkey.Form),
	})
}

// newHotkeySettingsForm maps global bindings and query hotkeys onto the shared form/table engine.
func newHotkeySettingsForm(data settingsData) formFieldsState {
	definitions := []formDefinition{
		{Type: "hotkey", Value: formDefinitionValue{Key: "MainHotkey", Label: "i18n:ui_hotkey", Tooltip: "i18n:ui_hotkey_tips"}},
	}
	if !data.IsLinuxWaylandSession || data.SelectionQuerySupported {
		tip := "i18n:ui_selection_hotkey_tips"
		if data.IsLinuxWaylandSession {
			tip = "i18n:ui_selection_hotkey_wayland_tips"
		}
		definitions = append(definitions,
			formDefinition{Type: "hotkey", Value: formDefinitionValue{Key: "SelectionHotkey", Label: "i18n:ui_selection_hotkey", Tooltip: tip}},
		)
	}
	if !data.IsLinuxWaylandSession {
		definitions = append(definitions, formDefinition{Type: "table", Value: formDefinitionValue{
			Key: "IgnoredHotkeyApps", Title: "i18n:ui_hotkey_ignore_apps", Tooltip: "i18n:ui_hotkey_ignore_apps_tips", MaxHeight: 220, InlineTable: true,
			Columns: []formTableColumn{{Key: "App", Label: "i18n:ui_hotkey_ignore_apps_app", Tooltip: "i18n:ui_hotkey_ignore_apps_tips", Width: 420, Type: "app", Validators: []formValidator{{Type: "not_empty"}}}},
		}})
	}
	definitions = append(definitions, formDefinition{Type: "table", Value: formDefinitionValue{
		Key: "ResultBindings", Title: "i18n:ui_result_bindings", Tooltip: "i18n:ui_result_bindings_tips", SortColumnKey: "Title", InlineTable: true,
		Columns: []formTableColumn{
			{Key: "Title", Label: "i18n:ui_result_bindings_title", Tooltip: "i18n:ui_result_bindings_title_tooltip", Width: 220, Type: "text", HideInUpdate: true},
			{Key: "Hotkey", Label: "i18n:ui_result_bindings_hotkey", Tooltip: "i18n:ui_result_bindings_hotkey_tooltip", Width: 140, Type: "hotkey"},
			{Key: "Alias", Label: "i18n:ui_result_bindings_alias", Tooltip: "i18n:ui_result_bindings_alias_tooltip", Width: 140, Type: "text"},
		},
	}})
	definitions = append(definitions, formDefinition{Type: "table", Value: formDefinitionValue{
		Key: "QueryHotkeys", Title: "i18n:ui_query_hotkeys", Tooltip: "i18n:ui_query_hotkeys_tips", SortColumnKey: "Query", InlineTable: true, UpdateDialogWidth: 700,
		Columns: []formTableColumn{
			{Key: "Name", Label: "i18n:ui_query_hotkeys_name", Tooltip: "i18n:ui_query_hotkeys_name_tooltip", Width: 140, Type: "text"},
			{Key: "Hotkey", Label: "i18n:ui_query_hotkeys_hotkey", Tooltip: "i18n:ui_query_hotkeys_hotkey_tooltip", Width: 120, Type: "hotkey", Validators: []formValidator{{Type: "not_empty"}}},
			{Key: "Query", Label: "i18n:ui_query_hotkeys_query", Tooltip: "i18n:ui_query_hotkeys_query_tooltip", Type: "queryHotkeyQuery", QueryTest: true, Validators: []formValidator{{Type: "not_empty"}}},
			{Key: "Position", Label: "i18n:ui_query_hotkeys_position", Tooltip: "i18n:ui_query_hotkeys_position_tooltip", Width: 120, Type: "select", HideInTable: true, SelectOptions: queryHotkeyPositionOptions()},
			{Key: "HideQueryBox", Label: "i18n:ui_query_hotkeys_hide_query_box", Tooltip: "i18n:ui_query_hotkeys_hide_query_box_tooltip", Width: 80, Type: "checkbox", HideInTable: true},
			{Key: "HideToolbar", Label: "i18n:ui_query_hotkeys_hide_toolbar", Tooltip: "i18n:ui_query_hotkeys_hide_toolbar_tooltip", Width: 80, Type: "checkbox", HideInTable: true},
			{Key: "HideResults", Label: "i18n:ui_query_hotkeys_hide_results", Tooltip: "i18n:ui_query_hotkeys_hide_results_tooltip", Width: 80, Type: "checkbox", HideInTable: true},
			{Key: "Width", Label: "i18n:ui_query_hotkeys_width", Tooltip: "i18n:ui_query_hotkeys_width_tooltip", Width: 50, Type: "text", HideInTable: true, EmptyAsZero: true, Validators: optionalIntegerValidators(false, 0, 0, "")},
			{Key: "MaxResultCount", Label: "i18n:ui_query_hotkeys_max_result_count", Tooltip: "i18n:ui_query_hotkeys_max_result_count_tooltip", Width: 90, Type: "text", HideInTable: true, EmptyAsZero: true, Validators: optionalIntegerValidators(true, 5, 15, "i18n:ui_query_hotkeys_max_result_count_range_error")},
			{Key: "IsSilentExecution", Label: "i18n:ui_query_hotkeys_silent", Tooltip: "i18n:ui_query_hotkeys_silent_tooltip", Width: 40, Type: "checkbox", HideInTable: true},
			{Key: "Disabled", Label: "i18n:ui_disabled", Tooltip: "i18n:ui_disabled_tooltip", Width: 60, Type: "checkbox"},
		},
	}})
	definitions = append(definitions,
		formDefinition{Type: "hotkey", Value: formDefinitionValue{Key: "ActionPanelHotkey", Label: "i18n:ui_action_panel_hotkey", Tooltip: "i18n:ui_action_panel_hotkey_tips"}},
		formDefinition{Type: "hotkey", Value: formDefinitionValue{Key: "RefinementHotkey", Label: "i18n:ui_refinement_hotkey", Tooltip: "i18n:ui_refinement_hotkey_tips"}},
	)
	values := map[string]string{
		"RefinementHotkey":  data.RefinementHotkey,
		"MainHotkey":        data.MainHotkey,
		"SelectionHotkey":   data.SelectionHotkey,
		"ActionPanelHotkey": data.ActionPanelHotkey,
		"IgnoredHotkeyApps": settingsIgnoredHotkeyAppRowsJSON(data.IgnoredHotkeyApps),
		"ResultBindings":    settingsRowsJSON(data.ResultBindings),
		"QueryHotkeys":      settingsRowsJSON(data.QueryHotkeys),
	}
	return newFormFieldsState(definitions, values, true)
}

// newGeneralQuerySettingsForm maps query aliases and tray launchers onto General.
func newGeneralQuerySettingsForm(data settingsData) formFieldsState {
	definitions := []formDefinition{{Type: "table", Value: formDefinitionValue{
		Key: "QueryAliases", Title: "i18n:ui_query_shortcuts", Tooltip: "i18n:ui_query_shortcuts_tips", SortColumnKey: "Query", InlineTable: true,
		Columns: []formTableColumn{
			{Key: "Shortcut", Label: "i18n:ui_query_shortcuts_shortcut", Tooltip: "i18n:ui_query_shortcuts_shortcut_tooltip", Width: 120, Type: "text", Validators: []formValidator{{Type: "not_empty"}}},
			{Key: "Query", Label: "i18n:ui_query_shortcuts_query", Tooltip: "i18n:ui_query_shortcuts_query_tooltip", Type: "text", QueryTest: true, Validators: []formValidator{{Type: "not_empty"}}},
			{Key: "Disabled", Label: "i18n:ui_disabled", Tooltip: "i18n:ui_disabled_tooltip", Width: 60, Type: "checkbox"},
		},
	}}}
	if !data.IsLinuxWaylandSession || data.TrayQuerySupported {
		definitions = append(definitions, formDefinition{Type: "table", Value: formDefinitionValue{
			Key: "TrayQueries", Title: "i18n:ui_tray_queries", Tooltip: "i18n:ui_tray_queries_tips", InlineTable: true,
			Columns: []formTableColumn{
				{Key: "Icon", Label: "i18n:ui_tray_queries_icon", Tooltip: "i18n:ui_tray_queries_icon_tooltip", Width: 40, Type: "woxImage"},
				{Key: "Query", Label: "i18n:ui_tray_queries_query", Tooltip: "i18n:ui_tray_queries_query_tooltip", Type: "text", QueryTest: true, Validators: []formValidator{{Type: "not_empty"}}},
				{Key: "HideQueryBox", Label: "i18n:ui_tray_queries_hide_query_box", Tooltip: "i18n:ui_tray_queries_hide_query_box_tooltip", Width: 80, Type: "checkbox", HideInTable: true},
				{Key: "HideToolbar", Label: "i18n:ui_tray_queries_hide_toolbar", Tooltip: "i18n:ui_tray_queries_hide_toolbar_tooltip", Width: 80, Type: "checkbox", HideInTable: true},
				{Key: "Width", Label: "i18n:ui_tray_queries_width", Tooltip: "i18n:ui_tray_queries_width_tooltip", Width: 40, Type: "text", HideInTable: true, EmptyAsZero: true, Validators: optionalIntegerValidators(false, 0, 0, "")},
				{Key: "MaxResultCount", Label: "i18n:ui_tray_queries_max_result_count", Tooltip: "i18n:ui_tray_queries_max_result_count_tooltip", Width: 90, Type: "text", HideInTable: true, EmptyAsZero: true, Validators: optionalIntegerValidators(true, 5, 15, "i18n:ui_query_hotkeys_max_result_count_range_error")},
				{Key: "Disabled", Label: "i18n:ui_disabled", Tooltip: "i18n:ui_disabled_tooltip", Width: 50, Type: "checkbox"},
			},
		}})
	}
	values := map[string]string{
		"QueryAliases": settingsRowsJSON(data.QueryAliases),
		"TrayQueries":  settingsJSONArray(data.TrayQueries),
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
func (c *hotkeySettingsController) loadHotkeyAppCandidates() {
	c.ReloadAppCandidates(context.Background(), c.services, c.sessionID)
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
		{Label: "i18n:ui_query_position_system_default", Value: string(setting.QueryHotkeyPositionSystemDefault), Icon: fromCoreImage(icons.Get("position.system-default"))},
		{Label: "i18n:ui_query_position_top_left", Value: string(setting.QueryHotkeyPositionTopLeft), Icon: fromCoreImage(icons.Get("position.top-left"))},
		{Label: "i18n:ui_query_position_top_center", Value: string(setting.QueryHotkeyPositionTopCenter), Icon: fromCoreImage(icons.Get("position.top-center"))},
		{Label: "i18n:ui_query_position_top_right", Value: string(setting.QueryHotkeyPositionTopRight), Icon: fromCoreImage(icons.Get("position.top-right"))},
		{Label: "i18n:ui_query_position_middle_left", Value: string(setting.QueryHotkeyPositionMiddleLeft), Icon: fromCoreImage(icons.Get("position.middle-left"))},
		{Label: "i18n:ui_query_position_center", Value: string(setting.QueryHotkeyPositionCenter), Icon: fromCoreImage(icons.Get("position.center"))},
		{Label: "i18n:ui_query_position_middle_right", Value: string(setting.QueryHotkeyPositionMiddleRight), Icon: fromCoreImage(icons.Get("position.middle-right"))},
		{Label: "i18n:ui_query_position_bottom_left", Value: string(setting.QueryHotkeyPositionBottomLeft), Icon: fromCoreImage(icons.Get("position.bottom-left"))},
		{Label: "i18n:ui_query_position_bottom_center", Value: string(setting.QueryHotkeyPositionBottomCenter), Icon: fromCoreImage(icons.Get("position.bottom-center"))},
		{Label: "i18n:ui_query_position_bottom_right", Value: string(setting.QueryHotkeyPositionBottomRight), Icon: fromCoreImage(icons.Get("position.bottom-right"))},
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
func (c *hotkeySettingsController) onHotkeySettingsKey(event woxui.KeyEvent) bool {
	active := c.settings.open && c.settings.tab == "hotkey" && c.Focused() && c.Form() != nil && c.formTables.settingsEditor == nil
	if !active {
		return false
	}
	switch event.Key {
	case woxui.KeyArrowUp:
		c.moveHotkeySettingsFocus(-1)
	case woxui.KeyArrowDown:
		c.moveHotkeySettingsFocus(1)
	case woxui.KeyEnter, woxui.KeySpace, woxui.KeyArrowRight:
		c.activateHotkeySettingsField()
	default:
		return false
	}
	return true
}

func (c *hotkeySettingsController) moveHotkeySettingsFocus(delta int) {
	fields := c.Form()
	if fields == nil || len(fields.definitions) == 0 {
		return
	}
	index := fields.focused
	for step := 0; step < len(fields.definitions); step++ {
		index = (index + delta + len(fields.definitions)) % len(fields.definitions)
		if formDefinitionFocusable(fields.definitions[index]) {
			setFormFieldsFocusLocked(fields, index)
			c.settings.row = index
			break
		}
	}
	c.hotkeyRecording.stopHotkeyRecordingForDifferentField(fields, index)
	c.settings.invalidateSettingsWindow()
}

func (c *hotkeySettingsController) focusHotkeySettingsField(index int) {
	c.hotkeyRecording.stopHotkeyRecordingForDifferentField(c.Form(), index)
	if fields := c.Form(); fields != nil && index >= 0 && index < len(fields.definitions) && formDefinitionFocusable(fields.definitions[index]) {
		setFormFieldsFocusLocked(fields, index)
		c.settings.row = index
		c.SetFocused(true)
	}
	c.settings.invalidateSettingsWindow()
}

func (c *hotkeySettingsController) activateHotkeySettingsField() {
	fields := c.Form()
	if fields == nil || fields.focused < 0 || fields.focused >= len(fields.definitions) {
		return
	}
	index := fields.focused
	typeName := fields.definitions[index].Type
	if typeName == "hotkey" {
		c.recordHotkeySettingsField(index)
	} else if typeName == "table" {
		c.openHotkeySettingsTable(index)
	}
}

func (c *hotkeySettingsController) recordHotkeySettingsField(index int) {
	fields := c.Form()
	if fields == nil || index < 0 || index >= len(fields.definitions) {
		return
	}
	key := fields.definitions[index].Value.Key
	c.hotkeyRecording.startHotkeyRecording("hotkey-settings", fields, index, key, nil, nil)
}

func (c *hotkeySettingsController) openHotkeySettingsTable(index int) {
	if form := c.Form(); c.settings.open && c.settings.tab == "hotkey" && form != nil {
		c.settings.row = index
		c.formTables.openFormTableLocked(form, index)
	}
	c.formTables.finishOpeningFormTable()
}

// onGeneralQuerySettingsKey routes navigation to the focused query table instead of a built-in row.
func (c *hotkeySettingsController) onGeneralQuerySettingsKey(event woxui.KeyEvent) bool {
	fields := c.generalSettings().Form()
	if !c.settings.open || c.settings.tab != "general" || !c.generalSettings().FormFocused() || fields == nil || len(fields.definitions) == 0 || c.formTables.settingsEditor != nil {
		return false
	}
	switch event.Key {
	case woxui.KeyArrowUp:
		c.focusGeneralQuerySettingsField((fields.focused - 1 + len(fields.definitions)) % len(fields.definitions))
	case woxui.KeyArrowDown:
		c.focusGeneralQuerySettingsField((fields.focused + 1) % len(fields.definitions))
	case woxui.KeyEnter, woxui.KeySpace, woxui.KeyArrowRight:
		c.openGeneralQuerySettingsTable(fields.focused)
	default:
		return false
	}
	return true
}

// focusGeneralQuerySettingsField keeps one General query table visible for search and tray edit.
func (c *hotkeySettingsController) focusGeneralQuerySettingsField(index int) {
	if fields := c.generalSettings().Form(); fields != nil && index >= 0 && index < len(fields.definitions) && formDefinitionFocusable(fields.definitions[index]) {
		setFormFieldsFocusLocked(fields, index)
		c.settings.row = index
		c.generalSettings().SetFormFocused(true)
	}
	c.settings.invalidateSettingsWindow()
}

// openGeneralQuerySettingsTable opens a General query table when that page is current.
func (c *hotkeySettingsController) openGeneralQuerySettingsTable(index int) {
	if form := c.generalSettings().Form(); c.settings.open && c.settings.tab == "general" && form != nil {
		c.settings.row = index
		c.formTables.openFormTableLocked(form, index)
	}
	c.formTables.finishOpeningFormTable()
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
func (c *hotkeySettingsController) openTrayQueryEditor(rowIndex int) {
	fields := c.generalSettings().Form()
	if !c.settings.open || c.settings.tab != "general" || fields == nil || rowIndex < 0 {
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
	c.focusGeneralQuerySettingsField(index)
	c.openGeneralQuerySettingsTable(index)
	state := c.formTables.activeFormTableEditor()
	if state == nil || state.invalid || rowIndex >= len(state.rows) {
		return
	}
	c.formTables.selectFormTableRow(rowIndex)
	c.formTables.beginEditFormTableRowDirect()
}

func (c *hotkeySettingsController) applyHotkeySettingsRawLocked(key, value string) {
	raw := json.RawMessage(append([]byte(nil), value...))
	switch key {
	case "QueryHotkeys":
		c.generalSettings().Update(func(d *settingsData) { _ = json.Unmarshal(raw, &d.QueryHotkeys) })
	case "ResultBindings":
		c.generalSettings().Update(func(d *settingsData) { _ = json.Unmarshal(raw, &d.ResultBindings) })
	case "IgnoredHotkeyApps":
		c.generalSettings().Update(func(d *settingsData) { d.IgnoredHotkeyApps = raw })
	case "QueryAliases":
		c.generalSettings().Update(func(d *settingsData) { _ = json.Unmarshal(raw, &d.QueryAliases) })
	case "TrayQueries":
		c.generalSettings().Update(func(d *settingsData) { d.TrayQueries = raw })
	}
}
