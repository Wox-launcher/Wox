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
	"strings"

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
	rowEditorOnly bool
	skillClone    bool
	status        string
	invalid       bool
	saving        bool
	deletePending int
	deleteDirect  bool
	appPicker     *formTableAppPickerState
	choicePicker  *formTableChoicePickerState
}

type formTableEditorSnapshot struct {
	definition    formDefinition
	rows          []map[string]any
	selected      int
	rowForm       *formFieldsSnapshot
	rowIndex      int
	skillClone    bool
	status        string
	invalid       bool
	saving        bool
	deletePending int
	deleteDirect  bool
	appPicker     *formTableAppPickerSnapshot
	choicePicker  *formTableChoicePickerSnapshot
}

type formTableAppPickerState struct {
	fieldIndex int
	candidates []ignoredHotkeyApp
	selected   int
}

type formTableAppPickerSnapshot struct {
	fieldIndex int
	candidates []ignoredHotkeyApp
	selected   int
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
			candidates: append([]ignoredHotkeyApp(nil), state.appPicker.candidates...),
			selected:   state.appPicker.selected,
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
	return &formTableEditorSnapshot{
		definition:    state.definition,
		rows:          cloneFormTableRows(state.rows),
		selected:      state.selected,
		rowForm:       rowForm,
		rowIndex:      state.rowIndex,
		skillClone:    state.skillClone,
		status:        state.status,
		invalid:       state.invalid,
		saving:        state.saving,
		deletePending: state.deletePending,
		deleteDirect:  state.deleteDirect,
		appPicker:     appPicker,
		choicePicker:  choicePicker,
	}
}

func (a *App) formTableTargetCurrentLocked(target *formFieldsState) bool {
	pluginForm := a.pluginSettings.Form()
	return a.formTableTargetCurrentWithFormsLocked(target, pluginForm, a.aiSettings.Form(), a.hotkeySettings.Form())
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
	a.tableEditor = state
}

// closeFormTableEditor returns input ownership to the form that opened the portable table overlay.
func (a *App) closeFormTableEditor() {
	a.stopHotkeyRecording()
	state := a.tableEditor
	a.tableEditor = nil
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
	state := a.tableEditor
	if state != nil && state.rowForm == nil && index >= 0 && index < len(state.rows) {
		state.selected = index
		state.deletePending = -1
		state.deleteDirect = false
		state.status = ""
	}
	a.invalidateFormTableWindow()
}

func (a *App) moveFormTableSelection(delta int) {
	state := a.tableEditor
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
		if column.Type == "textList" {
			textLists[column.Key] = true
		}
	}
	return newFormFieldsState(definitions, values, true), textLists
}

func (a *App) beginAddFormTableRow() {
	a.beginFormTableRowEdit(-1, false, false)
}

func (a *App) beginAddFormTableRowDirect() {
	a.beginFormTableRowEdit(-1, true, false)
}

func (a *App) beginEditFormTableRow() {
	a.beginSelectedFormTableRowEdit(false)
}

func (a *App) beginEditFormTableRowDirect() {
	a.beginSelectedFormTableRowEdit(true)
}

// beginSelectedFormTableRowEdit preserves whether the selected row came from the list or an inline table.
func (a *App) beginSelectedFormTableRowEdit(rowEditorOnly bool) {
	index := -1
	if a.tableEditor != nil {
		index = a.tableEditor.selected
	}
	if index >= 0 {
		a.beginFormTableRowEdit(index, rowEditorOnly, false)
	}
}

// beginCloneFormTableRowDirect opens a new-row editor prefilled from the selected row.
func (a *App) beginCloneFormTableRowDirect() {
	index := -1
	if a.tableEditor != nil {
		index = a.tableEditor.selected
	}
	if index >= 0 {
		a.beginFormTableRowEdit(index, true, true)
	}
}

