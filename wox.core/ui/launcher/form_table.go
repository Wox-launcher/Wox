package launcher

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	woxui "wox/ui/runtime"
)

const formTableRowIDKey = "wox_table_row_id"

type formTableEditorState struct {
	target       *formFieldsState
	fieldIndex   int
	definition   formDefinition
	rows         []map[string]any
	selected     int
	listScroll   float32
	listViewport float32
	rowForm      *formFieldsState
	rowIndex     int
	rowBase      map[string]any
	skillClone   bool
	status       string
	invalid      bool
	saving       bool
	deleteArmed  int
	appPicker    *formTableAppPickerState
}

type formTableEditorSnapshot struct {
	definition  formDefinition
	rows        []map[string]any
	selected    int
	listScroll  float32
	rowForm     *formFieldsSnapshot
	rowIndex    int
	skillClone  bool
	status      string
	invalid     bool
	saving      bool
	deleteArmed int
	appPicker   *formTableAppPickerSnapshot
}

type formTableAppPickerState struct {
	fieldIndex int
	candidates []ignoredHotkeyApp
	selected   int
	scroll     float32
	viewport   float32
}

type formTableAppPickerSnapshot struct {
	fieldIndex int
	candidates []ignoredHotkeyApp
	selected   int
	scroll     float32
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
			scroll:     state.appPicker.scroll,
		}
	}
	return &formTableEditorSnapshot{
		definition:  state.definition,
		rows:        cloneFormTableRows(state.rows),
		selected:    state.selected,
		listScroll:  state.listScroll,
		rowForm:     rowForm,
		rowIndex:    state.rowIndex,
		skillClone:  state.skillClone,
		status:      state.status,
		invalid:     state.invalid,
		saving:      state.saving,
		deleteArmed: state.deleteArmed,
		appPicker:   appPicker,
	}
}

func (a *App) formTableTargetCurrentLocked(target *formFieldsState) bool {
	return target != nil && ((a.form != nil && target == &a.form.formFieldsState) ||
		(a.requirementForm != nil && target == &a.requirementForm.formFieldsState) ||
		(a.pluginForm != nil && target == &a.pluginForm.formFieldsState) ||
		(a.settingsOpen && a.settingTab == "ai" && target == a.aiSettingsForm) ||
		(a.settingsOpen && a.settingTab == "general" && target == a.hotkeySettingsForm))
}

func (a *App) openActionFormTable(index int) {
	a.mu.Lock()
	if a.form != nil {
		a.openFormTableLocked(&a.form.formFieldsState, index)
	}
	a.mu.Unlock()
	a.finishOpeningFormTable()
}

func (a *App) openRequirementFormTable(index int) {
	a.mu.Lock()
	if a.requirementForm != nil {
		a.openFormTableLocked(&a.requirementForm.formFieldsState, index)
	}
	a.mu.Unlock()
	a.finishOpeningFormTable()
}

func (a *App) openPluginFormTable(index int) {
	a.mu.Lock()
	if a.pluginForm != nil {
		a.openFormTableLocked(&a.pluginForm.formFieldsState, index)
	}
	a.mu.Unlock()
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
	state := &formTableEditorState{target: target, fieldIndex: index, definition: definition, rows: rows, selected: selected, rowIndex: -1, deleteArmed: -1}
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
	a.mu.Lock()
	state := a.tableEditor
	a.tableEditor = nil
	textInput := state != nil && a.formTableTargetCurrentLocked(state.target) && state.target.editor != nil
	settingsTarget := state != nil && a.formTableTargetUsesSettingsLocked(state.target)
	a.mu.Unlock()
	if settingsTarget {
		a.updateSettingsTextInput(textInput)
		a.invalidateSettingsWindow()
	} else {
		a.updateFormTextInput(textInput)
		_ = a.window.Invalidate()
	}
}

func (a *App) selectFormTableRow(index int) {
	a.mu.Lock()
	state := a.tableEditor
	if state != nil && state.rowForm == nil && index >= 0 && index < len(state.rows) {
		state.selected = index
		state.deleteArmed = -1
		state.status = ""
		a.ensureFormTableSelectionVisibleLocked()
	}
	a.mu.Unlock()
	a.invalidateFormTableWindow()
}

func (a *App) moveFormTableSelection(delta int) {
	a.mu.Lock()
	state := a.tableEditor
	if state != nil && state.rowForm == nil && len(state.rows) > 0 {
		if state.selected < 0 {
			state.selected = 0
		} else {
			state.selected = (state.selected + delta + len(state.rows)) % len(state.rows)
		}
		state.deleteArmed = -1
		state.status = ""
		a.ensureFormTableSelectionVisibleLocked()
	}
	a.mu.Unlock()
	a.invalidateFormTableWindow()
}

