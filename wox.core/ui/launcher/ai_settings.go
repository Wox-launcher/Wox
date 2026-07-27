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
	"wox/util"
)

type aiProviderInfo struct {
	Name        string
	DefaultHost string
}

// buildAISettingsPage converts core-backed table values into the pure settings view.
func (a *App) buildAISettingsPage(snapshot settingsSnapshot, width, height, imageScale float32) woxwidget.Widget {
	aiForm := snapshot.ai.Form
	props := launcherview.AISettingsProps{
		Width: width, Height: height, Theme: snapshot.palette.componentTheme(), Available: aiForm != nil,
		Title: a.translate("i18n:ui_ai"), Description: a.translate("i18n:ui_ai_description"),
	}
	if aiForm == nil {
		return launcherview.AISettingsView(props)
	}
	props.Selected = -1
	if aiForm.active {
		props.Selected = aiForm.focused
	}
	callbacks := formFieldCallbacks{idPrefix: "ai-settings", imageScale: imageScale, focus: a.selectAISettingsTable, openTable: a.openAISettingsTable}
	contentWidth := launcherview.SettingsPageContentWidth(width)
	props.Tables = make([]launcherview.AISettingsTable, 0, len(aiForm.definitions))
	for index, definition := range aiForm.definitions {
		index := index
		tableHeight := formDefinitionHeight(definition, aiForm.values)
		field := a.formTableFieldProps(*aiForm, callbacks, snapshot.palette, index, definition, contentWidth, tableHeight)
		field.OnAdd = func() { a.addAISettingsTableRow(index) }
		if definition.Value.Key == "AISkills" {
			field.HideEditAction = true
			field.HideCloneAction = true
			a.addAISkillTableActions(&field, aiForm.values[definition.Value.Key], imageScale)
		}
		props.Tables = append(props.Tables, launcherview.AISettingsTable{
			Index: index,
			Field: field,
		})
	}
	props.Note = snapshot.note
	if snapshot.ai.ProvidersLoading {
		props.Note = "Loading the provider catalog…"
	} else if snapshot.ai.ProvidersError != "" {
		props.Note = snapshot.ai.ProvidersError
	}
	return launcherview.AISettingsView(props)
}

// addAISkillTableActions adds Flutter's folder action after the standard delete action.
func (a *App) addAISkillTableActions(field *launcherview.FormTableFieldProps, value string, imageScale float32) {
	rows, err := decodeFormTableRows(value)
	if err != nil {
		return
	}
	iconTint := field.Theme.ResultSubtitle
	folderIcon := a.imageForTint(settingControlIconSource("folder-open"), &iconTint, physicalImageSize(16, imageScale))
	for viewIndex := range field.Rows {
		sourceIndex := field.Rows[viewIndex].Index
		if sourceIndex < 0 || sourceIndex >= len(rows) {
			continue
		}
		skillPath := strings.TrimSpace(fmt.Sprint(rows[sourceIndex]["Path"]))
		if skillPath == "" {
			continue
		}
		field.Rows[viewIndex].TrailingActions = append(field.Rows[viewIndex].TrailingActions, launcherview.FormTableRowAction{
			ID: "open-folder", Label: a.translate("i18n:plugin_file_open"), Icon: folderIcon,
			OnTap: func() { a.openAISkillPath(skillPath) },
		})
	}
}

// openAISkillPath delegates folder opening to the same cross-platform service used by Flutter.
func (a *App) openAISkillPath(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	util.Go(a.lifecycleCtx, "open AI skill path", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		err := a.services.OpenPath(ctx, a.sessionID, path)
		cancel()
		if err != nil {
			_ = a.runOnUI("apply AI skill path error", func() {
				a.settingNote = "Could not open skill path: " + err.Error()
				a.invalidateSettingsWindow()
			})
		}
	})
}

