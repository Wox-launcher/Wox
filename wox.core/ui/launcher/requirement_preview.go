package launcher

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
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
	changed := a.requirementForm != nil && a.requirementForm.key != key
	if changed {
		a.deactivateRequirementForm()
	}

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
	if models := a.aiSettings.Models(); len(models) > 0 {
		applyAIModelOptionsLocked(&a.requirementForm.formFieldsState, models)
	}
	requestModels := hasFormDefinitionType(a.requirementForm.definitions, "selectAIModel") && !a.aiSettings.ModelsLoaded() && !a.aiSettings.ModelsLoading()
	if requestModels {
		a.aiSettings.SetModelsLoading(true)
	}

	if requestModels {
		util.Go(a.lifecycleCtx, "load AI models for requirement preview", a.loadAIModels)
	}
	return nil
}

// requirementFormSnapshotFor returns only the state prepared by the lifecycle coordinator.
func (a *App) requirementFormSnapshotFor(result queryResult, preview queryPreview) (*requirementFormSnapshot, error) {
	_, key, err := requirementPreviewDataAndKey(result, preview)
	if err != nil {
		return nil, err
	}
	if a.requirementForm == nil || a.requirementForm.key != key {
		return nil, fmt.Errorf("requirement settings are not ready")
	}
	return snapshotRequirementFormLocked(a.requirementForm, a.aiSettings.ModelsError()), nil
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
// Delegates the fetch+sort+cache to the AI settings controller and applies the App-side side
// effects (refreshing requirement/plugin/table row forms that consume selectAIModel options,
// and resetting the chat-preview model panel selection) through the onLoaded callback so the
// controller stays free of *App references.
func (a *App) loadAIModels() {
	a.aiSettings.LoadAIModels(context.Background(), a.services, a.sessionID, func(models []aiModel) {
		if models == nil {
			log.Printf("load AI models for requirement form: see controller error")
			_ = a.window.Invalidate()
			return
		}
		if a.requirementForm != nil {
			applyAIModelOptionsLocked(&a.requirementForm.formFieldsState, models)
		}
		if pluginForm := a.pluginSettings.Form(); pluginForm != nil {
			applyAIModelOptionsLocked(&pluginForm.formFieldsState, models)
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
		_ = a.window.Invalidate()
	})
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
	state := a.requirementForm
	active := state != nil && state.active
	return active
}

func (a *App) editRequirementFormKey(event woxui.KeyEvent) {
	if state := a.requirementForm; state != nil && state.active && state.editor != nil && state.focused >= 0 && state.focused < len(state.definitions) {
		_, changed := handleFormEditorKey(state.editor, state.definitions[state.focused], event)
		if changed {
			syncFormFieldsEditorLocked(&state.formFieldsState)
			state.error = ""
		}
	}
	_ = a.window.Invalidate()
}

func (a *App) moveRequirementFormFocus(delta int) {
	state := a.requirementForm
	if state == nil || len(state.definitions) == 0 {
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
	a.updateFormTextInput(textInput)
	_ = a.window.Invalidate()
}

func (a *App) focusRequirementFormField(index int) {
	state := a.requirementForm
	if state == nil || index < 0 || index >= len(state.definitions) || !formDefinitionFocusable(state.definitions[index]) || state.saving {
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	setFormFieldsFocusLocked(&state.formFieldsState, index)
	state.error = ""
	textInput := state.editor != nil
	a.updateFormTextInput(textInput)
	_ = a.window.Invalidate()
}

func (a *App) changeRequirementFormChoice(index, delta int) {
	state := a.requirementForm
	if state == nil || !state.active || state.saving {
		return
	}
	changeFormFieldsChoiceLocked(&state.formFieldsState, index, delta)
	state.error = ""
	a.updateFormTextInput(false)
	_ = a.window.Invalidate()
}

func (a *App) setRequirementFormText(index int, value string) {
	changed := a.requirementForm != nil && !a.requirementForm.saving && setFormFieldsTextLocked(&a.requirementForm.formFieldsState, index, value)
	if changed {
		a.requirementForm.error = ""
	}
	if changed {
		_ = a.window.Invalidate()
	}
}

// deactivateRequirementForm returns IME ownership to the launcher query without losing edits.
func (a *App) deactivateRequirementForm() {
	wasActive := a.requirementForm != nil && a.requirementForm.active
	if wasActive {
		syncFormFieldsEditorLocked(&a.requirementForm.formFieldsState)
		a.requirementForm.active = false
	}
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
	state := a.requirementForm
	if state == nil || state.saving {
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	if validationKey := validateFormFields(state.definitions, state.values); validationKey != "" {
		formKey := state.key
		validationMessage := a.translate(validationKey)
		if a.requirementForm != nil && a.requirementForm.key == formKey {
			a.requirementForm.error = validationMessage
		}
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
	a.restoreQueryTextInput()
	_ = a.window.Invalidate()

	util.Go(a.lifecycleCtx, "save query requirement settings", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		updates := make(map[string]string, len(keys))
		for _, key := range keys {
			updates[key] = values[key]
		}
		saveErr := a.services.UpdatePluginSettings(ctx, a.sessionID, pluginID, updates)
		refreshQuery := false
		if dispatchErr := a.runOnUI("apply query requirement settings", func() {
			current := a.requirementForm != nil && a.requirementForm.key == formKey && a.requirementForm.revision == revision
			if current {
				a.requirementForm.saving = false
				if saveErr != nil {
					a.requirementForm.error = saveErr.Error()
				}
			}
			if saveErr == nil {
				query := a.query
				query.QueryID = newID()
				a.setQuery(query)
				if err := a.applyWindowBounds(); err != nil {
					log.Printf("resize launcher after requirement settings: %v", err)
				}
				refreshQuery = true
				return
			}
			_ = a.window.Invalidate()
		}); dispatchErr != nil {
			log.Printf("dispatch query requirement settings: %v", dispatchErr)
			return
		}
		if saveErr != nil {
			log.Printf("save query requirement settings: %v", saveErr)
			return
		}
		if refreshQuery {
			if err := a.sendCurrentQuery(); err != nil {
				log.Printf("refresh query after requirement settings: %v", err)
			}
		}
	})
}
