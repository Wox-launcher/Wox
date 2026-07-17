package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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
	if snapshot.hotkeyForm == nil {
		return launcherview.HotkeySettingsView(launcherview.HotkeySettingsProps{Width: width, Height: height, Theme: snapshot.palette.componentTheme()})
	}
	innerWidth := max(float32(0), width-72)
	callbacks := formFieldCallbacks{
		idPrefix: "hotkey-settings", focus: a.focusHotkeySettingsField, openTable: a.openHotkeySettingsTable, recordKey: a.recordHotkeySettingsField,
	}
	rows := make([]woxwidget.Widget, 0, len(snapshot.hotkeyForm.definitions))
	for index, definition := range snapshot.hotkeyForm.definitions {
		rows = append(rows, a.buildFormField(*snapshot.hotkeyForm, callbacks, snapshot.palette, index, definition, innerWidth, formDefinitionHeight(definition, snapshot.hotkeyForm.values)))
	}
	return launcherview.HotkeySettingsView(launcherview.HotkeySettingsProps{
		Width: width, Height: height, Theme: snapshot.palette.componentTheme(), Available: true,
		Rows: rows, RowsHeight: formDefinitionsContentHeight(snapshot.hotkeyForm.definitions, snapshot.hotkeyForm.values), Scroll: snapshot.hotkeyForm.scroll, Note: snapshot.note,
		OnScroll: a.scrollHotkeySettings, OnSetViewport: a.setHotkeySettingsViewport,
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
			Key: "QueryHotkeys", Title: "i18n:ui_query_hotkeys", Tooltip: "i18n:ui_query_hotkeys_tips", SortColumnKey: "Query", InlineTable: true,
			Columns: []formTableColumn{
				{Key: "Name", Label: "i18n:ui_query_hotkeys_name", Tooltip: "i18n:ui_query_hotkeys_name_tooltip", Width: 140, Type: "text"},
				{Key: "Hotkey", Label: "i18n:ui_query_hotkeys_hotkey", Tooltip: "i18n:ui_query_hotkeys_hotkey_tooltip", Width: 120, Type: "hotkey", Validators: []formValidator{{Type: "not_empty"}}},
				{Key: "Query", Label: "i18n:ui_query_hotkeys_query", Tooltip: "i18n:ui_query_hotkeys_query_tooltip", Type: "queryHotkeyQuery", Validators: []formValidator{{Type: "not_empty"}}},
				{Key: "Position", Label: "i18n:ui_query_hotkeys_position", Tooltip: "i18n:ui_query_hotkeys_position_tooltip", Width: 120, Type: "select", HideInTable: true, SelectOptions: queryHotkeyPositionOptions()},
				{Key: "HideQueryBox", Label: "i18n:ui_query_hotkeys_hide_query_box", Tooltip: "i18n:ui_query_hotkeys_hide_query_box_tooltip", Width: 80, Type: "checkbox", HideInTable: true},
				{Key: "HideToolbar", Label: "i18n:ui_query_hotkeys_hide_toolbar", Tooltip: "i18n:ui_query_hotkeys_hide_toolbar_tooltip", Width: 80, Type: "checkbox", HideInTable: true},
				{Key: "Width", Label: "i18n:ui_query_hotkeys_width", Tooltip: "i18n:ui_query_hotkeys_width_tooltip", Width: 50, Type: "text", HideInTable: true},
				{Key: "MaxResultCount", Label: "i18n:ui_query_hotkeys_max_result_count", Tooltip: "i18n:ui_query_hotkeys_max_result_count_tooltip", Width: 90, Type: "text", HideInTable: true},
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
				{Key: "Width", Label: "i18n:ui_tray_queries_width", Tooltip: "i18n:ui_tray_queries_width_tooltip", Width: 40, Type: "text", HideInTable: true},
				{Key: "MaxResultCount", Label: "i18n:ui_tray_queries_max_result_count", Tooltip: "i18n:ui_tray_queries_max_result_count_tooltip", Width: 90, Type: "text", HideInTable: true},
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
func (a *App) loadHotkeyAppCandidates() {
	a.mu.Lock()
	if a.hotkeyAppsLoaded || a.hotkeyAppsLoading {
		a.mu.Unlock()
		return
	}
	a.hotkeyAppsLoading = true
	a.hotkeyAppsError = ""
	a.mu.Unlock()
	a.invalidateSettingsWindow()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var apps []ignoredHotkeyApp
	err := a.client.Post(ctx, "/setting/hotkey/apps", map[string]any{}, &apps)
	cancel()

	a.mu.Lock()
	a.hotkeyAppsLoading = false
	if err != nil {
		a.hotkeyAppsError = err.Error()
	} else {
		seen := make(map[string]bool, len(apps))
		filtered := make([]ignoredHotkeyApp, 0, len(apps))
		for _, app := range apps {
			identity := strings.ToLower(strings.TrimSpace(app.Identity))
			if identity == "" || seen[identity] {
				continue
			}
			seen[identity] = true
			filtered = append(filtered, app)
		}
		a.hotkeyAppCandidates = filtered
		a.hotkeyAppsLoaded = true
		a.hotkeyAppsError = ""
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
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

// onHotkeySettingsKey moves between shared fields without stealing keys from an active recorder.
func (a *App) onHotkeySettingsKey(event woxui.KeyEvent) bool {
	a.mu.RLock()
	active := a.settingsOpen && a.settingTab == "general" && a.settingsHotkeyFocus && a.hotkeySettingsForm != nil && a.tableEditor == nil
	a.mu.RUnlock()
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
	a.mu.Lock()
	fields := a.hotkeySettingsForm
	if fields == nil || len(fields.definitions) == 0 {
		a.mu.Unlock()
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
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

func (a *App) focusHotkeySettingsField(index int) {
	a.stopHotkeyRecordingForDifferentField(a.hotkeySettingsForm, index)
	a.mu.Lock()
	if fields := a.hotkeySettingsForm; fields != nil && index >= 0 && index < len(fields.definitions) && formDefinitionFocusable(fields.definitions[index]) {
		setFormFieldsFocusLocked(fields, index)
		a.settingRow = index
		a.settingsHotkeyFocus = true
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

func (a *App) activateHotkeySettingsField() {
	a.mu.RLock()
	fields := a.hotkeySettingsForm
	if fields == nil || fields.focused < 0 || fields.focused >= len(fields.definitions) {
		a.mu.RUnlock()
		return
	}
	index := fields.focused
	typeName := fields.definitions[index].Type
	a.mu.RUnlock()
	if typeName == "hotkey" {
		a.recordHotkeySettingsField(index)
	} else if typeName == "table" {
		a.openHotkeySettingsTable(index)
	}
}

func (a *App) recordHotkeySettingsField(index int) {
	a.mu.RLock()
	fields := a.hotkeySettingsForm
	if fields == nil || index < 0 || index >= len(fields.definitions) {
		a.mu.RUnlock()
		return
	}
	key := fields.definitions[index].Value.Key
	a.mu.RUnlock()
	a.startHotkeyRecording("hotkey-settings", fields, index, key, nil)
}

func (a *App) openHotkeySettingsTable(index int) {
	a.mu.Lock()
	if a.settingsOpen && a.settingTab == "general" && a.hotkeySettingsForm != nil {
		a.settingRow = index
		a.openFormTableLocked(a.hotkeySettingsForm, index)
	}
	a.mu.Unlock()
	a.finishOpeningFormTable()
}

func (a *App) setHotkeySettingsViewport(height float32) {
	a.mu.Lock()
	if a.hotkeySettingsForm != nil {
		a.hotkeySettingsForm.viewportHeight = max(float32(1), height)
		ensureFormFieldsFocusVisibleLocked(a.hotkeySettingsForm, a.hotkeySettingsForm.focused)
	}
	a.mu.Unlock()
}

func (a *App) scrollHotkeySettings(delta float32) {
	a.mu.Lock()
	if fields := a.hotkeySettingsForm; fields != nil {
		maxOffset := max(float32(0), formDefinitionsContentHeight(fields.definitions, fields.values)-fields.viewportHeight)
		fields.scroll = min(max(float32(0), fields.scroll+delta), maxOffset)
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

func hotkeySettingsLabel(key string) string {
	switch key {
	case "QueryHotkeys":
		return "Query hotkeys"
	case "IgnoredHotkeyApps":
		return "Ignored hotkey apps"
	case "QueryShortcuts":
		return "Query shortcuts"
	case "TrayQueries":
		return "Tray queries"
	default:
		return key
	}
}

func (a *App) applyHotkeySettingsRawLocked(key, value string) {
	raw := json.RawMessage(append([]byte(nil), value...))
	switch key {
	case "QueryHotkeys":
		_ = json.Unmarshal(raw, &a.settings.QueryHotkeys)
	case "IgnoredHotkeyApps":
		a.settings.IgnoredHotkeyApps = raw
	case "QueryShortcuts":
		_ = json.Unmarshal(raw, &a.settings.QueryShortcuts)
	case "TrayQueries":
		a.settings.TrayQueries = raw
	}
}