// newAISettingsForm maps the core settings arrays onto the shared portable table editor.
func newAISettingsForm(data settingsData) formFieldsState {
	definitions := []formDefinition{
		{
			Type: "table",
			Value: formDefinitionValue{
				Key: "AIProviders", Title: "i18n:ui_ai_model", SortColumnKey: "Name", InlineTable: true,
				Columns: []formTableColumn{
					{Key: "Status", Label: "i18n:ui_ai_providers_status", Width: 40, Type: "aiModelStatus", HideInUpdate: true},
					{Key: "Name", Label: "i18n:ui_ai_providers_name", Tooltip: "i18n:ui_ai_providers_name_tooltip", Width: 100, Type: "select", Validators: []formValidator{{Type: "not_empty"}}},
					{Key: "Alias", Label: "i18n:ui_ai_providers_alias", Tooltip: "i18n:ui_ai_providers_alias_tooltip", Width: 120, Type: "text"},
					{Key: "Host", Label: "i18n:ui_ai_providers_host", Tooltip: "i18n:ui_ai_providers_host_tooltip", Width: 160, Type: "text"},
					{Key: "ApiKey", Label: "i18n:ui_ai_providers_api_key", Tooltip: "i18n:ui_ai_providers_api_key_tooltip", Type: "text"},
				},
			},
		},
		{
			Type: "table",
			Value: formDefinitionValue{
				Key: "AIMCPServers", Title: "i18n:ui_ai_mcp_servers", Tooltip: "i18n:ui_ai_mcp_servers_tooltip", SortColumnKey: "Name", InlineTable: true,
				Columns: []formTableColumn{
					{Key: "Name", Label: "i18n:plugin_ai_chat_mcp_server_name", Tooltip: "i18n:plugin_ai_chat_mcp_server_name_tooltip", Width: 100, Type: "text", Validators: []formValidator{{Type: "not_empty"}}},
					{Key: "Tools", Label: "i18n:plugin_ai_chat_mcp_server_tools", Tooltip: "i18n:plugin_ai_chat_mcp_server_tools_tooltip", Width: 50, Type: "aiMCPServerTools", HideInUpdate: true},
					{Key: "Disabled", Label: "i18n:plugin_ai_chat_mcp_server_disabled", Width: 80, Type: "checkbox"},
					{Key: "Type", Label: "i18n:plugin_ai_chat_mcp_server_type", Tooltip: "i18n:plugin_ai_chat_mcp_server_type_tooltip", Width: 80, Type: "select", SelectOptions: []formOption{{Label: "STDIO", Value: "stdio"}, {Label: "Streamable HTTP", Value: "streamable-http"}}, Validators: []formValidator{{Type: "not_empty"}}},
					{Key: "Command", Label: "i18n:plugin_ai_chat_mcp_server_command", Tooltip: "i18n:plugin_ai_chat_mcp_server_command_tooltip", Width: 100, Type: "text"},
					{Key: "EnvironmentVariables", Label: "i18n:plugin_ai_chat_mcp_server_environment_variables", Tooltip: "i18n:plugin_ai_chat_mcp_server_environment_variables_tooltip", Width: 160, Type: "textList", TextMaxLines: 6},
					{Key: "Url", Label: "i18n:plugin_ai_chat_mcp_server_url", Tooltip: "i18n:plugin_ai_chat_mcp_server_url_tooltip", Width: 120, Type: "text", TextMaxLines: 10},
				},
			},
		},
		{
			Type: "table",
			Value: formDefinitionValue{
				Key: "AISkills", Title: "i18n:ui_ai_skills", Tooltip: "i18n:ui_ai_skills_tooltip", SortColumnKey: "Name", MaxHeight: 360, InlineTable: true,
				Columns: []formTableColumn{
					{Key: "Name", Label: "i18n:plugin_ai_chat_skill_name", Width: 200, Type: "text", HideInUpdate: true},
					{Key: "Source", Label: "i18n:plugin_ai_chat_skill_type", Width: 100, Type: "aiSkillSource", HideInUpdate: true},
					{Key: "Description", Label: "i18n:plugin_ai_chat_skill_description", Width: 400, Type: "text", HideInUpdate: true},
					{Key: "SourceUrl", Label: "i18n:plugin_ai_chat_skill_source_url", Width: 200, Type: "text", HideInUpdate: true, HideInTable: true},
					{Key: "SourceName", Type: "text", HideInUpdate: true, HideInTable: true},
					{Key: "ManifestPath", Type: "text", HideInUpdate: true, HideInTable: true},
					{Key: "Enabled", Type: "checkbox", HideInUpdate: true, HideInTable: true},
					{Key: "Error", Type: "text", HideInUpdate: true, HideInTable: true},
					{Key: "Path", Label: "i18n:ui_ai_skill_add_path", Width: 400, Type: "dirPath", HideInTable: true, Validators: []formValidator{{Type: "not_empty"}}},
				},
			},
		},
	}
	values := map[string]string{
		"AIProviders":  settingsJSONArray(data.AIProviders),
		"AIMCPServers": settingsJSONArray(data.AIMCPServers),
		"AISkills":     settingsJSONArray(data.AISkills),
	}
	return newFormFieldsState(definitions, values, true)
}

