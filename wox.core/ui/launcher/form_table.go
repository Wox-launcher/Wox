package launcher

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"wox/common"
	"wox/plugin"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

const formTableRowIDKey = "wox_table_row_id"

type formTableEditorState struct {
	target     *formFieldsState
	fieldIndex int
	definition formDefinition
	rows       []map[string]any
	selected   int
	rowForm    *formFieldsState
	rowIndex   int
	rowBase    map[string]any
	// rowEditorOnly closes the whole overlay when a row opened directly from an inline table exits.
	rowEditorOnly     bool
	status            string
	fieldErrors       map[string]string
	invalid           bool
	saving            bool
	deletePending     int
	deleteDirect      bool
	appPicker         *formTableAppPickerState
	choicePicker      *formTableChoicePickerState
	queryVariable     *formTableQueryVariablePickerState
	emojiPicker       *formTableEmojiPickerState
	skillAdd          *formTableSkillAddState
	queryPreset       queryHotkeyPreset
	windowGroupEditor *windowGroupEditorState
}

type formTableEditorSnapshot struct {
	definition        formDefinition
	rows              []map[string]any
	selected          int
	rowForm           *formFieldsSnapshot
	rowIndex          int
	status            string
	fieldErrors       map[string]string
	invalid           bool
	saving            bool
	deletePending     int
	deleteDirect      bool
	appPicker         *formTableAppPickerSnapshot
	choicePicker      *formTableChoicePickerSnapshot
	queryVariable     *formTableQueryVariablePickerSnapshot
	emojiPicker       *formTableEmojiPickerSnapshot
	skillAdd          *formTableSkillAddSnapshot
	queryPreset       queryHotkeyPreset
	windowGroupEditor *windowGroupEditorSnapshot
}

type queryHotkeyPreset string

const (
	queryHotkeyPresetNormal   queryHotkeyPreset = "normal"
	queryHotkeyPresetWebPanel queryHotkeyPreset = "web-panel"
	queryHotkeyPresetSilent   queryHotkeyPreset = "silent"
	queryHotkeyPresetCustom   queryHotkeyPreset = "custom"
)

type formTableAppPickerState struct {
	fieldIndex int
	current    ignoredHotkeyApp
}

type formTableAppPickerSnapshot struct {
	fieldIndex int
	current    ignoredHotkeyApp
}

type formTableChoicePickerState struct {
	fieldIndex int
	anchor     woxui.Rect
}

type formTableChoicePickerSnapshot struct {
	fieldIndex   int
	anchor       woxui.Rect
	title        string
	currentValue string
	options      []formOption
}

type formTableQueryVariablePickerState struct {
	fieldIndex   int
	anchor       woxui.Rect
	triggerStart int
	selected     int
}

type formTableEmojiPickerState struct {
	fieldIndex   int
	initialEmoji string
}

type formTableEmojiPickerSnapshot struct {
	fieldIndex   int
	initialEmoji string
}

type formTableQueryVariablePickerSnapshot struct {
	fieldIndex int
	anchor     woxui.Rect
	query      string
	selected   int
}

type queryHotkeyVariable struct {
	value, label, description, icon string
}

var queryHotkeyVariables = []queryHotkeyVariable{
	{"{wox:selected_text}", "i18n:ui_query_variable_selected_text", "i18n:ui_query_variable_selected_text_tooltip", "copy"},
	{"{wox:selected_file}", "i18n:ui_query_variable_selected_file", "i18n:ui_query_variable_selected_file_tooltip", "document"},
	{"{wox:active_browser_url}", "i18n:ui_query_variable_active_browser_url", "i18n:ui_query_variable_active_browser_url_tooltip", "external"},
	{"{wox:file_explorer_path}", "i18n:ui_query_variable_file_explorer_path", "i18n:ui_query_variable_file_explorer_path_tooltip", "folder-open"},
}

const (
	queryHotkeyTestSelectedText = "test text"
	queryHotkeyTestSelectedFile = "/path/to/test.txt"
	queryHotkeyTestBrowserURL   = "https://example.com"
	queryHotkeyTestExplorerPath = "/path/to/folder"
)

// replaceQueryHotkeyVariablesForTest swaps runtime placeholders for stable sample values so the query can be previewed.
func replaceQueryHotkeyVariablesForTest(query string) string {
	replaced := query
	replaced = strings.ReplaceAll(replaced, plugin.QueryVariableSelectedText, queryHotkeyTestSelectedText)
	replaced = strings.ReplaceAll(replaced, plugin.QueryVariableSelectedFile, queryHotkeyTestSelectedFile)
	replaced = strings.ReplaceAll(replaced, plugin.QueryVariableActiveBrowserUrl, queryHotkeyTestBrowserURL)
	replaced = strings.ReplaceAll(replaced, plugin.QueryVariableFileExplorerPath, queryHotkeyTestExplorerPath)
	return replaced
}

// runFormTableQueryHotkeyTest opens the launcher with the editor query, substituting variables with sample values.
func (a *App) runFormTableQueryHotkeyTest() {
	state := a.activeFormTableEditor()
	if state == nil || state.rowForm == nil || state.definition.Value.Key != "QueryHotkeys" {
		return
	}
	if state.rowForm.editor != nil {
		syncFormFieldsEditorLocked(state.rowForm)
	}
	// Keep leading/trailing spaces from the editor; only reject blank-only queries.
	queryText := state.rowForm.values["Query"]
	if strings.TrimSpace(queryText) == "" {
		return
	}
	resolved := replaceQueryHotkeyVariablesForTest(queryText)
	a.setSettingChoiceTooltip(false, "", woxui.Rect{})
	a.closeFormTableQueryVariablePicker()

	params := a.show
	params.SelectAll = false
	params.HideQueryBox = false
	params.HideToolbar = false
	params.ShowSource = string(common.ShowSourceDefault)
	if params.Position.Type == "" && a.window != nil {
		if bounds, err := a.window.Bounds(); err == nil {
			params.Position = position{Type: "last_location", X: int(bounds.X), Y: int(bounds.Y)}
		}
	}
	a.setQuery(newInputQuery(resolved))
	util.Go(a.lifecycleCtx, "show launcher for query hotkey test", func() {
		if err := a.showWindow(params); err != nil {
			util.GetLogger().Error(a.lifecycleCtx, "show launcher for query hotkey test: "+err.Error())
			return
		}
		if err := a.sendCurrentQuery(); err != nil {
			util.GetLogger().Error(a.lifecycleCtx, "send query hotkey test: "+err.Error())
		}
	})
}

// decodeFormTableRows preserves JSON numbers and unknown row fields so the shared editor can round-trip future column types safely.
func decodeFormTableRows(value string) ([]map[string]any, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "null" {
		return []map[string]any{}, nil
	}
	decoder := json.NewDecoder(bytes.NewBufferString(value))
	decoder.UseNumber()
	var decoded []map[string]any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("unexpected trailing JSON value")
		}
		return nil, err
	}
	if decoded == nil {
		decoded = []map[string]any{}
	}
	return decoded, nil
}

func cloneFormTableRow(row map[string]any) map[string]any {
	copy := make(map[string]any, len(row))
	for key, value := range row {
		copy[key] = value
	}
	return copy
}

