package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

type aiProviderInfo struct {
	Name        string
	Icon        woxImage
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
		field := a.formTableFieldProps(*aiForm, callbacks, snapshot.palette, index, definition, contentWidth, 0)
		field.OnAdd = func() { a.addAISettingsTableRow(index) }
		if definition.Value.Key == "AISkills" {
			field.HideEditAction = true
			field.HideCloneAction = true
			a.addAISkillTableActions(&field, aiForm.values[definition.Value.Key], imageScale)
		}
		props.Tables = append(props.Tables, launcherview.AISettingsTable{
			Index: index, Field: field, Highlighted: snapshot.highlight == "built-in:"+definition.Value.Key,
		})
	}
	if snapshot.ai.ProvidersError != "" {
		props.Error = snapshot.ai.ProvidersError
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
			log.Printf("open AI skill path: %v", err)
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
		if state := a.settingsTableEditor; state != nil && state.target == a.aiSettings.Form() && state.definition.Value.Key == "AIProviders" {
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
		options = append(options, formOption{Label: name, Value: name, Icon: provider.Icon})
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
	active := a.settingsOpen && a.settingTab == "ai" && a.aiSettings.Form() != nil && a.settingsTableEditor == nil
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

// addAISettingsTableRow opens the shared editor directly at its create flow. The
// skills table uses Flutter's tabbed add dialog instead of the generic row editor.
func (a *App) addAISettingsTableRow(index int) {
	a.openAISettingsTable(index)
	if index < 2 {
		a.beginAddFormTableRowDirect()
		return
	}
	a.openFormTableSkillAdd()
}

// openAISettingsTableRow carries the inline row selection into the shared table editor.
func (a *App) openAISettingsTableRow(tableIndex, rowIndex int) {
	form := a.aiSettings.Form()
	if a.settingsOpen && a.settingTab == "ai" && form != nil {
		a.settingRow = tableIndex
		a.openFormTableLocked(form, tableIndex)
		if a.settingsTableEditor != nil && rowIndex >= 0 && rowIndex < len(a.settingsTableEditor.rows) {
			a.settingsTableEditor.selected = rowIndex
		}
	}
	a.finishOpeningFormTable()
	if tableIndex < 2 {
		a.beginEditFormTableRowDirect()
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
				if a.settingsTableEditor == state {
					state.saving = false
					state.status = "Could not save: " + err.Error()
				}
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
			if a.settingsTableEditor == state {
				if rows, decodeErr := decodeFormTableRows(previousValue); decodeErr == nil {
					state.rows = rows
					state.selected = min(state.selected, len(rows)-1)
				}
				state.saving = false
				state.status = "Could not save: " + err.Error()
			}
		} else {
			if state.target == a.aiSettings.Form() {
				a.applyAISettingsRawLocked(key, value)
			} else if state.target == a.hotkeySettings.Form() {
				a.applyHotkeySettingsRawLocked(key, coreValue)
			}
			if a.settingsTableEditor == state {
				state.saving = false
				state.status = ""
			}
		}
		a.invalidateSettingsWindow()
	})
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

// formTableSkillRowReadOnly protects only built-in skills from removal.
// User-discovered skills are deletable, matching the Flutter skills table which
// allows deleting any row; the blanket ReadOnly flag set by skill discovery is
// not a local removal restriction.
func formTableSkillRowReadOnly(definition formDefinition, row map[string]any) bool {
	if definition.Value.Key != "AISkills" {
		return false
	}
	builtin, _ := row["Builtin"].(bool)
	return builtin
}