func (a *App) setFormTableListViewport(height float32) {
	a.mu.Lock()
	if a.tableEditor != nil {
		a.tableEditor.listViewport = max(float32(1), height)
		a.ensureFormTableSelectionVisibleLocked()
	}
	a.mu.Unlock()
}

func (a *App) scrollFormTableList(delta float32) {
	a.mu.Lock()
	state := a.tableEditor
	if state != nil && state.rowForm == nil {
		maxOffset := max(float32(0), float32(len(state.rows))*formTableListRowHeight-state.listViewport)
		state.listScroll = min(max(float32(0), state.listScroll+delta), maxOffset)
	}
	a.mu.Unlock()
	a.invalidateFormTableWindow()
}

func (a *App) ensureFormTableSelectionVisibleLocked() {
	state := a.tableEditor
	if state == nil || state.selected < 0 {
		return
	}
	viewport := max(float32(1), state.listViewport)
	rowTop := float32(state.selected) * formTableListRowHeight
	rowBottom := rowTop + formTableListRowHeight
	if rowTop < state.listScroll {
		state.listScroll = rowTop
	} else if rowBottom > state.listScroll+viewport {
		state.listScroll = rowBottom - viewport
	}
	maxOffset := max(float32(0), float32(len(state.rows))*formTableListRowHeight-viewport)
	state.listScroll = min(max(float32(0), state.listScroll), maxOffset)
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
		value.Label += " (emoji or WoxImage JSON)"
		value.MaxLines = 1
		return formDefinition{Type: "textbox", Value: value}, true
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
	a.beginFormTableRowEdit(-1)
}

func (a *App) beginEditFormTableRow() {
	a.mu.RLock()
	index := -1
	if a.tableEditor != nil {
		index = a.tableEditor.selected
	}
	a.mu.RUnlock()
	if index >= 0 {
		a.beginFormTableRowEdit(index)
	}
}

func (a *App) beginFormTableRowEdit(index int) {
	requestModels := false
	a.mu.Lock()
	state := a.tableEditor
	if state == nil || state.invalid || state.saving || state.rowForm != nil || index >= len(state.rows) {
		a.mu.Unlock()
		return
	}
	if index >= 0 && (state.definition.Value.Key == "AISkills" || formTableSkillRowReadOnly(state.definition, state.rows[index])) {
		a.mu.Unlock()
		return
	}
	base := map[string]any{}
	if index >= 0 {
		base = cloneFormTableRow(state.rows[index])
	}
	fields, _ := formTableRowFields(state.definition, base)
	if len(a.aiModels) > 0 {
		applyAIModelOptionsLocked(&fields, a.aiModels)
	}
	state.rowForm = &fields
	state.appPicker = nil
	state.rowIndex = index
	state.rowBase = base
	state.skillClone = false
	state.status = ""
	state.deleteArmed = -1
	applyAIProviderDefaultHostLocked(state, false, a.aiProviderCatalog)
	requestModels = hasFormDefinitionType(fields.definitions, "selectAIModel") && !a.aiModelsLoaded && !a.aiModelsLoading
	if requestModels {
		a.aiModelsLoading = true
	}
	textInput := fields.editor != nil
	a.mu.Unlock()
	a.updateFormTableTextInput(textInput)
	if requestModels {
		go a.loadAIModels()
	}
	a.invalidateFormTableWindow()
}