func cloneFormTableRows(rows []map[string]any) []map[string]any {
	copy := make([]map[string]any, len(rows))
	for index, row := range rows {
		copy[index] = cloneFormTableRow(row)
	}
	return copy
}

func snapshotFormTableEditorLocked(state *formTableEditorState) *formTableEditorSnapshot {
	if state == nil {
		return nil
	}
	var rowForm *formFieldsSnapshot
	if state.rowForm != nil {
		snapshot := snapshotFormFieldsLocked(state.rowForm)
		rowForm = &snapshot
	}
	var appPicker *formTableAppPickerSnapshot
	if state.appPicker != nil {
		appPicker = &formTableAppPickerSnapshot{
			fieldIndex: state.appPicker.fieldIndex,
			current:    state.appPicker.current,
		}
	}
	var choicePicker *formTableChoicePickerSnapshot
	if state.choicePicker != nil && state.rowForm != nil && state.choicePicker.fieldIndex >= 0 && state.choicePicker.fieldIndex < len(state.rowForm.definitions) {
		definition := state.rowForm.definitions[state.choicePicker.fieldIndex]
		choicePicker = &formTableChoicePickerSnapshot{
			fieldIndex: state.choicePicker.fieldIndex, anchor: state.choicePicker.anchor, title: definition.Value.Label,
			currentValue: state.rowForm.values[definition.Value.Key], options: append([]formOption(nil), definition.Value.Options...),
		}
	}
	var queryVariable *formTableQueryVariablePickerSnapshot
	if picker := state.queryVariable; picker != nil && state.rowForm != nil && picker.fieldIndex >= 0 && picker.fieldIndex < len(state.rowForm.definitions) {
		query := ""
		if picker.triggerStart >= 0 {
			value := []rune(state.rowForm.values[state.rowForm.definitions[picker.fieldIndex].Value.Key])
			caret := len(value)
			if state.rowForm.editor != nil {
				caret = state.rowForm.editor.State().Selection.Focus
			}
			if picker.triggerStart < caret && caret <= len(value) {
				query = string(value[picker.triggerStart+1 : caret])
			}
		}
		queryVariable = &formTableQueryVariablePickerSnapshot{fieldIndex: picker.fieldIndex, anchor: picker.anchor, query: query, selected: picker.selected}
	}
	var emojiPicker *formTableEmojiPickerSnapshot
	if picker := state.emojiPicker; picker != nil {
		emojiPicker = &formTableEmojiPickerSnapshot{fieldIndex: picker.fieldIndex, initialEmoji: picker.initialEmoji}
	}
	var skillAdd *formTableSkillAddSnapshot
	if state.skillAdd != nil {
		fields := snapshotFormFieldsLocked(state.skillAdd.fields)
		skillAdd = &formTableSkillAddSnapshot{tab: state.skillAdd.tab, fields: &fields, error: state.skillAdd.error, cloning: state.skillAdd.cloning}
	}
	return &formTableEditorSnapshot{
		definition:        state.definition,
		rows:              cloneFormTableRows(state.rows),
		selected:          state.selected,
		rowForm:           rowForm,
		rowIndex:          state.rowIndex,
		status:            state.status,
		fieldErrors:       cloneFormTableFieldErrors(state.fieldErrors),
		invalid:           state.invalid,
		saving:            state.saving,
		deletePending:     state.deletePending,
		deleteDirect:      state.deleteDirect,
		appPicker:         appPicker,
		choicePicker:      choicePicker,
		queryVariable:     queryVariable,
		emojiPicker:       emojiPicker,
		skillAdd:          skillAdd,
		queryPreset:       state.queryPreset,
		windowGroupEditor: snapshotWindowGroupEditorLocked(state.windowGroupEditor),
	}
}

func (a *App) formTableTargetCurrentLocked(target *formFieldsState) bool {
	pluginForm := a.pluginSettings.Form()
	return a.formTableTargetCurrentWithFormsLocked(target, pluginForm, a.aiSettings.Form(), a.hotkeySettings.Form())
}

// activeFormTableEditor returns the settings flow first because its modal state remains independent when the launcher is activated.
func (a *App) activeFormTableEditor() *formTableEditorState {
	if a.settingsTableEditor != nil {
		return a.settingsTableEditor
	}
	return a.launcherTableEditor
}

// formTableTargetCurrentWithFormsLocked compares one table target using controller pointers captured for the UI-thread snapshot.
func (a *App) formTableTargetCurrentWithFormsLocked(target *formFieldsState, pluginForm *pluginSettingsFormState, aiForm *formFieldsState, hotkeyForm *formFieldsState) bool {
	return target != nil && ((a.form != nil && target == &a.form.formFieldsState) ||
		(a.requirementForm != nil && target == &a.requirementForm.formFieldsState) ||
		(pluginForm != nil && target == &pluginForm.formFieldsState) ||
		(a.settingsOpen && a.settingTab == "ai" && target == aiForm) ||
		(a.settingsOpen && a.settingTab == "general" && target == hotkeyForm))
}

func (a *App) openActionFormTable(index int) {
	if a.form != nil {
		a.openFormTableLocked(&a.form.formFieldsState, index)
	}
	a.finishOpeningFormTable()
}

func (a *App) openRequirementFormTable(index int) {
	if a.requirementForm != nil {
		a.openFormTableLocked(&a.requirementForm.formFieldsState, index)
	}
	a.finishOpeningFormTable()
}

func (a *App) openPluginFormTable(index int) {
	a.pluginSettings.SetSearchFocused(false)
	pluginForm := a.pluginSettings.Form()
	if pluginForm != nil {
		a.openFormTableLocked(&pluginForm.formFieldsState, index)
	}
	a.finishOpeningFormTable()
}

func (a *App) finishOpeningFormTable() {
	a.updateFormTableTextInput(false)
	a.invalidateFormTableWindow()
}

func (a *App) openFormTableLocked(target *formFieldsState, index int) {
	if target == nil || index < 0 || index >= len(target.definitions) || target.definitions[index].Type != "table" {
		return
	}
	syncFormFieldsEditorLocked(target)
	setFormFieldsFocusLocked(target, index)
	definition := target.definitions[index]
	rows, err := decodeFormTableRows(target.values[definition.Value.Key])
	selected := -1
	if len(rows) > 0 {
		selected = 0
	}
	state := &formTableEditorState{target: target, fieldIndex: index, definition: definition, rows: rows, selected: selected, rowIndex: -1, deletePending: -1}
	if err != nil {
		state.rows = []map[string]any{}
		state.invalid = true
		state.status = "Invalid table JSON. Close this editor without saving to preserve the original value."
	}
	if a.formTableTargetUsesSettingsLocked(target) {
		a.settingsTableEditor = state
	} else {
		a.launcherTableEditor = state
	}
}