func settingsJSONArray(value json.RawMessage) string {
	trimmed := strings.TrimSpace(string(value))
	if trimmed == "" || trimmed == "null" {
		return "[]"
	}
	return trimmed
}

// loadAIProviderCatalog hydrates provider choices without coupling the widget package to core types.
// Delegates the fetch+cache to the AI settings controller and applies the App-side side effects
// (refreshing the AI settings form dropdown and any open AIProviders row editor) through the
// onLoaded callback so the controller stays free of *App references.
func (a *App) loadAIProviderCatalog() {
	a.aiSettings.ReloadProviders(context.Background(), a.services, a.sessionID, func(providers []aiProviderInfo) {
		if form := a.aiSettings.Form(); form != nil {
			applyAIProviderCatalogLocked(form, providers)
		}
		if state := a.tableEditor; state != nil && state.target == a.aiSettings.Form() && state.definition.Value.Key == "AIProviders" {
			state.definition = state.target.definitions[state.fieldIndex]
			applyAIProviderOptionsToRowFormLocked(state.rowForm, state.definition)
			applyAIProviderDefaultHostLocked(state, false, providers)
		}
	})
}

// applyAIProviderCatalogLocked merges live provider names with configured names that core may no longer advertise.
func applyAIProviderCatalogLocked(fields *formFieldsState, providers []aiProviderInfo) {
	if fields == nil {
		return
	}
	options := make([]formOption, 0, len(providers))
	seen := make(map[string]bool, len(providers))
	for _, provider := range providers {
		name := strings.TrimSpace(provider.Name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		options = append(options, formOption{Label: name, Value: name})
	}
	if rows, err := decodeFormTableRows(fields.values["AIProviders"]); err == nil {
		for _, row := range rows {
			name := strings.TrimSpace(fmt.Sprint(row["Name"]))
			if name != "" && !seen[name] {
				seen[name] = true
				options = append(options, formOption{Label: name, Value: name})
			}
		}
	}
	for definitionIndex := range fields.definitions {
		definition := &fields.definitions[definitionIndex]
		if definition.Type != "table" || definition.Value.Key != "AIProviders" {
			continue
		}
		for columnIndex := range definition.Value.Columns {
			column := &definition.Value.Columns[columnIndex]
			if column.Key == "Name" {
				column.SelectOptions = append([]formOption(nil), options...)
			}
		}
	}
}

// applyAIProviderOptionsToRowFormLocked refreshes a row editor that opened before the provider request completed.
func applyAIProviderOptionsToRowFormLocked(fields *formFieldsState, definition formDefinition) {
	if fields == nil {
		return
	}
	var options []formOption
	for _, column := range definition.Value.Columns {
		if column.Key == "Name" {
			options = column.SelectOptions
			break
		}
	}
	for index := range fields.definitions {
		if fields.definitions[index].Value.Key == "Name" {
			fields.definitions[index].Value.Options = append([]formOption(nil), options...)
		}
	}
}

// applyAIProviderDefaultHostLocked mirrors the provider-to-default-host mapping used by the UI settings form.
func applyAIProviderDefaultHostLocked(state *formTableEditorState, overwrite bool, providers []aiProviderInfo) {
	if state == nil || state.definition.Value.Key != "AIProviders" || state.rowForm == nil {
		return
	}
	if !overwrite && strings.TrimSpace(state.rowForm.values["Host"]) != "" {
		return
	}
	name := state.rowForm.values["Name"]
	for _, provider := range providers {
		if provider.Name == name {
			state.rowForm.values["Host"] = provider.DefaultHost
			return
		}
	}
}

// onAISettingsKey keeps table selection portable while the modal editor owns row-level input.
func (a *App) onAISettingsKey(event woxui.KeyEvent) bool {
	active := a.settingsOpen && a.settingTab == "ai" && a.aiSettings.Form() != nil && a.tableEditor == nil
	if !active {
		return false
	}
	switch event.Key {
	case woxui.KeyArrowUp:
		a.moveAISettingsTable(-1)
	case woxui.KeyArrowDown:
		a.moveAISettingsTable(1)
	case woxui.KeyEnter, woxui.KeySpace, woxui.KeyArrowRight:
		a.openSelectedAISettingsTable()
	default:
		return false
	}
	return true
}

// selectAISettingsTable moves keyboard focus between the three table cards.
func (a *App) selectAISettingsTable(index int) {
	if form := a.aiSettings.Form(); form != nil && index >= 0 && index < len(form.definitions) {
		a.settingRow = index
		setFormFieldsFocusLocked(form, index)
	}
	a.invalidateSettingsWindow()
}

// moveAISettingsTable wraps table-card selection without entering the modal editor.
func (a *App) moveAISettingsTable(delta int) {
	if form := a.aiSettings.Form(); form != nil && len(form.definitions) > 0 {
		a.settingRow = (a.settingRow + delta + len(form.definitions)) % len(form.definitions)
		setFormFieldsFocusLocked(form, a.settingRow)
	}
	a.invalidateSettingsWindow()
}

func (a *App) openSelectedAISettingsTable() {
	index := a.settingRow
	a.openAISettingsTable(index)
}

// openAISettingsTable opens a settings-owned target in the same modal table editor used by plugin forms.
func (a *App) openAISettingsTable(index int) {
	form := a.aiSettings.Form()
	if a.settingsOpen && a.settingTab == "ai" && form != nil {
		a.settingRow = index
		a.openFormTableLocked(form, index)
	}
	a.finishOpeningFormTable()
}

// addAISettingsTableRow opens the shared editor directly at its create flow while preserving the skills source chooser.
func (a *App) addAISettingsTableRow(index int) {
	a.openAISettingsTable(index)
	if index < 2 {
		a.beginAddFormTableRowDirect()
	}
}

// openAISettingsTableRow carries the inline row selection into the shared table editor.
func (a *App) openAISettingsTableRow(tableIndex, rowIndex int) {
	form := a.aiSettings.Form()
	if a.settingsOpen && a.settingTab == "ai" && form != nil {
		a.settingRow = tableIndex
		a.openFormTableLocked(form, tableIndex)
		if a.tableEditor != nil && rowIndex >= 0 && rowIndex < len(a.tableEditor.rows) {
			a.tableEditor.selected = rowIndex
		}
	}
	a.finishOpeningFormTable()
	if tableIndex < 2 {
		a.beginEditFormTableRowDirect()
	}
}

// beginCloneRemoteAISkill reuses the row form surface for the one URL needed by core's clone operation.
func (a *App) beginCloneRemoteAISkill() {
	state := a.tableEditor
	if state == nil || state.definition.Value.Key != "AISkills" || state.invalid || state.saving || state.rowForm != nil || state.target != a.aiSettings.Form() {
		return
	}
	fields := newFormFieldsState([]formDefinition{{
		Type: "textbox",
		Value: formDefinitionValue{
			Key: "SourceUrl", Label: "Repository URL", MaxLines: 1,
			Validators: []formValidator{{Type: "not_empty"}},
		},
	}}, nil, true)
	state.rowForm = &fields
	state.rowIndex = -1
	state.rowBase = nil
	state.skillClone = true
	state.status = ""
	state.deletePending = -1
	state.deleteDirect = false
	a.updateSettingsTextInput(true)
	a.invalidateSettingsWindow()
}

// cloneRemoteAISkills discovers repository skills, appends them atomically, then saves the combined setting.
func (a *App) cloneRemoteAISkills(state *formTableEditorState, url, previousValue string) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	loaded, err := a.services.CloneAISkills(ctx, a.sessionID, url)
	cancel()
	skills := make([]map[string]any, len(loaded))
	for index, skill := range loaded {
		skills[index] = map[string]any{
			"Path": skill.Path, "ManifestPath": skill.ManifestPath, "Name": skill.Name, "Description": skill.Description,
			"Error": skill.Error, "Source": skill.Source, "SourceName": skill.SourceName, "SourceUrl": skill.SourceURL, "Enabled": skill.Enabled,
		}
	}
	if err == nil && len(skills) == 0 {
		err = fmt.Errorf("the repository did not contain any skills")
	}

	var value string
	save := false
	_ = a.runOnUI("apply cloned AI skills", func() {
		if err != nil {
			a.settingSaving = false
			if a.tableEditor == state {
				state.saving = false
				state.status = "Could not clone: " + err.Error()
			}
			a.settingNote = "Could not clone remote skills: " + err.Error()
			a.invalidateSettingsWindow()
			return
		}
		state.rows = append(state.rows, cloneFormTableRows(skills)...)
		state.selected = len(state.rows) - 1
		if commitErr := a.commitFormTableRowsLocked(state); commitErr != nil {
			a.settingSaving = false
			state.rows, _ = decodeFormTableRows(previousValue)
			state.target.values[state.definition.Value.Key] = previousValue
			if a.tableEditor == state {
				state.saving = false
				state.status = commitErr.Error()
			}
			a.invalidateSettingsWindow()
			return
		}
		value = state.target.values[state.definition.Value.Key]
		if a.tableEditor == state {
			state.status = "Saving cloned skills…"
		}
		save = true
	})
	if save {
		a.saveSettingsTable(state, "AISkills", value, previousValue)
	}
}

