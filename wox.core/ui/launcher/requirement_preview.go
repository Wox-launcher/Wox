package launcher

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"wox/ui/coreclient"
	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

type queryRequirementPreviewRequirement struct {
	SettingKey string `json:"SettingKey"`
	Message    string `json:"Message"`
}

type queryRequirementPreviewData struct {
	PluginID           string                               `json:"PluginId"`
	PluginName         string                               `json:"PluginName"`
	Title              string                               `json:"Title"`
	Message            string                               `json:"Message"`
	Requirements       []queryRequirementPreviewRequirement `json:"Requirements"`
	SettingDefinitions []formDefinition                     `json:"SettingDefinitions"`
	Values             map[string]string                    `json:"Values"`
}

type requirementFormState struct {
	formFieldsState
	key        string
	pluginID   string
	pluginName string
	title      string
	message    string
	saving     bool
	error      string
	revision   uint64
}

type requirementFormSnapshot struct {
	formFieldsSnapshot
	key         string
	pluginID    string
	pluginName  string
	title       string
	message     string
	saving      bool
	error       string
	modelsError string
}

type aiModel struct {
	Name          string `json:"Name"`
	Provider      string `json:"Provider"`
	ProviderAlias string `json:"ProviderAlias"`
}

// buildRequirementPreview adapts requirement state and form rows to the pure preview view.
func (a *App) buildRequirementPreview(result queryResult, preview queryPreview, palette uiPalette, width, height float32) woxwidget.Widget {
	form, err := a.requirementFormSnapshotFor(result, preview)
	if err != nil {
		return previewview.RequirementPreviewView(previewview.RequirementPreviewProps{Width: width, Height: height, Theme: palette.componentTheme(), FatalError: err.Error()})
	}
	errorMessage := form.error
	if errorMessage == "" && form.modelsError != "" && hasFormDefinitionType(form.definitions, "selectAIModel") {
		errorMessage = "Unable to load AI models: " + form.modelsError
	}
	callbacks := formFieldCallbacks{
		idPrefix: "requirement-form", focus: a.focusRequirementFormField, change: a.changeRequirementFormChoice,
		setText: a.setRequirementFormText, onKey: a.onRequirementFormKey, openTable: a.openRequirementFormTable,
	}
	rows := make([]woxwidget.Widget, 0, len(form.definitions))
	for index, definition := range form.definitions {
		rows = append(rows, a.buildFormField(form.formFieldsSnapshot, callbacks, palette, index, definition, width-36, formDefinitionHeight(definition, form.values)))
	}
	return previewview.RequirementPreviewView(previewview.RequirementPreviewProps{
		Width: width, Height: height, Theme: palette.componentTheme(), Title: form.title, Message: form.message, PluginName: form.pluginName,
		Error: errorMessage, SaveLabel: a.translate("i18n:ui_save"), Saving: form.saving, Rows: rows,
		RowsHeight: formDefinitionsContentHeight(form.definitions, form.values), KeepVisible: formFieldsKeepVisible(form.formFieldsSnapshot),
		OnSubmit: a.submitRequirementForm,
	})
}

// requirementPreviewDataAndKey validates the payload and derives its stable controller identity.
func requirementPreviewDataAndKey(result queryResult, preview queryPreview) (queryRequirementPreviewData, string, error) {
	var data queryRequirementPreviewData
	if err := json.Unmarshal([]byte(preview.PreviewData), &data); err != nil {
		return queryRequirementPreviewData{}, "", fmt.Errorf("decode requirement settings: %w", err)
	}
	if data.PluginID == "" {
		return queryRequirementPreviewData{}, "", fmt.Errorf("requirement settings are missing PluginId")
	}
	hash := sha256.Sum256([]byte(preview.PreviewData))
	return data, fmt.Sprintf("%s|%s|%x", result.QueryID, result.ID, hash), nil
}

// activateRequirementPreview prepares form state and optional model data before rendering.
func (a *App) activateRequirementPreview(result queryResult, preview queryPreview) error {
	data, key, err := requirementPreviewDataAndKey(result, preview)
	if err != nil {
		return err
	}
	a.mu.RLock()
	changed := a.requirementForm != nil && a.requirementForm.key != key
	a.mu.RUnlock()
	if changed {
		a.deactivateRequirementForm()
	}

	a.mu.Lock()
	if a.requirementForm == nil || a.requirementForm.key != key {
		fields := newFormFieldsState(data.SettingDefinitions, data.Values, false)
		a.requirementForm = &requirementFormState{
			formFieldsState: fields,
			key:             key,
			pluginID:        data.PluginID,
			pluginName:      data.PluginName,
			title:           data.Title,
			message:         data.Message,
		}
	}
	if len(a.aiModels) > 0 {
		applyAIModelOptionsLocked(&a.requirementForm.formFieldsState, a.aiModels)
	}
	requestModels := hasFormDefinitionType(a.requirementForm.definitions, "selectAIModel") && !a.aiModelsLoaded && !a.aiModelsLoading
	if requestModels {
		a.aiModelsLoading = true
	}
	a.mu.Unlock()

	if requestModels {
		go a.loadAIModels()
	}
	return nil
}