// closeFormTableEditor returns input ownership to the form that opened the portable table overlay.
func (a *App) closeFormTableEditor() {
	a.stopHotkeyRecording()
	state := a.activeFormTableEditor()
	if state == a.settingsTableEditor {
		a.settingsTableEditor = nil
	} else if state == a.launcherTableEditor {
		a.launcherTableEditor = nil
	}
	textInput := state != nil && a.formTableTargetCurrentLocked(state.target) && state.target.editor != nil
	settingsTarget := state != nil && a.formTableTargetUsesSettingsLocked(state.target)
	if settingsTarget {
		a.updateSettingsTextInput(textInput)
		a.invalidateSettingsWindow()
	} else {
		a.updateFormTextInput(textInput)
		_ = a.window.Invalidate()
	}
}

func (a *App) selectFormTableRow(index int) {
	state := a.activeFormTableEditor()
	if state != nil && state.rowForm == nil && index >= 0 && index < len(state.rows) {
		state.selected = index
		state.deletePending = -1
		state.deleteDirect = false
		state.status = ""
	}
	a.invalidateFormTableWindow()
}

func (a *App) moveFormTableSelection(delta int) {
	state := a.activeFormTableEditor()
	if state != nil && state.rowForm == nil && len(state.rows) > 0 {
		if state.selected < 0 {
			state.selected = 0
		} else {
			state.selected = (state.selected + delta + len(state.rows)) % len(state.rows)
		}
		state.deletePending = -1
		state.deleteDirect = false
		state.status = ""
	}
	a.invalidateFormTableWindow()
}

func formTableColumnValue(column formTableColumn, row map[string]any) string {
	value, ok := row[column.Key]
	if !ok || value == nil {
		return ""
	}
	if column.Type == "woxImage" {
		return formTableWoxImageValue(value)
	}
	if column.Type == "app" {
		return formTableAppValue(value)
	}
	if column.Type == "textList" {
		switch list := value.(type) {
		case []any:
			items := make([]string, 0, len(list))
			for _, item := range list {
				items = append(items, fmt.Sprint(item))
			}
			return strings.Join(items, "\n")
		case []string:
			return strings.Join(list, "\n")
		}
	}
	if column.Type == "selectAIModel" {
		if text, ok := value.(string); ok {
			return text
		}
		if encoded, err := json.Marshal(value); err == nil {
			return string(encoded)
		}
	}
	return fmt.Sprint(value)
}

func formTableColumnDefinition(column formTableColumn, row map[string]any) (formDefinition, bool) {
	value := formDefinitionValue{Key: column.Key, Label: column.Label, Tooltip: column.Tooltip, Validators: column.Validators}
	switch column.Type {
	case "text", "queryHotkeyQuery", "aiCommandPrompt", "dictationPrompt":
		value.MaxLines = max(1, column.TextMaxLines)
		return formDefinition{Type: "textbox", Value: value}, true
	case "dirPath":
		value.MaxLines = 1
		return formDefinition{Type: "dirPath", Value: value}, true
	case "textList":
		value.MaxLines = max(4, column.TextMaxLines)
		return formDefinition{Type: "textbox", Value: value}, true
	case "checkbox":
		return formDefinition{Type: "checkbox", Value: value}, true
	case "select":
		value.Options = append([]formOption(nil), column.SelectOptions...)
		return formDefinition{Type: "select", Value: value}, true
	case "selectAIModel":
		return formDefinition{Type: "selectAIModel", Value: value}, true
	case "hotkey":
		return formDefinition{Type: "hotkey", Value: value}, true
	case "woxImage":
		value.MaxLines = 1
		return formDefinition{Type: "woxImage", Value: value}, true
	case "app":
		return formDefinition{Type: "app", Value: value}, true
	default:
		current := formTableColumnValue(column, row)
		if current == "" {
			current = "Not editable in Go UI yet"
		} else {
			current = "Read-only in Go UI: " + current
		}
		// ponytail: Specialized table columns stay read-only until a real setting needs their native picker; untouched values still round-trip.
		return formDefinition{Type: "label", Value: formDefinitionValue{Content: column.Label + " · " + current}}, false
	}
}

func formTableRowFields(definition formDefinition, row map[string]any) (formFieldsState, map[string]bool) {
	definitions := make([]formDefinition, 0, len(definition.Value.Columns))
	values := make(map[string]string, len(definition.Value.Columns))
	textLists := make(map[string]bool)
	for _, column := range definition.Value.Columns {
		if column.HideInUpdate {
			continue
		}
		field, editable := formTableColumnDefinition(column, row)
		definitions = append(definitions, field)
		if !editable {
			continue
		}
		value, exists := row[column.Key]
		if !exists {
			switch column.Type {
			case "checkbox":
				values[column.Key] = "false"
			case "select":
				if len(column.SelectOptions) > 0 {
					values[column.Key] = column.SelectOptions[0].Value
				}
			case "woxImage":
				values[column.Key] = "🤖"
			case "app":
				values[column.Key] = "{}"
			}
		} else {
			values[column.Key] = formTableColumnValue(column, map[string]any{column.Key: value})
		}
		if column.EmptyAsZero {
			values[column.Key] = normalizeEmptyAsZeroFormValue(values[column.Key])
		}
		if column.Type == "textList" {
			textLists[column.Key] = true
		}
	}
	return newFormFieldsState(definitions, values, true), textLists
}

// normalizeEmptyAsZeroFormValue maps blank and zero to an empty editor value for EmptyAsZero columns.
func normalizeEmptyAsZeroFormValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "0" {
		return ""
	}
	return trimmed
}

func (a *App) beginAddFormTableRow() {
	a.beginFormTableRowEdit(-1, false, false)
}

func (a *App) beginAddFormTableRowDirect() {
	if a.beginWindowManagerGroupEdit(-1, true) {
		return
	}
	a.beginFormTableRowEdit(-1, true, false)
}

func (a *App) beginEditFormTableRow() {
	a.beginSelectedFormTableRowEdit(false)
}

func (a *App) beginEditFormTableRowDirect() {
	if state := a.activeFormTableEditor(); state != nil && isWindowManagerGroupsEditor(a, state) {
		if state.selected >= 0 {
			if a.beginWindowManagerGroupEdit(state.selected, true) {
				return
			}
		}
	}
	a.beginSelectedFormTableRowEdit(true)
}

// beginSelectedFormTableRowEdit preserves whether the selected row came from the list or an inline table.
func (a *App) beginSelectedFormTableRowEdit(rowEditorOnly bool) {
	index := -1
	if state := a.activeFormTableEditor(); state != nil {
		index = state.selected
	}
	if index >= 0 {
		a.beginFormTableRowEdit(index, rowEditorOnly, false)
	}
}

// beginCloneFormTableRowDirect opens a new-row editor prefilled from the selected row.
func (a *App) beginCloneFormTableRowDirect() {
	index := -1
	if state := a.activeFormTableEditor(); state != nil {
		index = state.selected
	}
	if index >= 0 {
		a.beginFormTableRowEdit(index, true, true)
	}
}