// validateAISettingsTableRow enforces the transport-specific requirements that the generic schema cannot express.
func validateAISettingsTableRow(definition formDefinition, fields *formFieldsState) string {
	switch definition.Value.Key {
	case "AIProviders":
		if fields.values["Name"] != "ollama" && strings.TrimSpace(fields.values["ApiKey"]) == "" {
			return "API key is required for this provider."
		}
	case "AIMCPServers":
		switch fields.values["Type"] {
		case "stdio":
			if strings.TrimSpace(fields.values["Command"]) == "" {
				return "Command is required for a STDIO server."
			}
		case "streamable-http":
			if strings.TrimSpace(fields.values["Url"]) == "" {
				return "URL is required for a Streamable HTTP server."
			}
		}
	}
	return ""
}

func aiSettingsTableLabel(key string) string {
	switch key {
	case "AIProviders":
		return "AI providers"
	case "AIMCPServers":
		return "MCP servers"
	case "AISkills":
		return "Skills"
	default:
		return key
	}
}

// saveSettingsTable persists one settings-owned table and rolls the editor back if core rejects it.
func (a *App) saveSettingsTable(state *formTableEditorState, key, value, previousValue string) {
	coreValue := value
	if key == "IgnoredHotkeyApps" {
		var err error
		coreValue, err = settingsIgnoredHotkeyAppsCoreJSON(value)
		if err != nil {
			_ = a.runOnUI("apply invalid settings table value", func() {
				a.settingSaving = false
				state.target.values[key] = previousValue
				if a.tableEditor == state {
					state.saving = false
					state.status = "Could not save: " + err.Error()
				}
				a.settingNote = "Could not save " + settingsTableLabel(key) + ": " + err.Error()
				a.invalidateSettingsWindow()
			})
			return
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err := a.services.UpdateGeneralSetting(ctx, a.sessionID, key, coreValue)
	cancel()

	_ = a.runOnUI("apply settings table save", func() {
		a.settingSaving = false
		if err != nil {
			state.target.values[key] = previousValue
			if a.tableEditor == state {
				if rows, decodeErr := decodeFormTableRows(previousValue); decodeErr == nil {
					state.rows = rows
					state.selected = min(state.selected, len(rows)-1)
				}
				state.saving = false
				state.status = "Could not save: " + err.Error()
			}
			a.settingNote = "Could not save " + settingsTableLabel(key) + ": " + err.Error()
		} else {
			if state.target == a.aiSettings.Form() {
				a.applyAISettingsRawLocked(key, value)
			} else if state.target == a.hotkeySettings.Form() {
				a.applyHotkeySettingsRawLocked(key, coreValue)
			}
			if a.tableEditor == state {
				state.saving = false
				state.status = "Saved"
			}
			a.settingNote = settingsTableLabel(key) + " saved"
		}
		a.invalidateSettingsWindow()
	})
}

func settingsTableLabel(key string) string {
	if label := hotkeySettingsLabel(key); label != key {
		return label
	}
	return aiSettingsTableLabel(key)
}

// applyAISettingsRawLocked keeps the settings snapshot and dependent chat catalogs coherent after a save.
func (a *App) applyAISettingsRawLocked(key, value string) {
	raw := json.RawMessage(append([]byte(nil), value...))
	switch key {
	case "AIProviders":
		a.generalSettings.Update(func(d *settingsData) { d.AIProviders = raw })
		a.aiSettings.ResetModels()
	case "AIMCPServers":
		a.generalSettings.Update(func(d *settingsData) { d.AIMCPServers = raw })
	case "AISkills":
		a.generalSettings.Update(func(d *settingsData) { d.AISkills = raw })
		a.aiSettings.ResetSkills()
	}
}

// formTableSkillRowReadOnly prevents built-in and discovered read-only skills from being removed locally.
func formTableSkillRowReadOnly(definition formDefinition, row map[string]any) bool {
	if definition.Value.Key != "AISkills" {
		return false
	}
	readOnly, _ := row["ReadOnly"].(bool)
	builtin, _ := row["Builtin"].(bool)
	return readOnly || builtin
}