// requirementFormSnapshotFor returns only the state prepared by the lifecycle coordinator.
func (a *App) requirementFormSnapshotFor(result queryResult, preview queryPreview) (*requirementFormSnapshot, error) {
	_, key, err := requirementPreviewDataAndKey(result, preview)
	if err != nil {
		return nil, err
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.requirementForm == nil || a.requirementForm.key != key {
		return nil, fmt.Errorf("requirement settings are not ready")
	}
	return snapshotRequirementFormLocked(a.requirementForm, a.aiModelsError), nil
}

func snapshotRequirementFormLocked(state *requirementFormState, modelsError string) *requirementFormSnapshot {
	if state == nil {
		return nil
	}
	return &requirementFormSnapshot{
		formFieldsSnapshot: snapshotFormFieldsLocked(&state.formFieldsState),
		key:                state.key,
		pluginID:           state.pluginID,
		pluginName:         state.pluginName,
		title:              state.title,
		message:            state.message,
		saving:             state.saving,
		error:              state.error,
		modelsError:        modelsError,
	}
}

func hasFormDefinitionType(definitions []formDefinition, definitionType string) bool {
	for _, definition := range definitions {
		if definition.Type == definitionType {
			return true
		}
	}
	return false
}

// loadAIModels shares the core model catalog between requirement and plugin setting forms.
func (a *App) loadAIModels() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var models []aiModel
	err := a.client.Post(ctx, "/ai/models", map[string]any{}, &models)
	if err == nil {
		sort.Slice(models, func(i, j int) bool {
			left := models[i].Provider + "\x00" + models[i].ProviderAlias + "\x00" + models[i].Name
			right := models[j].Provider + "\x00" + models[j].ProviderAlias + "\x00" + models[j].Name
			return left < right
		})
	}
	a.mu.Lock()
	a.aiModelsLoading = false
	a.aiModelsLoaded = true
	if err != nil {
		a.aiModelsError = err.Error()
	} else {
		a.aiModels = models
		a.aiModelsError = ""
		if a.requirementForm != nil {
			applyAIModelOptionsLocked(&a.requirementForm.formFieldsState, models)
		}
		if a.pluginForm != nil {
			applyAIModelOptionsLocked(&a.pluginForm.formFieldsState, models)
		}
		if a.tableEditor != nil && a.tableEditor.rowForm != nil {
			applyAIModelOptionsLocked(a.tableEditor.rowForm, models)
		}
		if a.chatPreview != nil && a.chatPreview.panel == "models" {
			a.chatPreview.panelSelected = 0
			for index, model := range models {
				if model == a.chatPreview.chat.Model {
					a.chatPreview.panelSelected = index
					break
				}
			}
			a.chatPreview.panelScroll = 0
			a.chatPreview.panelViewport = 0
		}
	}
	a.mu.Unlock()
	if err != nil {
		log.Printf("load AI models for requirement form: %v", err)
	}
	_ = a.window.Invalidate()
}

// applyAIModelOptionsLocked materializes model structs as the JSON strings expected by plugin settings.
func applyAIModelOptionsLocked(fields *formFieldsState, models []aiModel) {
	for index := range fields.definitions {
		definition := &fields.definitions[index]
		if definition.Type != "selectAIModel" {
			continue
		}
		options := make([]formOption, 0, len(models)+1)
		current := fields.values[definition.Value.Key]
		currentFound := current == ""
		for _, model := range models {
			encoded, err := json.Marshal(model)
			if err != nil {
				continue
			}
			value := string(encoded)
			if value == current {
				currentFound = true
			}
			options = append(options, formOption{Label: aiModelLabel(model), Value: value})
		}
		if !currentFound {
			var persisted aiModel
			label := current
			if json.Unmarshal([]byte(current), &persisted) == nil {
				label = aiModelLabel(persisted)
			}
			options = append([]formOption{{Label: label, Value: current}}, options...)
		}
		definition.Value.Options = options
	}
}

func aiModelLabel(model aiModel) string {
	provider := model.Provider
	if model.ProviderAlias != "" {
		provider = model.ProviderAlias
	}
	if provider == "" {
		return model.Name
	}
	return provider + " / " + model.Name
}