func (a *App) beginFormTableRowEdit(index int, rowEditorOnly, cloneRow bool) {
	requestModels := false
	state := a.activeFormTableEditor()
	if state == nil || state.invalid || state.saving || state.rowForm != nil || index >= len(state.rows) {
		return
	}
	if index >= 0 && (state.definition.Value.Key == "AISkills" || formTableSkillRowReadOnly(state.definition, state.rows[index])) {
		return
	}
	base := map[string]any{}
	if index >= 0 {
		base = cloneFormTableRow(state.rows[index])
	}
	if cloneRow {
		index = -1
	}
	fields, _ := formTableRowFields(state.definition, base)
	if state.definition.Value.Key == "QueryHotkeys" {
		state.queryPreset = inferQueryHotkeyPreset(fields.values)
	}
	if models := a.aiSettings.Models(); len(models) > 0 {
		applyAIModelOptionsLocked(&fields, models)
	}
	state.rowForm = &fields
	state.appPicker = nil
	state.queryVariable = nil
	state.rowIndex = index
	state.rowBase = base
	state.rowEditorOnly = rowEditorOnly
	clearFormTableRowValidationLocked(state)
	state.deletePending = -1
	state.deleteDirect = false
	applyAIProviderDefaultHostLocked(state, false, a.aiSettings.ProviderCatalog())
	requestModels = hasFormDefinitionType(fields.definitions, "selectAIModel") && !a.aiSettings.ModelsLoaded() && !a.aiSettings.ModelsLoading()
	if requestModels {
		a.aiSettings.SetModelsLoading(true)
	}
	textInput := fields.editor != nil
	a.updateFormTableTextInput(textInput)
	if requestModels {
		util.Go(a.lifecycleCtx, "load AI models for form table", a.loadAIModels)
	}
	a.invalidateFormTableWindow()
}

// inferQueryHotkeyPreset maps persisted display values back to Flutter's task-oriented presets.
func inferQueryHotkeyPreset(values map[string]string) queryHotkeyPreset {
	width := strings.TrimSpace(values["Width"])
	maxResults := strings.TrimSpace(values["MaxResultCount"])
	hasDisplayOptions := values["Position"] != "" && values["Position"] != "system_default" ||
		values["HideQueryBox"] == "true" || values["HideToolbar"] == "true" ||
		width != "" && width != "0" || maxResults != "" && maxResults != "0"
	if values["IsSilentExecution"] == "true" && !hasDisplayOptions {
		return queryHotkeyPresetSilent
	}
	if values["HideQueryBox"] == "true" && values["HideToolbar"] == "true" {
		return queryHotkeyPresetWebPanel
	}
	if hasDisplayOptions {
		return queryHotkeyPresetCustom
	}
	return queryHotkeyPresetNormal
}

// queryHotkeyFieldVisible keeps each preset limited to the fields shown by Flutter.
func queryHotkeyFieldVisible(preset queryHotkeyPreset, key string, editing bool) bool {
	switch key {
	case "Name", "Hotkey", "Query":
		return true
	case "Position", "Width", "MaxResultCount":
		return preset == queryHotkeyPresetWebPanel || preset == queryHotkeyPresetCustom
	case "IsSilentExecution", "HideQueryBox", "HideToolbar":
		return preset == queryHotkeyPresetCustom
	case "Disabled":
		return editing
	default:
		return false
	}
}

// applyQueryHotkeyPreset mirrors Flutter's task presets and clears display values hidden by the selected mode.
func (a *App) applyQueryHotkeyPreset(preset string) {
	state := a.activeFormTableEditor()
	if state == nil || state.definition.Value.Key != "QueryHotkeys" || state.rowForm == nil {
		return
	}
	syncFormFieldsEditorLocked(state.rowForm)
	selected := queryHotkeyPreset(preset)
	switch selected {
	case queryHotkeyPresetNormal, queryHotkeyPresetSilent:
		state.rowForm.values["Position"] = "system_default"
		state.rowForm.values["HideQueryBox"] = "false"
		state.rowForm.values["HideToolbar"] = "false"
		state.rowForm.values["Width"] = ""
		state.rowForm.values["MaxResultCount"] = ""
		state.rowForm.values["IsSilentExecution"] = fmt.Sprint(selected == queryHotkeyPresetSilent)
	case queryHotkeyPresetWebPanel:
		state.rowForm.values["Position"] = "center"
		state.rowForm.values["HideQueryBox"] = "true"
		state.rowForm.values["HideToolbar"] = "true"
		state.rowForm.values["Width"] = "500"
		state.rowForm.values["MaxResultCount"] = "12"
		state.rowForm.values["IsSilentExecution"] = "false"
	case queryHotkeyPresetCustom:
	default:
		return
	}
	state.queryPreset = selected
	state.queryVariable = nil
	state.status = ""
	a.updateFormTableTextInput(false)
	a.invalidateFormTableWindow()
}

func (a *App) cancelFormTableRowEdit() {
	state := a.activeFormTableEditor()
	closeEditor := state != nil && state.rowEditorOnly
	if closeEditor {
		a.closeFormTableEditor()
		return
	}
	a.stopHotkeyRecording()
	if state := a.activeFormTableEditor(); state != nil {
		state.rowForm = nil
		state.rowIndex = -1
		state.rowBase = nil
		state.rowEditorOnly = false
		state.appPicker = nil
		state.queryVariable = nil
		state.status = ""
	}
	a.updateFormTableTextInput(false)
	a.invalidateFormTableWindow()
}

func formTableRowFromFields(definition formDefinition, fields *formFieldsState, base map[string]any) map[string]any {
	row := cloneFormTableRow(base)
	delete(row, formTableRowIDKey)
	for key := range row {
		if strings.HasPrefix(key, "_wox_original_") {
			delete(row, key)
		}
	}
	for _, column := range definition.Value.Columns {
		if column.HideInUpdate {
			continue
		}
		value, editable := fields.values[column.Key]
		if !editable {
			continue
		}
		switch column.Type {
		case "checkbox":
			row[column.Key] = value == "true"
		case "textList":
			lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
			items := make([]string, 0, len(lines))
			for _, line := range lines {
				if line != "" {
					items = append(items, line)
				}
			}
			row[column.Key] = items
		case "text", "dirPath", "queryHotkeyQuery", "aiCommandPrompt", "dictationPrompt", "select", "selectAIModel", "hotkey":
			row[column.Key] = value
		case "woxImage":
			image, _ := parseFormTableWoxImage(value)
			row[column.Key] = image
		case "app":
			app, _ := parseFormTableApp(value)
			row[column.Key] = app
		}
	}
	normalizeEmptyAsZeroFieldValues(definition, fields.values)
	for _, column := range definition.Value.Columns {
		if column.EmptyAsZero {
			row[column.Key], _ = strconv.Atoi(strings.TrimSpace(fields.values[column.Key]))
		}
	}
	return row
}

// normalizeEmptyAsZeroFieldValues keeps editor values blank when EmptyAsZero columns still contain a stored 0.
func normalizeEmptyAsZeroFieldValues(definition formDefinition, values map[string]string) {
	for _, column := range definition.Value.Columns {
		if column.EmptyAsZero {
			values[column.Key] = normalizeEmptyAsZeroFormValue(values[column.Key])
		}
	}
}

func cloneFormTableFieldErrors(errors map[string]string) map[string]string {
	if len(errors) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(errors))
	for key, message := range errors {
		cloned[key] = message
	}
	return cloned
}

// clearFormTableRowValidationLocked clears dialog-level and per-field validation after the user edits again.
func clearFormTableRowValidationLocked(state *formTableEditorState) {
	if state == nil {
		return
	}
	state.status = ""
	state.fieldErrors = nil
}