func (a *App) cancelFormTableRowEdit() {
	a.stopHotkeyRecording()
	a.mu.Lock()
	if a.tableEditor != nil {
		a.tableEditor.rowForm = nil
		a.tableEditor.rowIndex = -1
		a.tableEditor.rowBase = nil
		a.tableEditor.appPicker = nil
		a.tableEditor.skillClone = false
		a.tableEditor.status = ""
	}
	a.mu.Unlock()
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
	a.mu.Lock()
	state := a.tableEditor
	if state == nil || state.rowForm == nil || state.invalid || state.saving || !a.formTableTargetCurrentLocked(state.target) {
		a.mu.Unlock()
		return
	}
	syncFormFieldsEditorLocked(state.rowForm)
	if state.skillClone {
		if validationKey := validateFormFields(state.rowForm.definitions, state.rowForm.values); validationKey != "" {
			a.mu.Unlock()
			message := a.translate(validationKey)
			a.mu.Lock()
			if a.tableEditor == state {
				state.status = message
			}
			a.mu.Unlock()
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
		a.mu.Unlock()
		a.updateFormTableTextInput(false)
		a.invalidateFormTableWindow()
		go a.cloneRemoteAISkills(state, url, previousValue)
		return
	}
	if validationKey := validateFormTableRow(state.definition, state.rowForm, state.rows, state.rowIndex); validationKey != "" {
		a.mu.Unlock()
		message := a.translate(validationKey)
		a.mu.Lock()
		if a.tableEditor == state {
			state.status = message
		}
		a.mu.Unlock()
		a.invalidateFormTableWindow()
		return
	}
	if validationMessage := validateAISettingsTableRow(state.definition, state.rowForm); validationMessage != "" {
		state.status = validationMessage
		a.mu.Unlock()
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
		a.mu.Unlock()
		a.invalidateFormTableWindow()
		return
	}
	persist := state.target == a.aiSettingsForm || state.target == a.hotkeySettingsForm
	key := state.definition.Value.Key
	value := state.target.values[key]
	state.rowForm = nil
	state.rowIndex = -1
	state.rowBase = nil
	state.status = ""
	if persist {
		state.saving = true
		state.status = "Saving…"
		a.settingSaving = true
	}
	a.ensureFormTableSelectionVisibleLocked()
	a.mu.Unlock()
	a.updateFormTableTextInput(false)
	a.invalidateFormTableWindow()
	if persist {
		go a.saveSettingsTable(state, key, value, previousValue)
	}
}

func (a *App) deleteFormTableRow() {
	a.mu.Lock()
	state := a.tableEditor
	if state == nil || state.invalid || state.saving || state.rowForm != nil || state.selected < 0 || state.selected >= len(state.rows) || !a.formTableTargetCurrentLocked(state.target) || formTableSkillRowReadOnly(state.definition, state.rows[state.selected]) {
		a.mu.Unlock()
		return
	}
	if state.deleteArmed != state.selected {
		state.deleteArmed = state.selected
		state.status = "Press Delete again to confirm removing the selected row."
		a.mu.Unlock()
		a.invalidateFormTableWindow()
		return
	}
	previousValue := state.target.values[state.definition.Value.Key]
	state.rows = append(state.rows[:state.selected], state.rows[state.selected+1:]...)
	if state.selected >= len(state.rows) {
		state.selected = len(state.rows) - 1
	}
	persist := false
	key := state.definition.Value.Key
	value := ""
	if err := a.commitFormTableRowsLocked(state); err != nil {
		state.status = err.Error()
	} else {
		state.status = ""
		persist = state.target == a.aiSettingsForm || state.target == a.hotkeySettingsForm
		value = state.target.values[key]
		if persist {
			state.saving = true
			state.status = "Saving…"
			a.settingSaving = true
		}
	}
	state.deleteArmed = -1
	a.ensureFormTableSelectionVisibleLocked()
	a.mu.Unlock()
	a.invalidateFormTableWindow()
	if persist {
		go a.saveSettingsTable(state, key, value, previousValue)
	}
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
	a.mu.RLock()
	var target *formFieldsState
	if a.tableEditor != nil {
		target = a.tableEditor.rowForm
	}
	a.mu.RUnlock()
	a.stopHotkeyRecordingForDifferentField(target, index)
	a.mu.Lock()
	state := a.tableEditor
	if state == nil || state.rowForm == nil || index < 0 || index >= len(state.rowForm.definitions) || !formDefinitionFocusable(state.rowForm.definitions[index]) {
		a.mu.Unlock()
		return
	}
	syncFormFieldsEditorLocked(state.rowForm)
	setFormFieldsFocusLocked(state.rowForm, index)
	state.status = ""
	textInput := state.rowForm.editor != nil
	a.mu.Unlock()
	a.updateFormTableTextInput(textInput)
	a.invalidateFormTableWindow()
}

func (a *App) moveFormTableRowFocus(delta int) {
	a.mu.Lock()
	state := a.tableEditor
	if state == nil || state.rowForm == nil || len(state.rowForm.definitions) == 0 {
		a.mu.Unlock()
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
	textInput := state.rowForm.editor != nil
	a.mu.Unlock()
	a.updateFormTableTextInput(textInput)
	a.invalidateFormTableWindow()
}

func (a *App) changeFormTableRowChoice(index, delta int) {
	a.mu.Lock()
	if state := a.tableEditor; state != nil && state.rowForm != nil {
		changeFormFieldsChoiceLocked(state.rowForm, index, delta)
		if index >= 0 && index < len(state.rowForm.definitions) && state.rowForm.definitions[index].Value.Key == "Name" {
			applyAIProviderDefaultHostLocked(state, true, a.aiProviderCatalog)
		}
		state.status = ""
	}
	a.mu.Unlock()
	a.updateFormTableTextInput(false)
	a.invalidateFormTableWindow()
}

func (a *App) editFormTableRowKey(event woxui.KeyEvent) {
	a.mu.Lock()
	state := a.tableEditor
	if state != nil && state.rowForm != nil && state.rowForm.editor != nil && state.rowForm.focused >= 0 && state.rowForm.focused < len(state.rowForm.definitions) {
		_, changed := handleFormEditorKey(state.rowForm.editor, state.rowForm.definitions[state.rowForm.focused], event)
		if changed {
			syncFormFieldsEditorLocked(state.rowForm)
			state.status = ""
		}
	}
	a.mu.Unlock()
	a.invalidateFormTableWindow()
}

func (a *App) setFormTableRowCaret(index, offset int) {
	a.mu.Lock()
	state := a.tableEditor
	if state != nil && state.rowForm != nil && state.rowForm.focused == index && state.rowForm.editor != nil {
		state.rowForm.editor.SetCaret(offset)
	}
	a.mu.Unlock()
	a.invalidateFormTableWindow()
}

// pickFormTableRowDirectory uses the platform window adapter while keeping the selected path in the shared row form.
func (a *App) pickFormTableRowDirectory(index int) {
	a.mu.RLock()
	state := a.tableEditor
	if state == nil || state.rowForm == nil || index < 0 || index >= len(state.rowForm.definitions) || state.rowForm.definitions[index].Type != "dirPath" {
		a.mu.RUnlock()
		return
	}
	rowForm := state.rowForm
	a.mu.RUnlock()
	a.updateFormTableTextInput(false)
	path, err := a.formTableNativeWindow().PickFile(woxui.FileDialogOptions{Directory: true})
	a.mu.Lock()
	if a.tableEditor != state || state.rowForm != rowForm {
		a.mu.Unlock()
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
	a.mu.Unlock()
	a.updateFormTableTextInput(textInput)
	a.invalidateFormTableWindow()
}

func (a *App) setFormTableRowViewport(height float32) {
	a.mu.Lock()
	if state := a.tableEditor; state != nil && state.rowForm != nil {
		state.rowForm.viewportHeight = max(float32(1), height)
		state.rowForm.scroll = min(state.rowForm.scroll, max(float32(0), formDefinitionsContentHeight(state.rowForm.definitions)-state.rowForm.viewportHeight))
	}
	a.mu.Unlock()
}

func (a *App) scrollFormTableRow(delta float32) {
	a.mu.Lock()
	if state := a.tableEditor; state != nil && state.rowForm != nil {
		maxOffset := max(float32(0), formDefinitionsContentHeight(state.rowForm.definitions)-state.rowForm.viewportHeight)
		state.rowForm.scroll = min(max(float32(0), state.rowForm.scroll+delta), maxOffset)
	}
	a.mu.Unlock()
	a.invalidateFormTableWindow()
}

// onFormTableKey gives the modal table editor first refusal before launcher or settings navigation.
func (a *App) onFormTableKey(event woxui.KeyEvent) bool {
	a.mu.RLock()
	state := a.tableEditor
	if state == nil || !a.formTableTargetCurrentLocked(state.target) {
		a.mu.RUnlock()
		return false
	}
	rowForm := state.rowForm
	selected := state.selected
	saving := state.saving
	appPicker := state.appPicker
	appSelected := -1
	if appPicker != nil {
		appSelected = appPicker.selected
	}
	fieldType := ""
	multiline := false
	focused := -1
	if rowForm != nil {
		focused = rowForm.focused
		if focused >= 0 && focused < len(rowForm.definitions) {
			fieldType = rowForm.definitions[focused].Type
			multiline = fieldType == "textbox" && rowForm.definitions[focused].Value.MaxLines > 1
		}
	}
	a.mu.RUnlock()
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
	switch event.Key {
	case woxui.KeyTab, woxui.KeyArrowDown:
		if event.Key == woxui.KeyArrowDown && multiline {
			a.editFormTableRowKey(event)
			break
		}
		delta := 1
		if event.Key == woxui.KeyTab && event.Modifiers&woxui.KeyModifierShift != 0 {
			delta = -1
		}
		a.moveFormTableRowFocus(delta)
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
		} else if fieldType == "checkbox" || fieldType == "select" || fieldType == "selectAIModel" {
			a.changeFormTableRowChoice(focused, 1)
		}
	default:
		a.editFormTableRowKey(event)
	}
	return true
}

func (a *App) onFormTableTextInput(event woxui.TextInputEvent) bool {
	a.mu.Lock()
	state := a.tableEditor
	if state == nil || !a.formTableTargetCurrentLocked(state.target) {
		a.mu.Unlock()
		return false
	}
	if state.appPicker != nil {
		a.mu.Unlock()
		return true
	}
	if state.rowForm != nil && state.rowForm.editor != nil && state.rowForm.focused >= 0 && state.rowForm.focused < len(state.rowForm.definitions) && formDefinitionTextEditable(state.rowForm.definitions[state.rowForm.focused]) {
		if state.rowForm.editor.HandleTextInput(event) {
			syncFormFieldsEditorLocked(state.rowForm)
			state.status = ""
		}
	}
	a.mu.Unlock()
	a.invalidateFormTableWindow()
	return true
}