func (a *App) beginFormTableRowEdit(index int, rowEditorOnly, cloneRow bool) {
	requestModels := false
	state := a.tableEditor
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
	if models := a.aiSettings.Models(); len(models) > 0 {
		applyAIModelOptionsLocked(&fields, models)
	}
	state.rowForm = &fields
	state.appPicker = nil
	state.rowIndex = index
	state.rowBase = base
	state.rowEditorOnly = rowEditorOnly
	state.skillClone = false
	state.status = ""
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

func (a *App) cancelFormTableRowEdit() {
	closeEditor := a.tableEditor != nil && a.tableEditor.rowEditorOnly
	if closeEditor {
		a.closeFormTableEditor()
		return
	}
	a.stopHotkeyRecording()
	if a.tableEditor != nil {
		a.tableEditor.rowForm = nil
		a.tableEditor.rowIndex = -1
		a.tableEditor.rowBase = nil
		a.tableEditor.rowEditorOnly = false
		a.tableEditor.appPicker = nil
		a.tableEditor.skillClone = false
		a.tableEditor.status = ""
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
	return row
}

func validateFormTableRow(definition formDefinition, fields *formFieldsState, rows []map[string]any, editingIndex int) string {
	if validationKey := validateFormFields(fields.definitions, fields.values); validationKey != "" {
		return validationKey
	}
	for _, column := range definition.Value.Columns {
		if column.Type == "woxImage" {
			if _, err := parseFormTableWoxImage(fields.values[column.Key]); err != nil {
				return err.Error()
			}
		}
		if column.Type == "app" {
			if _, err := parseFormTableApp(fields.values[column.Key]); err != nil {
				return err.Error()
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
				return "i18n:ui_validator_value_must_be_unique"
			}
		}
	}
	return ""
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
	state := a.tableEditor
	if state == nil || state.rowForm == nil || state.invalid || state.saving || !a.formTableTargetCurrentLocked(state.target) {
		return
	}
	syncFormFieldsEditorLocked(state.rowForm)
	if state.skillClone {
		if validationKey := validateFormFields(state.rowForm.definitions, state.rowForm.values); validationKey != "" {
			message := a.translate(validationKey)
			if a.tableEditor == state {
				state.status = message
			}
			a.invalidateFormTableWindow()
			return
		}
		url := strings.TrimSpace(state.rowForm.values["SourceUrl"])
		previousValue := state.target.values[state.definition.Value.Key]
		state.rowForm = nil
		state.rowIndex = -1
		state.rowBase = nil
		state.skillClone = false
		state.saving = true
		state.status = "Cloning remote skills…"
		a.settingSaving = true
		a.updateFormTableTextInput(false)
		a.invalidateFormTableWindow()
		util.Go(a.lifecycleCtx, "clone remote AI skills", func() {
			a.cloneRemoteAISkills(state, url, previousValue)
		})
		return
	}
	if validationKey := validateFormTableRow(state.definition, state.rowForm, state.rows, state.rowIndex); validationKey != "" {
		message := a.translate(validationKey)
		if a.tableEditor == state {
			state.status = message
		}
		a.invalidateFormTableWindow()
		return
	}
	if validationMessage := validateAISettingsTableRow(state.definition, state.rowForm); validationMessage != "" {
		state.status = validationMessage
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
	state.status = ""
	if persist {
		state.saving = true
		state.status = "Saving…"
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
	state := a.tableEditor
	if state == nil || state.invalid || state.saving || state.rowForm != nil || state.selected < 0 || state.selected >= len(state.rows) || !a.formTableTargetCurrentLocked(state.target) || formTableSkillRowReadOnly(state.definition, state.rows[state.selected]) {
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
	state := a.tableEditor
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
	state := a.tableEditor
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
			state.status = "Saving…"
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
	if a.tableEditor != nil {
		target = a.tableEditor.rowForm
	}
	a.stopHotkeyRecordingForDifferentField(target, index)
	state := a.tableEditor
	if state == nil || state.rowForm == nil || index < 0 || index >= len(state.rowForm.definitions) || !formDefinitionFocusable(state.rowForm.definitions[index]) {
		return
	}
	syncFormFieldsEditorLocked(state.rowForm)
	setFormFieldsFocusLocked(state.rowForm, index)
	state.status = ""
	textInput := state.rowForm.editor != nil
	a.updateFormTableTextInput(textInput)
	a.invalidateFormTableWindow()
}

func (a *App) moveFormTableRowFocus(delta int) {
	state := a.tableEditor
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
	if state := a.tableEditor; state != nil && state.rowForm != nil {
		changeFormFieldsChoiceLocked(state.rowForm, index, delta)
		if index >= 0 && index < len(state.rowForm.definitions) && state.rowForm.definitions[index].Value.Key == "Name" {
			applyAIProviderDefaultHostLocked(state, true, a.aiSettings.ProviderCatalog())
		}
		state.status = ""
	}
	a.updateFormTableTextInput(false)
	a.invalidateFormTableWindow()
}

func (a *App) editFormTableRowKey(event woxui.KeyEvent) {
	state := a.tableEditor
	if state != nil && state.rowForm != nil && state.rowForm.editor != nil && state.rowForm.focused >= 0 && state.rowForm.focused < len(state.rowForm.definitions) {
		_, changed := handleFormEditorKey(state.rowForm.editor, state.rowForm.definitions[state.rowForm.focused], event)
		if changed {
			syncFormFieldsEditorLocked(state.rowForm)
			state.status = ""
		}
	}
	a.invalidateFormTableWindow()
}

func (a *App) setFormTableRowText(index int, value string) {
	state := a.tableEditor
	changed := state != nil && state.rowForm != nil && !state.saving && setFormFieldsTextLocked(state.rowForm, index, value)
	if changed {
		state.status = ""
	}
	if changed {
		a.invalidateFormTableWindow()
	}
}

// beginFormTableRowEmojiEdit selects the current icon value so the next emoji input replaces it.
func (a *App) beginFormTableRowEmojiEdit(index int) {
	state := a.tableEditor
	if state == nil || state.rowForm == nil || index < 0 || index >= len(state.rowForm.definitions) || state.rowForm.definitions[index].Type != "woxImage" {
		return
	}
	setFormFieldsFocusLocked(state.rowForm, index)
	state.rowForm.editor.SelectAll()
	state.status = ""
	a.updateFormTableTextInput(true)
	a.invalidateFormTableWindow()
}

// pickFormTableRowImage stores an uploaded image in the same portable WoxImage shape used by Flutter.
func (a *App) pickFormTableRowImage(index int) {
	state := a.tableEditor
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

	if a.tableEditor != state || state.rowForm != rowForm {
		return
	}
	if err != nil {
		state.status = err.Error()
	} else if path != "" {
		encoded, encodeErr := json.Marshal(image)
		if encodeErr != nil {
			state.status = encodeErr.Error()
		} else {
			setFormFieldsFocusLocked(rowForm, index)
			rowForm.editor.SetText(string(encoded), false)
			syncFormFieldsEditorLocked(rowForm)
			state.status = ""
		}
	}
	a.updateFormTableTextInput(false)
	a.invalidateFormTableWindow()
}

// pickFormTableRowDirectory uses the platform window adapter while keeping the selected path in the shared row form.
func (a *App) pickFormTableRowDirectory(index int) {
	state := a.tableEditor
	if state == nil || state.rowForm == nil || index < 0 || index >= len(state.rowForm.definitions) || state.rowForm.definitions[index].Type != "dirPath" {
		return
	}
	rowForm := state.rowForm
	a.updateFormTableTextInput(false)
	path, err := a.formTableNativeWindow().PickFile(woxui.FileDialogOptions{Directory: true})
	if a.tableEditor != state || state.rowForm != rowForm {
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
	state := a.tableEditor
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
	if !event.Down || event.Composing {
		return false
	}
	selected := state.selected
	saving := state.saving
	appPicker := state.appPicker
	appSelected := -1
	if appPicker != nil {
		appSelected = appPicker.selected
	}
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
	if appPicker != nil {
		a.onFormTableAppPickerKey(event, appSelected)
		return true
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
	state := a.tableEditor
	if state == nil || !a.formTableTargetCurrentLocked(state.target) {
		return false
	}
	return true
}