func (a *App) translateFormTableFieldErrors(errors map[string]string) map[string]string {
	if len(errors) == 0 {
		return nil
	}
	translated := make(map[string]string, len(errors))
	for key, message := range errors {
		translated[key] = a.translate(message)
	}
	return translated
}

func validateFormTableRow(definition formDefinition, fields *formFieldsState, rows []map[string]any, editingIndex int) map[string]string {
	normalizeEmptyAsZeroFieldValues(definition, fields.values)
	errors := validateFormFieldErrors(fields.definitions, fields.values)
	if errors == nil {
		errors = map[string]string{}
	}
	for _, column := range definition.Value.Columns {
		if errors[column.Key] != "" {
			continue
		}
		if column.Type == "woxImage" {
			if _, err := parseFormTableWoxImage(fields.values[column.Key]); err != nil {
				errors[column.Key] = err.Error()
				continue
			}
		}
		if column.Type == "app" {
			if _, err := parseFormTableApp(fields.values[column.Key]); err != nil {
				errors[column.Key] = err.Error()
				continue
			}
		}
		unique := false
		for _, validator := range column.Validators {
			if validator.Type == "unique" {
				unique = true
				break
			}
		}
		if !unique {
			continue
		}
		candidate := fields.values[column.Key]
		for index, row := range rows {
			if index != editingIndex && formTableColumnValue(column, row) == candidate {
				errors[column.Key] = "i18n:ui_validator_value_must_be_unique"
				break
			}
		}
	}
	if len(errors) == 0 {
		return nil
	}
	return errors
}

// formTableWoxImageValue presents the common emoji case compactly while preserving every structured image type as JSON.
func formTableWoxImageValue(value any) string {
	if image, ok := value.(woxImage); ok {
		if image.ImageType == "emoji" {
			return image.ImageData
		}
	}
	if image, ok := value.(map[string]any); ok {
		imageType, _ := image["ImageType"].(string)
		imageData, _ := image["ImageData"].(string)
		if imageType == "emoji" {
			return imageData
		}
	}
	if encoded, err := json.Marshal(value); err == nil {
		return string(encoded)
	}
	return fmt.Sprint(value)
}

// parseFormTableWoxImage turns the portable emoji shorthand or a full WoxImage object into the core DTO shape.
func parseFormTableWoxImage(value string) (map[string]any, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("Icon must not be empty")
	}
	if !strings.HasPrefix(value, "{") {
		return map[string]any{"ImageType": "emoji", "ImageData": value}, nil
	}
	var image woxImage
	if err := json.Unmarshal([]byte(value), &image); err != nil {
		return nil, fmt.Errorf("Icon must be an emoji or valid WoxImage JSON: %w", err)
	}
	if strings.TrimSpace(image.ImageType) == "" || strings.TrimSpace(image.ImageData) == "" {
		return nil, fmt.Errorf("WoxImage JSON requires ImageType and ImageData")
	}
	return map[string]any{"ImageType": image.ImageType, "ImageData": image.ImageData}, nil
}

func formTableAppValue(value any) string {
	if encoded, err := json.Marshal(value); err == nil {
		return string(encoded)
	}
	return "{}"
}