// onRequirementFormKey keeps navigation and editing inside the inline form while it owns focus.
func (a *App) onRequirementFormKey(event woxui.KeyEvent) bool {
	a.mu.RLock()
	state := a.requirementForm
	active := state != nil && state.active
	focused := -1
	fieldType := ""
	multiline := false
	if active {
		focused = state.focused
		if focused >= 0 && focused < len(state.definitions) {
			fieldType = state.definitions[focused].Type
			multiline = fieldType == "textbox" && state.definitions[focused].Value.MaxLines > 1
		}
	}
	a.mu.RUnlock()
	if !active {
		return false
	}
	if event.Key == woxui.KeyEscape {
		a.deactivateRequirementForm()
		return true
	}
	if event.Key == woxui.KeyEnter && event.Modifiers.HasPrimary() {
		a.submitRequirementForm()
		return true
	}
	textEditable := fieldType == "textbox" || fieldType == "password" || fieldType == "dirPath"
	if textEditable {
		switch event.Key {
		case woxui.KeyTab:
			delta := 1
			if event.Modifiers&woxui.KeyModifierShift != 0 {
				delta = -1
			}
			a.moveRequirementFormFocus(delta)
			return true
		case woxui.KeyArrowDown:
			if !multiline {
				a.moveRequirementFormFocus(1)
				return true
			}
		case woxui.KeyArrowUp:
			if !multiline {
				a.moveRequirementFormFocus(-1)
				return true
			}
		case woxui.KeyEnter:
			return !multiline
		}
		return false
	}
	switch event.Key {
	case woxui.KeyTab, woxui.KeyArrowDown:
		if event.Key == woxui.KeyArrowDown && multiline {
			a.editRequirementFormKey(event)
			break
		}
		delta := 1
		if event.Key == woxui.KeyTab && event.Modifiers&woxui.KeyModifierShift != 0 {
			delta = -1
		}
		a.moveRequirementFormFocus(delta)
	case woxui.KeyArrowUp:
		if multiline {
			a.editRequirementFormKey(event)
		} else {
			a.moveRequirementFormFocus(-1)
		}
	case woxui.KeyArrowLeft:
		if fieldType == "select" || fieldType == "selectAIModel" {
			a.changeRequirementFormChoice(focused, -1)
		} else {
			a.editRequirementFormKey(event)
		}
	case woxui.KeyArrowRight:
		if fieldType == "select" || fieldType == "selectAIModel" {
			a.changeRequirementFormChoice(focused, 1)
		} else {
			a.editRequirementFormKey(event)
		}
	case woxui.KeySpace, woxui.KeyEnter:
		if event.Key == woxui.KeyEnter && multiline {
			a.editRequirementFormKey(event)
		} else if fieldType == "table" {
			a.openRequirementFormTable(focused)
		} else if fieldType == "checkbox" || fieldType == "select" || fieldType == "selectAIModel" {
			a.changeRequirementFormChoice(focused, 1)
		}
	default:
		a.editRequirementFormKey(event)
	}
	return true
}

// onRequirementFormTextInput forwards committed and composing input from every native backend.
func (a *App) onRequirementFormTextInput(_ woxui.TextInputEvent) bool {
	a.mu.RLock()
	state := a.requirementForm
	active := state != nil && state.active
	a.mu.RUnlock()
	return active
}

