package launcher

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"wox/ui/contract"
	"wox/util"
)

const aiCommandPluginID = "c9910664-1c28-47ae-bad6-e7332a02d471"

// selectedPluginID returns the catalog identity behind the current detail pane.
func (a *App) selectedPluginID() string {
	plugins := a.pluginSettings.Plugins()
	selected := a.pluginSettings.Selected()
	if selected < 0 || selected >= len(plugins) {
		return ""
	}
	return plugins[selected].ID
}

// openAICommandTemplatePicker loads the shared catalog before presenting the
// same searchable choice surface used elsewhere in settings.
func (a *App) openAICommandTemplatePicker(fieldIndex int) {
	service, ok := a.services.(contract.AICommandTemplateServices)
	form := a.pluginSettings.Form()
	if !ok || form == nil || form.pluginID != aiCommandPluginID || fieldIndex < 0 || fieldIndex >= len(form.definitions) || form.definitions[fieldIndex].Value.Key != "commands" {
		return
	}
	form.status = a.translate("i18n:ui_cloud_sync_plugin_exclusions_loading")
	form.statusError = false
	a.invalidateSettingsWindow()
	util.Go(a.lifecycleCtx, "load AI command templates for plugin settings", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		templates, err := service.AICommandTemplates(ctx, a.sessionID)
		defaultModel, _ := service.DefaultAIModel(ctx, a.sessionID)
		cancel()
		_ = a.runOnUI("show AI command template picker", func() {
			current := a.pluginSettings.Form()
			if current == nil || current.pluginID != aiCommandPluginID {
				return
			}
			if err != nil {
				current.status = strings.ReplaceAll(a.translate("i18n:ui_ai_command_template_load_failed"), "{error}", err.Error())
				current.statusError = true
				a.invalidateSettingsWindow()
				return
			}
			if len(templates) == 0 {
				current.status = a.translate("i18n:ui_ai_command_template_empty")
				current.statusError = false
				a.invalidateSettingsWindow()
				return
			}
			a.showAICommandTemplateChoices(fieldIndex, templates, defaultModel)
		})
	})
}

// showAICommandTemplateChoices keeps template data behind stable IDs while the
// shared picker owns filtering and keyboard selection.
func (a *App) showAICommandTemplateChoices(fieldIndex int, templates []contract.AICommandTemplate, defaultModel contract.AIModel) {
	choices := make([]settingChoice, 0, len(templates))
	trailers := make(map[string]string, len(templates))
	templatesByID := make(map[string]contract.AICommandTemplate, len(templates))
	for _, template := range templates {
		if strings.TrimSpace(template.ID) == "" {
			continue
		}
		label := strings.TrimSpace(template.Name)
		if label == "" {
			label = template.Command
		}
		choices = append(choices, settingChoice{value: template.ID, label: label})
		trailers[template.ID] = template.Category
		templatesByID[template.ID] = template
	}
	if len(choices) == 0 {
		if form := a.pluginSettings.Form(); form != nil {
			form.status = a.translate("i18n:ui_ai_command_template_empty")
		}
		a.invalidateSettingsWindow()
		return
	}
	if form := a.pluginSettings.Form(); form != nil {
		form.status = ""
		form.statusError = false
	}
	a.generalSettings.SetChoicePicker(&settingChoicePickerState{
		item: settingItem{
			key: "AICommandTemplate", title: a.translate("i18n:ui_ai_command_template_store"), filterable: true,
			choices: choices, trailers: trailers,
		},
		onChoose: func(choice settingChoice) {
			template, exists := templatesByID[choice.value]
			if exists {
				a.beginAICommandTemplateRow(fieldIndex, template, defaultModel)
			}
		},
	})
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
}

// beginAICommandTemplateRow opens the normal row editor with template values so
// users can review the model and command before committing the plugin setting.
func (a *App) beginAICommandTemplateRow(fieldIndex int, template contract.AICommandTemplate, defaultModel contract.AIModel) {
	form := a.pluginSettings.Form()
	if form == nil || form.pluginID != aiCommandPluginID {
		return
	}
	a.openPluginFormTable(fieldIndex)
	a.beginAddFormTableRowDirect()
	state := a.tableEditor
	if state == nil || state.rowForm == nil {
		return
	}
	values := map[string]string{
		"name": template.Name, "command": template.Command, "prompt": template.Prompt,
		"thinkingMode": template.ThinkingMode, "defaultAction": template.DefaultAction,
	}
	if template.Vision {
		values["vision"] = "true"
	} else {
		values["vision"] = "false"
	}
	if defaultModel.Name != "" && defaultModel.Provider != "" {
		if encoded, err := json.Marshal(aiModel{Name: defaultModel.Name, Provider: defaultModel.Provider, ProviderAlias: defaultModel.ProviderAlias}); err == nil {
			values["model"] = string(encoded)
		}
	}
	for index, definition := range state.rowForm.definitions {
		value, exists := values[definition.Value.Key]
		if !exists {
			continue
		}
		if formDefinitionTextEditable(definition) {
			setFormFieldsTextLocked(state.rowForm, index, value)
		} else {
			state.rowForm.values[definition.Value.Key] = value
		}
	}
	state.status = ""
	a.updateFormTableTextInput(state.rowForm.editor != nil)
	a.invalidateFormTableWindow()
}