func parseFormTableApp(value string) (map[string]any, error) {
	var app ignoredHotkeyApp
	if err := json.Unmarshal([]byte(value), &app); err != nil {
		return nil, fmt.Errorf("Application selection is invalid: %w", err)
	}
	if strings.TrimSpace(app.Identity) == "" {
		return nil, fmt.Errorf("Select an application")
	}
	encoded, err := json.Marshal(app)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(encoded, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (a *App) saveFormTableRowEdit() {
	a.stopHotkeyRecording()
	state := a.activeFormTableEditor()
	if state == nil || state.rowForm == nil || state.invalid || state.saving || !a.formTableTargetCurrentLocked(state.target) {
		return
	}
	syncFormFieldsEditorLocked(state.rowForm)
	clearFormTableRowValidationLocked(state)
	if validationMessage := a.validatePluginTriggerKeywordTableRow(state); validationMessage != "" {
		state.fieldErrors = map[string]string{"keyword": validationMessage}
		a.invalidateFormTableWindow()
		return
	}
	if fieldErrors := validateFormTableRow(state.definition, state.rowForm, state.rows, state.rowIndex); len(fieldErrors) > 0 {
		if a.activeFormTableEditor() == state {
			state.fieldErrors = a.translateFormTableFieldErrors(fieldErrors)
		}
		a.invalidateFormTableWindow()
		return
	}
	if fieldErrors := validateAISettingsTableRow(state.definition, state.rowForm); len(fieldErrors) > 0 {
		state.fieldErrors = fieldErrors
		a.invalidateFormTableWindow()
		return
	}
	previousValue := state.target.values[state.definition.Value.Key]
	row := formTableRowFromFields(state.definition, state.rowForm, state.rowBase)
	if state.rowIndex >= 0 && state.rowIndex < len(state.rows) {
		state.rows[state.rowIndex] = row
		state.selected = state.rowIndex
	} else {
		state.rows = append(state.rows, row)
		state.selected = len(state.rows) - 1
	}
	if err := a.commitFormTableRowsLocked(state); err != nil {
		state.status = err.Error()
		a.invalidateFormTableWindow()
		return
	}
	pluginForm := a.pluginSettings.Form()
	pluginTarget := pluginForm != nil && state.target == &pluginForm.formFieldsState
	persist := state.target == a.aiSettings.Form() || state.target == a.hotkeySettings.Form()
	closeEditor := state.rowEditorOnly
	key := state.definition.Value.Key
	value := state.target.values[key]
	state.rowForm = nil
	state.rowIndex = -1
	state.rowBase = nil
	state.rowEditorOnly = false
	clearFormTableRowValidationLocked(state)
	if persist {
		state.saving = true
		a.settingSaving = true
	}
	if closeEditor {
		a.closeFormTableEditor()
	} else {
		a.updateFormTableTextInput(false)
		a.invalidateFormTableWindow()
	}
	if persist {
		util.Go(a.lifecycleCtx, "save settings table", func() {
			a.saveSettingsTable(state, key, value, previousValue)
		})
	} else if pluginTarget {
		a.submitPluginSettings()
	}
}

func (a *App) deleteFormTableRow() {
	a.beginFormTableRowDelete(false)
}

// beginDeleteFormTableRowDirect opens Flutter's confirmation dialog directly over the settings page.
func (a *App) beginDeleteFormTableRowDirect() {
	a.beginFormTableRowDelete(true)
}

func (a *App) beginFormTableRowDelete(direct bool) {
	state := a.activeFormTableEditor()
	if state == nil || state.invalid || state.saving || state.rowForm != nil || state.selected < 0 || state.selected >= len(state.rows) || !a.formTableTargetCurrentLocked(state.target) || formTableSkillRowReadOnly(state.definition, state.rows[state.selected]) {
		return
	}
	if len(state.rows) <= state.definition.Value.MinimumRowCount {
		state.status = a.translate(state.definition.Value.MinimumRowMessage)
		a.invalidateFormTableWindow()
		return
	}
	state.deletePending = state.selected
	state.deleteDirect = direct
	state.status = ""
	a.updateFormTableTextInput(false)
	a.invalidateFormTableWindow()
}

// cancelFormTableRowDelete dismisses the confirmation without mutating the table.
func (a *App) cancelFormTableRowDelete() {
	state := a.activeFormTableEditor()
	if state == nil || state.deletePending < 0 {
		return
	}
	direct := state.deleteDirect
	state.deletePending = -1
	state.deleteDirect = false
	if direct {
		a.closeFormTableEditor()
		return
	}
	a.invalidateFormTableWindow()
}

// confirmFormTableRowDelete re-matches the captured row against the latest table value before removal.
func (a *App) confirmFormTableRowDelete() {
	state := a.activeFormTableEditor()
	if state == nil || state.invalid || state.saving || state.rowForm != nil || state.deletePending < 0 || state.deletePending >= len(state.rows) || !a.formTableTargetCurrentLocked(state.target) {
		return
	}
	originalRow := cloneFormTableRow(state.rows[state.deletePending])
	direct := state.deleteDirect
	state.deletePending = -1
	state.deleteDirect = false
	previousValue := state.target.values[state.definition.Value.Key]
	freshRows, err := decodeFormTableRows(previousValue)
	if err != nil {
		state.status = err.Error()
		a.invalidateFormTableWindow()
		return
	}
	deleteIndex := findFormTableRow(freshRows, originalRow)
	if deleteIndex < 0 {
		if direct {
			a.closeFormTableEditor()
		} else {
			a.invalidateFormTableWindow()
		}
		return
	}
	state.rows = append(freshRows[:deleteIndex], freshRows[deleteIndex+1:]...)
	state.selected = min(deleteIndex, len(state.rows)-1)
	if len(state.rows) == 0 {
		state.selected = -1
	}
	persist := false
	pluginTarget := false
	key := state.definition.Value.Key
	value := ""
	if err := a.commitFormTableRowsLocked(state); err != nil {
		state.status = err.Error()
	} else {
		state.status = ""
		persist = state.target == a.aiSettings.Form() || state.target == a.hotkeySettings.Form()
		pluginForm := a.pluginSettings.Form()
		pluginTarget = pluginForm != nil && state.target == &pluginForm.formFieldsState
		value = state.target.values[key]
		if persist {
			state.saving = true
			a.settingSaving = true
		}
	}
	if direct {
		a.closeFormTableEditor()
	} else {
		a.invalidateFormTableWindow()
	}
	if persist {
		util.Go(a.lifecycleCtx, "save settings table after delete", func() {
			a.saveSettingsTable(state, key, value, previousValue)
		})
	} else if pluginTarget {
		a.submitPluginSettings()
	}
}

func findFormTableRow(rows []map[string]any, original map[string]any) int {
	for index, candidate := range rows {
		matches := true
		for key, value := range original {
			if key == formTableRowIDKey {
				continue
			}
			if fmt.Sprint(candidate[key]) != fmt.Sprint(value) {
				matches = false
				break
			}
		}
		if matches {
			return index
		}
	}
	return -1
}

func (a *App) commitFormTableRowsLocked(state *formTableEditorState) error {
	rows := cloneFormTableRows(state.rows)
	for _, row := range rows {
		delete(row, formTableRowIDKey)
	}
	encoded, err := json.Marshal(rows)
	if err != nil {
		return fmt.Errorf("encode table rows: %w", err)
	}
	state.target.values[state.definition.Value.Key] = string(encoded)
	return nil
}

func (a *App) focusFormTableRowField(index int) {
	var target *formFieldsState
	if state := a.activeFormTableEditor(); state != nil {
		target = state.rowForm
	}
	a.stopHotkeyRecordingForDifferentField(target, index)
	state := a.activeFormTableEditor()
	if state == nil || state.rowForm == nil || index < 0 || index >= len(state.rowForm.definitions) || !formDefinitionFocusable(state.rowForm.definitions[index]) {
		return
	}
	syncFormFieldsEditorLocked(state.rowForm)
	state.rowForm.active = true
	if state.rowForm.focused != index || state.rowForm.editor == nil {
		setFormFieldsFocusLocked(state.rowForm, index)
	}
	textInput := state.rowForm.editor != nil
	a.updateFormTableTextInput(textInput)
	a.invalidateFormTableWindow()
}

func (a *App) moveFormTableRowFocus(delta int) {
	state := a.activeFormTableEditor()
	if state == nil || state.rowForm == nil || len(state.rowForm.definitions) == 0 {
		return
	}
	syncFormFieldsEditorLocked(state.rowForm)
	index := state.rowForm.focused
	for step := 0; step < len(state.rowForm.definitions); step++ {
		index = (index + delta + len(state.rowForm.definitions)) % len(state.rowForm.definitions)
		if formDefinitionFocusable(state.rowForm.definitions[index]) {
			setFormFieldsFocusLocked(state.rowForm, index)
			break
		}
	}
	host := a.host
	if a.formTableUsesSettingsWindow() {
		host = a.settingsHost
	}
	if host != nil {
		host.RequestFocus(woxwidget.Key(fmt.Sprintf("form-table-row-field-%d", index)))
	}
	a.stopHotkeyRecordingForDifferentField(state.rowForm, index)
	textInput := state.rowForm.editor != nil
	a.updateFormTableTextInput(textInput)
	a.invalidateFormTableWindow()
}

func (a *App) changeFormTableRowChoice(index, delta int) {
	if state := a.activeFormTableEditor(); state != nil && state.rowForm != nil {
		changeFormFieldsChoiceLocked(state.rowForm, index, delta)
		if index >= 0 && index < len(state.rowForm.definitions) && state.rowForm.definitions[index].Value.Key == "Name" {
			applyAIProviderDefaultHostLocked(state, true, a.aiSettings.ProviderCatalog())
		}
		clearFormTableRowValidationLocked(state)
	}
	a.updateFormTableTextInput(false)
	a.invalidateFormTableWindow()
}

func (a *App) editFormTableRowKey(event woxui.KeyEvent) {
	state := a.activeFormTableEditor()
	if state != nil && state.rowForm != nil && state.rowForm.editor != nil && state.rowForm.focused >= 0 && state.rowForm.focused < len(state.rowForm.definitions) {
		_, changed := handleFormEditorKey(state.rowForm.editor, state.rowForm.definitions[state.rowForm.focused], event)
		if changed {
			syncFormFieldsEditorLocked(state.rowForm)
			clearFormTableRowValidationLocked(state)
		}
	}
	a.invalidateFormTableWindow()
}

func (a *App) setFormTableRowText(index int, value string) {
	state := a.activeFormTableEditor()
	changed := state != nil && state.rowForm != nil && !state.saving && setFormFieldsTextLocked(state.rowForm, index, value)
	if changed {
		clearFormTableRowValidationLocked(state)
		if state.definition.Value.Key == "QueryHotkeys" && index >= 0 && index < len(state.rowForm.definitions) && state.rowForm.definitions[index].Value.Key == "Query" {
			a.updateFormTableQueryVariableTrigger(index)
		}
	}
	if changed {
		a.invalidateFormTableWindow()
	}
}

func queryVariableTriggerStart(value string, caret int) int {
	runes := []rune(value)
	caret = max(0, min(caret, len(runes)))
	runes = runes[:caret]
	for index := len(runes) - 1; index >= 0; index-- {
		if runes[index] != '{' {
			continue
		}
		for _, current := range runes[index+1:] {
			if current == '}' || unicode.IsSpace(current) {
				return -1
			}
		}
		return index
	}
	return -1
}

func (a *App) updateFormTableQueryVariableTrigger(index int) {
	state := a.activeFormTableEditor()
	if state == nil || state.rowForm == nil || state.rowForm.editor == nil || index < 0 || index >= len(state.rowForm.definitions) || state.rowForm.definitions[index].Value.Key != "Query" {
		return
	}
	editorState := state.rowForm.editor.State()
	start := queryVariableTriggerStart(editorState.Text, editorState.Selection.Focus)
	if start < 0 {
		state.queryVariable = nil
		return
	}
	anchor := woxui.Rect{}
	if state.queryVariable != nil && state.queryVariable.fieldIndex == index {
		anchor = state.queryVariable.anchor
	} else {
		host := a.host
		if a.settingsTableEditor != nil {
			host = a.settingsHost
		}
		if host != nil {
			anchor, _ = host.BoundsForKey(woxwidget.Key(fmt.Sprintf("form-table-row-field-%d", index)))
		}
	}
	state.queryVariable = &formTableQueryVariablePickerState{fieldIndex: index, anchor: anchor, triggerStart: start}
}

func (a *App) filteredQueryHotkeyVariables(query string) []queryHotkeyVariable {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return queryHotkeyVariables
	}
	filtered := make([]queryHotkeyVariable, 0, len(queryHotkeyVariables))
	for _, option := range queryHotkeyVariables {
		searchable := option.value + " " + a.translate(option.label)
		if strings.Contains(strings.ToLower(searchable), query) {
			filtered = append(filtered, option)
		}
	}
	if len(filtered) == 0 {
		return queryHotkeyVariables
	}
	return filtered
}

func (a *App) openFormTableQueryVariablePicker(index int, anchor woxui.Rect) {
	state := a.activeFormTableEditor()
	if state == nil || state.rowForm == nil || index < 0 || index >= len(state.rowForm.definitions) || state.rowForm.definitions[index].Value.Key != "Query" {
		return
	}
	if state.rowForm.focused != index || state.rowForm.editor == nil {
		setFormFieldsFocusLocked(state.rowForm, index)
	}
	state.appPicker = nil
	state.choicePicker = nil
	state.queryVariable = &formTableQueryVariablePickerState{fieldIndex: index, anchor: anchor, triggerStart: -1}
	state.status = ""
	host := a.host
	if a.settingsTableEditor != nil {
		host = a.settingsHost
	}
	if host != nil {
		host.RequestFocus(woxwidget.Key(fmt.Sprintf("form-table-row-field-%d", index)))
	}
	a.updateFormTableTextInput(true)
	a.invalidateFormTableWindow()
}

func (a *App) closeFormTableQueryVariablePicker() {
	state := a.activeFormTableEditor()
	if state != nil {
		state.queryVariable = nil
	}
	a.updateFormTableTextInput(state != nil && state.rowForm != nil && state.rowForm.editor != nil)
	a.invalidateFormTableWindow()
}

func (a *App) chooseFormTableQueryVariable(index int) {
	state := a.activeFormTableEditor()
	if state == nil || state.rowForm == nil || state.queryVariable == nil {
		return
	}
	picker := state.queryVariable
	options := a.filteredQueryHotkeyVariables(snapshotFormTableEditorLocked(state).queryVariable.query)
	if index < 0 || index >= len(options) || picker.fieldIndex < 0 || picker.fieldIndex >= len(state.rowForm.definitions) {
		return
	}
	if state.rowForm.focused != picker.fieldIndex || state.rowForm.editor == nil {
		setFormFieldsFocusLocked(state.rowForm, picker.fieldIndex)
	}
	editor := state.rowForm.editor
	selection := editor.State().Selection
	start, end := selection.Start(), selection.End()
	if picker.triggerStart >= 0 {
		start = picker.triggerStart
		end = selection.Focus
	}
	runes := []rune(editor.State().Text)
	start = max(0, min(start, len(runes)))
	end = max(start, min(end, len(runes)))
	inserted := []rune(options[index].value)
	next := append(append(append([]rune{}, runes[:start]...), inserted...), runes[end:]...)
	editor.SetText(string(next), false)
	editor.SetCaret(start + len(inserted))
	syncFormFieldsEditorLocked(state.rowForm)
	state.queryVariable = nil
	state.status = ""
	a.updateFormTableTextInput(true)
	a.invalidateFormTableWindow()
}

func (a *App) moveFormTableQueryVariableSelection(delta int) {
	state := a.activeFormTableEditor()
	if state == nil || state.queryVariable == nil {
		return
	}
	snapshot := snapshotFormTableEditorLocked(state).queryVariable
	count := len(a.filteredQueryHotkeyVariables(snapshot.query))
	if count > 0 {
		state.queryVariable.selected = (state.queryVariable.selected + delta + count) % count
	}
	a.invalidateFormTableWindow()
}

// pickFormTableRowImage stores an uploaded image in the same portable WoxImage shape used by Flutter.
func (a *App) pickFormTableRowImage(index int) {
	state := a.activeFormTableEditor()
	if state == nil || state.rowForm == nil || index < 0 || index >= len(state.rowForm.definitions) || state.rowForm.definitions[index].Type != "woxImage" {
		return
	}
	rowForm := state.rowForm

	a.updateFormTableTextInput(false)
	path, err := a.formTableNativeWindow().PickFile(woxui.FileDialogOptions{})
	var image woxImage
	if err == nil && path != "" {
		var data []byte
		data, err = os.ReadFile(path)
		if err == nil {
			if strings.EqualFold(filepath.Ext(path), ".svg") {
				image = woxImage{ImageType: "svg", ImageData: string(data)}
			} else {
				mediaType := mime.TypeByExtension(filepath.Ext(path))
				if mediaType == "" {
					mediaType = "application/octet-stream"
				}
				image = woxImage{ImageType: "base64", ImageData: "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(data)}
			}
		}
	}

	if a.activeFormTableEditor() != state || state.rowForm != rowForm {
		return
	}
	if err != nil {
		state.status = err.Error()
	} else if path != "" {
		encoded, encodeErr := json.Marshal(image)
		if encodeErr != nil {
			state.status = encodeErr.Error()
		} else {
			rowForm.values[rowForm.definitions[index].Value.Key] = string(encoded)
			state.status = ""
		}
	}
	a.updateFormTableTextInput(false)
	a.invalidateFormTableWindow()
}

// pickFormTableRowDirectory uses the platform window adapter while keeping the selected path in the shared row form.
func (a *App) pickFormTableRowDirectory(index int) {
	state := a.activeFormTableEditor()
	if state == nil || state.rowForm == nil || index < 0 || index >= len(state.rowForm.definitions) || state.rowForm.definitions[index].Type != "dirPath" {
		return
	}
	rowForm := state.rowForm
	a.updateFormTableTextInput(false)
	path, err := a.formTableNativeWindow().PickFile(woxui.FileDialogOptions{Directory: true})
	if a.activeFormTableEditor() != state || state.rowForm != rowForm {
		return
	}
	if err != nil {
		state.status = err.Error()
	} else if path != "" {
		setFormFieldsFocusLocked(rowForm, index)
		rowForm.editor.SetText(path, false)
		syncFormFieldsEditorLocked(rowForm)
		state.status = ""
	}
	textInput := rowForm.editor != nil
	a.updateFormTableTextInput(textInput)
	a.invalidateFormTableWindow()
}

// onFormTableKey gives the modal table editor first refusal before launcher or settings navigation.
func (a *App) onFormTableKey(event woxui.KeyEvent) bool {
	state := a.activeFormTableEditor()
	if state == nil || !a.formTableTargetCurrentLocked(state.target) {
		return false
	}
	rowForm := state.rowForm
	focused := -1
	fieldType := ""
	if rowForm != nil {
		focused = rowForm.focused
		if focused >= 0 && focused < len(rowForm.definitions) {
			fieldType = rowForm.definitions[focused].Type
		}
	}
	if !event.Down {
		return false
	}
	if queryVariable := state.queryVariable; queryVariable != nil {
		switch event.Key {
		case woxui.KeyEscape:
			a.closeFormTableQueryVariablePicker()
			return true
		case woxui.KeyArrowUp:
			a.moveFormTableQueryVariableSelection(-1)
			return true
		case woxui.KeyArrowDown:
			a.moveFormTableQueryVariableSelection(1)
			return true
		case woxui.KeyEnter, woxui.KeyTab:
			a.chooseFormTableQueryVariable(queryVariable.selected)
			return true
		}
	}
	if event.Composing {
		return false
	}
	if state.skillAdd != nil {
		// The add-skill dialog owns Enter and Escape; printable keys continue into
		// the focused text field for normal editing.
		if event.Down {
			switch event.Key {
			case woxui.KeyEscape:
				a.cancelFormTableSkillAdd()
				return true
			case woxui.KeyEnter:
				a.addFormTableSkill()
				return true
			}
		}
		return false
	}
	if editor := state.windowGroupEditor; editor != nil {
		if event.Key == woxui.KeyEscape {
			if editor.appPickerSlot != "" {
				a.closeWindowManagerGroupAppPicker()
			} else if editor.urlEditorSlot != "" {
				a.cancelWindowManagerGroupUrlEditor()
			} else {
				a.cancelWindowManagerGroupEdit()
			}
			return true
		}
		// The retained dialog controls own navigation and editing. Returning false for
		// printable keys lets the native window continue into its text input client.
		return false
	}
	selected := state.selected
	saving := state.saving
	appPicker := state.appPicker
	choicePicker := state.choicePicker
	multiline := false
	textEditable := false
	if rowForm != nil {
		if focused >= 0 && focused < len(rowForm.definitions) {
			multiline = fieldType == "textbox" && rowForm.definitions[focused].Value.MaxLines > 1
			textEditable = formDefinitionTextEditable(rowForm.definitions[focused])
		}
	}
	if state.deletePending >= 0 {
		if event.Down {
			switch event.Key {
			case woxui.KeyEscape:
				a.cancelFormTableRowDelete()
			case woxui.KeyEnter:
				a.confirmFormTableRowDelete()
			}
		}
		return true
	}
	if choicePicker != nil {
		if event.Key == woxui.KeyEscape {
			a.closeFormTableChoicePicker()
		}
		return true
	}
	if state.emojiPicker != nil {
		if event.Key == woxui.KeyEscape {
			a.closeFormTableEmojiPicker()
			return true
		}
		// Printable keys must continue into the focused search field's native text input client.
		return false
	}
	if appPicker != nil {
		if event.Key == woxui.KeyEscape {
			a.closeFormTableAppPicker()
			return true
		}
		return false
	}
	if event.Key == woxui.KeyEscape {
		if rowForm != nil {
			a.cancelFormTableRowEdit()
		} else {
			a.closeFormTableEditor()
		}
		return true
	}
	if saving {
		return true
	}
	if rowForm == nil {
		switch event.Key {
		case woxui.KeyArrowUp:
			a.moveFormTableSelection(-1)
		case woxui.KeyArrowDown:
			a.moveFormTableSelection(1)
		case woxui.KeyEnter:
			if selected >= 0 {
				a.beginEditFormTableRow()
			} else {
				a.beginAddFormTableRow()
			}
		case woxui.KeyDelete:
			a.deleteFormTableRow()
		default:
			if event.Modifiers.HasPrimary() && event.Key == woxui.Key("n") {
				a.beginAddFormTableRow()
			} else {
				return true
			}
		}
		return true
	}
	if event.Modifiers.HasPrimary() && (event.Key == woxui.KeyEnter || event.Key == woxui.Key("s")) {
		a.saveFormTableRowEdit()
		return true
	}
	if textEditable {
		switch event.Key {
		case woxui.KeyArrowDown:
			if !multiline {
				a.moveFormTableRowFocus(1)
				return true
			}
		case woxui.KeyArrowUp:
			if !multiline {
				a.moveFormTableRowFocus(-1)
				return true
			}
		case woxui.KeyEnter:
			return !multiline
		}
		return false
	}
	if event.Key == woxui.KeyTab && event.Modifiers & ^woxui.KeyModifierShift == 0 {
		return false
	}
	switch event.Key {
	case woxui.KeyArrowDown:
		if multiline {
			a.editFormTableRowKey(event)
			break
		}
		a.moveFormTableRowFocus(1)
	case woxui.KeyArrowUp:
		if multiline {
			a.editFormTableRowKey(event)
		} else {
			a.moveFormTableRowFocus(-1)
		}
	case woxui.KeyArrowLeft:
		if fieldType == "select" || fieldType == "selectAIModel" {
			a.changeFormTableRowChoice(focused, -1)
		} else {
			a.editFormTableRowKey(event)
		}
	case woxui.KeyArrowRight:
		if fieldType == "select" || fieldType == "selectAIModel" {
			a.changeFormTableRowChoice(focused, 1)
		} else {
			a.editFormTableRowKey(event)
		}
	case woxui.KeySpace, woxui.KeyEnter:
		if event.Key == woxui.KeyEnter && multiline {
			a.editFormTableRowKey(event)
		} else if fieldType == "hotkey" {
			a.recordFormTableRowHotkey(focused)
		} else if fieldType == "app" {
			a.openFormTableAppPicker(focused)
		} else if fieldType == "select" || fieldType == "selectAIModel" {
			a.openFocusedFormTableRowChoice(focused)
		} else if fieldType == "checkbox" {
			a.changeFormTableRowChoice(focused, 1)
		}
	default:
		a.editFormTableRowKey(event)
	}
	return true
}

func (a *App) onFormTableTextInput(_ woxui.TextInputEvent) bool {
	state := a.activeFormTableEditor()
	if state == nil || !a.formTableTargetCurrentLocked(state.target) {
		return false
	}
	return state.appPicker == nil && state.emojiPicker == nil
}