func (a *App) editRequirementFormKey(event woxui.KeyEvent) {
	a.mu.Lock()
	if state := a.requirementForm; state != nil && state.active && state.editor != nil && state.focused >= 0 && state.focused < len(state.definitions) {
		_, changed := handleFormEditorKey(state.editor, state.definitions[state.focused], event)
		if changed {
			syncFormFieldsEditorLocked(&state.formFieldsState)
			state.error = ""
		}
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) moveRequirementFormFocus(delta int) {
	a.mu.Lock()
	state := a.requirementForm
	if state == nil || len(state.definitions) == 0 {
		a.mu.Unlock()
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	index := state.focused
	for step := 0; step < len(state.definitions); step++ {
		index = (index + delta + len(state.definitions)) % len(state.definitions)
		if formDefinitionFocusable(state.definitions[index]) {
			setFormFieldsFocusLocked(&state.formFieldsState, index)
			break
		}
	}
	textInput := state.editor != nil
	a.mu.Unlock()
	a.updateFormTextInput(textInput)
	_ = a.window.Invalidate()
}

func (a *App) focusRequirementFormField(index int) {
	a.mu.Lock()
	state := a.requirementForm
	if state == nil || index < 0 || index >= len(state.definitions) || !formDefinitionFocusable(state.definitions[index]) || state.saving {
		a.mu.Unlock()
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	setFormFieldsFocusLocked(&state.formFieldsState, index)
	state.error = ""
	textInput := state.editor != nil
	a.mu.Unlock()
	a.updateFormTextInput(textInput)
	_ = a.window.Invalidate()
}

func (a *App) changeRequirementFormChoice(index, delta int) {
	a.mu.Lock()
	state := a.requirementForm
	if state == nil || !state.active || state.saving {
		a.mu.Unlock()
		return
	}
	changeFormFieldsChoiceLocked(&state.formFieldsState, index, delta)
	state.error = ""
	a.mu.Unlock()
	a.updateFormTextInput(false)
	_ = a.window.Invalidate()
}

func (a *App) setRequirementFormText(index int, value string) {
	a.mu.Lock()
	changed := a.requirementForm != nil && !a.requirementForm.saving && setFormFieldsTextLocked(&a.requirementForm.formFieldsState, index, value)
	if changed {
		a.requirementForm.error = ""
	}
	a.mu.Unlock()
	if changed {
		_ = a.window.Invalidate()
	}
}

// deactivateRequirementForm returns IME ownership to the launcher query without losing edits.
func (a *App) deactivateRequirementForm() {
	a.mu.Lock()
	wasActive := a.requirementForm != nil && a.requirementForm.active
	if wasActive {
		syncFormFieldsEditorLocked(&a.requirementForm.formFieldsState)
		a.requirementForm.active = false
	}
	a.mu.Unlock()
	if !wasActive {
		return
	}
	a.restoreQueryTextInput()
	_ = a.window.Invalidate()
}

// validateFormFields implements the validator subset shared by core query requirements.
func validateFormFields(definitions []formDefinition, values map[string]string) string {
	for _, definition := range definitions {
		key := definition.Value.Key
		if key == "" {
			continue
		}
		value := values[key]
		for _, validator := range definition.Value.Validators {
			switch validator.Type {
			case "not_empty":
				if strings.TrimSpace(value) == "" {
					return "i18n:ui_validator_value_can_not_be_empty"
				}
			case "is_number":
				if validator.Value.IsInteger {
					if _, err := strconv.Atoi(value); err != nil {
						return "i18n:ui_validator_must_be_integer"
					}
				} else if validator.Value.IsFloat {
					if _, err := strconv.ParseFloat(value, 64); err != nil {
						return "i18n:ui_validator_must_be_number"
					}
				}
			}
		}
	}
	return ""
}

func editableFormKeys(definitions []formDefinition) []string {
	keys := make([]string, 0, len(definitions))
	seen := make(map[string]struct{})
	for _, definition := range definitions {
		if definition.Type != "textbox" && definition.Type != "dirPath" && definition.Type != "checkbox" && definition.Type != "select" && definition.Type != "selectAIModel" && definition.Type != "table" && definition.Type != "dictationModel" && definition.Type != "ocrModel" && definition.Type != "dictationHotkey" {
			continue
		}
		key := definition.Value.Key
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	return keys
}

// submitRequirementForm validates and persists the compact form before issuing a fresh query ID.
func (a *App) submitRequirementForm() {
	a.mu.Lock()
	state := a.requirementForm
	if state == nil || state.saving {
		a.mu.Unlock()
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	if validationKey := validateFormFields(state.definitions, state.values); validationKey != "" {
		formKey := state.key
		a.mu.Unlock()
		validationMessage := a.translate(validationKey)
		a.mu.Lock()
		if a.requirementForm != nil && a.requirementForm.key == formKey {
			a.requirementForm.error = validationMessage
		}
		a.mu.Unlock()
		_ = a.window.Invalidate()
		return
	}
	values := make(map[string]string, len(state.values))
	for key, value := range state.values {
		values[key] = value
	}
	keys := editableFormKeys(state.definitions)
	state.saving = true
	state.error = ""
	state.active = false
	state.revision++
	revision := state.revision
	formKey := state.key
	pluginID := state.pluginID
	a.mu.Unlock()
	a.restoreQueryTextInput()
	_ = a.window.Invalidate()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var saveErr error
		for _, key := range keys {
			if err := a.client.Post(ctx, "/setting/plugin/update", map[string]string{"PluginId": pluginID, "Key": key, "Value": values[key]}, nil); err != nil {
				saveErr = fmt.Errorf("save %s: %w", key, err)
				break
			}
		}
		a.mu.Lock()
		current := a.requirementForm != nil && a.requirementForm.key == formKey && a.requirementForm.revision == revision
		if current {
			a.requirementForm.saving = false
			if saveErr != nil {
				a.requirementForm.error = saveErr.Error()
			}
		}
		a.mu.Unlock()
		if saveErr != nil {
			log.Printf("save query requirement settings: %v", saveErr)
			_ = a.window.Invalidate()
			return
		}

		a.mu.RLock()
		query := a.query
		a.mu.RUnlock()
		query.QueryID = coreclient.NewID()
		a.setQuery(query)
		if err := a.sendCurrentQuery(); err != nil {
			log.Printf("refresh query after requirement settings: %v", err)
		}
		if err := a.applyWindowBounds(); err != nil {
			log.Printf("resize launcher after requirement settings: %v", err)
		}
	}()
}
