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

type settingsInlineColumn struct {
	key    string
	label  string
	weight float32
}

type aiProviderInfo struct {
	Name        string
	DefaultHost string
}

// buildAISettingsPage converts core-backed table values into the pure settings view.
func (a *App) buildAISettingsPage(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	props := launcherview.AISettingsProps{
		Width: width, Height: height, Theme: snapshot.palette.componentTheme(), Available: snapshot.aiForm != nil,
		Title: a.translate("i18n:ui_ai"), Description: a.translate("i18n:ui_ai_description"),
		AddLabel: a.translate("i18n:ui_add"), NoDataLabel: a.translate("i18n:ui_no_data"), Scroll: snapshot.pageScroll.offset,
		OnScroll: a.scrollSettingsPage, OnSetGeometry: func(viewport, content, _ float32) { a.setSettingsPageGeometry(viewport, content) },
	}
	if snapshot.aiForm == nil {
		return launcherview.AISettingsView(props)
	}
	props.Tables = make([]launcherview.AISettingsTable, 0, len(snapshot.aiForm.definitions))
	for index, definition := range snapshot.aiForm.definitions {
		index := index
		definition := definition
		rows, err := decodeFormTableRows(snapshot.aiForm.values[definition.Value.Key])
		if err != nil {
			rows = nil
		}
		columns, description, maxRows := a.aiInlineTableColumns(definition.Value.Key)
		visibleRows := min(len(rows), maxRows)
		convertedColumns := make([]launcherview.AISettingsColumn, len(columns))
		for columnIndex, column := range columns {
			convertedColumns[columnIndex] = launcherview.AISettingsColumn{Label: a.translate(column.label), Weight: column.weight}
		}
		convertedRows := make([][]launcherview.AISettingsCell, 0, visibleRows)
		for _, row := range rows[:visibleRows] {
			cells := make([]launcherview.AISettingsCell, len(columns))
			for columnIndex, column := range columns {
				kind := launcherview.AISettingsCellText
				if column.key == "Status" {
					kind = launcherview.AISettingsCellStatus
				} else if column.key == "_action" {
					kind = launcherview.AISettingsCellAction
				}
				cells[columnIndex] = launcherview.AISettingsCell{Text: a.inlineTableCellValue(definition, column.key, row), Kind: kind}
			}
			convertedRows = append(convertedRows, cells)
		}
		props.Tables = append(props.Tables, launcherview.AISettingsTable{
			Index: index, Title: a.translate(formTableTitle(definition)), Description: description,
			Columns: convertedColumns, Rows: convertedRows,
			OnAdd:     func() { a.addAISettingsTableRow(index) },
			OnOpenRow: func(row int) { a.openAISettingsTableRow(index, row) },
		})
	}
	props.Note = snapshot.note
	if snapshot.aiProvidersLoading {
		props.Note = "Loading the provider catalog…"
	} else if snapshot.aiProvidersError != "" {
		props.Note = snapshot.aiProvidersError
	}
	return launcherview.AISettingsView(props)
}

func (a *App) aiInlineTableColumns(key string) ([]settingsInlineColumn, string, int) {
	switch key {
	case "AIProviders":
		return []settingsInlineColumn{
			{key: "Status", label: "i18n:ui_ai_providers_status", weight: 0.06},
			{key: "Name", label: "i18n:ui_ai_providers_name", weight: 0.15},
			{key: "Alias", label: "i18n:ui_ai_providers_alias", weight: 0.17},
			{key: "Host", label: "i18n:ui_ai_providers_host", weight: 0.23},
			{key: "ApiKey", label: "i18n:ui_ai_providers_api_key", weight: 0.27},
			{key: "_action", label: "i18n:ui_operation", weight: 0.12},
		}, "", 4
	case "AIMCPServers":
		return []settingsInlineColumn{
			{key: "Name", label: "i18n:plugin_ai_chat_mcp_server_name", weight: 0.15},
			{key: "Tools", label: "i18n:plugin_ai_chat_mcp_server_tools", weight: 0.09},
			{key: "Disabled", label: "i18n:plugin_ai_chat_mcp_server_disabled", weight: 0.10},
			{key: "Type", label: "i18n:plugin_ai_chat_mcp_server_type", weight: 0.13},
			{key: "Command", label: "i18n:plugin_ai_chat_mcp_server_command", weight: 0.15},
			{key: "EnvironmentVariables", label: "i18n:plugin_ai_chat_mcp_server_environment_variables", weight: 0.19},
			{key: "Url", label: "i18n:plugin_ai_chat_mcp_server_url", weight: 0.19},
		}, a.translate("i18n:ui_ai_mcp_servers_tooltip"), 3
	default:
		return []settingsInlineColumn{
			{key: "Name", label: "i18n:plugin_ai_chat_skill_name", weight: 0.26},
			{key: "Source", label: "i18n:plugin_ai_chat_skill_type", weight: 0.14},
			{key: "Description", label: "i18n:plugin_ai_chat_skill_description", weight: 0.48},
			{key: "_action", label: "i18n:ui_operation", weight: 0.12},
		}, a.translate("i18n:ui_ai_skills_tooltip"), 6
	}
}

func (a *App) inlineTableCellValue(definition formDefinition, key string, row map[string]any) string {
	if key == "_action" {
		return a.translate("i18n:ui_setting_theme_edit")
	}
	if key == "Source" {
		if strings.EqualFold(fmt.Sprint(row[key]), "remote") {
			return a.translate("i18n:ui_ai_skill_type_remote")
		}
		return a.translate("i18n:ui_ai_skill_type_local")
	}
	for _, column := range definition.Value.Columns {
		if column.Key == key {
			return compactFormTableText(a.formTableDisplayValue(column, row), 34)
		}
	}
	return compactFormTableText(fmt.Sprint(row[key]), 34)
}

// newAISettingsForm maps the core settings arrays onto the shared portable table editor.
func newAISettingsForm(data settingsData) formFieldsState {
	definitions := []formDefinition{
		{
			Type: "table",
			Value: formDefinitionValue{
				Key:   "AIProviders",
				Title: "i18n:ui_ai_model",
				Columns: []formTableColumn{
					{Key: "Name", Label: "i18n:ui_ai_providers_name", Type: "select", Validators: []formValidator{{Type: "not_empty"}}},
					{Key: "Alias", Label: "i18n:ui_ai_providers_alias", Type: "text"},
					{Key: "Host", Label: "i18n:ui_ai_providers_host", Type: "text"},
					{Key: "ApiKey", Label: "i18n:ui_ai_providers_api_key", Type: "text", HideInTable: true},
				},
			},
		},
		{
			Type: "table",
			Value: formDefinitionValue{
				Key:     "AIMCPServers",
				Title:   "i18n:ui_ai_mcp_servers",
				Tooltip: "i18n:ui_ai_mcp_servers_tooltip",
				Columns: []formTableColumn{
					{Key: "Name", Label: "i18n:plugin_ai_chat_mcp_server_name", Type: "text", Validators: []formValidator{{Type: "not_empty"}}},
					{Key: "Tools", Label: "i18n:plugin_ai_chat_mcp_server_tools", Type: "text", HideInUpdate: true},
					{Key: "Disabled", Label: "i18n:plugin_ai_chat_mcp_server_disabled", Type: "checkbox"},
					{Key: "Type", Label: "i18n:plugin_ai_chat_mcp_server_type", Type: "select", SelectOptions: []formOption{{Label: "STDIO", Value: "stdio"}, {Label: "Streamable HTTP", Value: "streamable-http"}}, Validators: []formValidator{{Type: "not_empty"}}},
					{Key: "Command", Label: "i18n:plugin_ai_chat_mcp_server_command", Type: "text"},
					{Key: "EnvironmentVariables", Label: "i18n:plugin_ai_chat_mcp_server_environment_variables", Type: "textList", TextMaxLines: 6},
					{Key: "Url", Label: "i18n:plugin_ai_chat_mcp_server_url", Type: "text", TextMaxLines: 4},
				},
			},
		},
		{
			Type: "table",
			Value: formDefinitionValue{
				Key:     "AISkills",
				Title:   "i18n:ui_ai_skills",
				Tooltip: "i18n:ui_ai_skills_tooltip",
				Columns: []formTableColumn{
					{Key: "Name", Label: "i18n:plugin_ai_chat_skill_name", Type: "text", HideInUpdate: true},
					{Key: "Source", Label: "i18n:plugin_ai_chat_skill_type", Type: "text", HideInUpdate: true},
					{Key: "Description", Label: "i18n:plugin_ai_chat_skill_description", Type: "text", HideInUpdate: true},
					{Key: "SourceUrl", Label: "i18n:plugin_ai_chat_skill_source_url", Type: "text", HideInUpdate: true, HideInTable: true},
					{Key: "SourceName", Type: "text", HideInUpdate: true, HideInTable: true},
					{Key: "ManifestPath", Type: "text", HideInUpdate: true, HideInTable: true},
					{Key: "Enabled", Type: "checkbox", HideInUpdate: true, HideInTable: true},
					{Key: "Error", Type: "text", HideInUpdate: true, HideInTable: true},
					{Key: "Path", Label: "i18n:ui_ai_skill_add_path", Type: "dirPath", Validators: []formValidator{{Type: "not_empty"}}},
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
func (a *App) loadAIProviderCatalog() {
	a.mu.Lock()
	if a.aiProvidersLoading || a.aiProvidersLoaded {
		a.mu.Unlock()
		return
	}
	a.aiProvidersLoading = true
	a.aiProvidersError = ""
	a.mu.Unlock()
	a.invalidateSettingsWindow()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var providers []aiProviderInfo
	err := a.client.Post(ctx, "/ai/providers", map[string]any{}, &providers)

	a.mu.Lock()
	a.aiProvidersLoading = false
	a.aiProvidersLoaded = err == nil
	if err != nil {
		a.aiProvidersError = err.Error()
	} else {
		a.aiProviderCatalog = providers
		a.aiProvidersError = ""
		if a.aiSettingsForm != nil {
			applyAIProviderCatalogLocked(a.aiSettingsForm, providers)
		}
		if state := a.tableEditor; state != nil && state.target == a.aiSettingsForm && state.definition.Value.Key == "AIProviders" {
			state.definition = state.target.definitions[state.fieldIndex]
			applyAIProviderOptionsToRowFormLocked(state.rowForm, state.definition)
			applyAIProviderDefaultHostLocked(state, false, providers)
		}
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
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
	a.mu.RLock()
	active := a.settingsOpen && a.settingTab == "ai" && a.aiSettingsForm != nil && a.tableEditor == nil
	a.mu.RUnlock()
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
	a.mu.Lock()
	if a.aiSettingsForm != nil && index >= 0 && index < len(a.aiSettingsForm.definitions) {
		a.settingRow = index
		setFormFieldsFocusLocked(a.aiSettingsForm, index)
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

// moveAISettingsTable wraps table-card selection without entering the modal editor.
func (a *App) moveAISettingsTable(delta int) {
	a.mu.Lock()
	if a.aiSettingsForm != nil && len(a.aiSettingsForm.definitions) > 0 {
		a.settingRow = (a.settingRow + delta + len(a.aiSettingsForm.definitions)) % len(a.aiSettingsForm.definitions)
		setFormFieldsFocusLocked(a.aiSettingsForm, a.settingRow)
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

func (a *App) openSelectedAISettingsTable() {
	a.mu.RLock()
	index := a.settingRow
	a.mu.RUnlock()
	a.openAISettingsTable(index)
}

// openAISettingsTable opens a settings-owned target in the same modal table editor used by plugin forms.
func (a *App) openAISettingsTable(index int) {
	a.mu.Lock()
	if a.settingsOpen && a.settingTab == "ai" && a.aiSettingsForm != nil {
		a.settingRow = index
		a.openFormTableLocked(a.aiSettingsForm, index)
	}
	a.mu.Unlock()
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
	a.mu.Lock()
	if a.settingsOpen && a.settingTab == "ai" && a.aiSettingsForm != nil {
		a.settingRow = tableIndex
		a.openFormTableLocked(a.aiSettingsForm, tableIndex)
		if a.tableEditor != nil && rowIndex >= 0 && rowIndex < len(a.tableEditor.rows) {
			a.tableEditor.selected = rowIndex
		}
	}
	a.mu.Unlock()
	a.finishOpeningFormTable()
	if tableIndex < 2 {
		a.beginEditFormTableRowDirect()
	}
}

// beginCloneRemoteAISkill reuses the row form surface for the one URL needed by core's clone endpoint.
func (a *App) beginCloneRemoteAISkill() {
	a.mu.Lock()
	state := a.tableEditor
	if state == nil || state.definition.Value.Key != "AISkills" || state.invalid || state.saving || state.rowForm != nil || state.target != a.aiSettingsForm {
		a.mu.Unlock()
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
	state.deleteArmed = -1
	a.mu.Unlock()
	a.updateSettingsTextInput(true)
	a.invalidateSettingsWindow()
}

// cloneRemoteAISkills discovers repository skills, appends them atomically, then saves the combined setting.
func (a *App) cloneRemoteAISkills(state *formTableEditorState, url, previousValue string) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	var skills []map[string]any
	err := a.client.Post(ctx, "/ai/skills/clone", map[string]string{"url": url}, &skills)
	cancel()
	if err == nil && len(skills) == 0 {
		err = fmt.Errorf("the repository did not contain any skills")
	}

	a.mu.Lock()
	if err != nil {
		a.settingSaving = false
		if a.tableEditor == state {
			state.saving = false
			state.status = "Could not clone: " + err.Error()
		}
		a.settingNote = "Could not clone remote skills: " + err.Error()
		a.mu.Unlock()
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
		a.mu.Unlock()
		a.invalidateSettingsWindow()
		return
	}
	value := state.target.values[state.definition.Value.Key]
	if a.tableEditor == state {
		state.status = "Saving cloned skills…"
		a.ensureFormTableSelectionVisibleLocked()
	}
	a.mu.Unlock()
	a.saveSettingsTable(state, "AISkills", value, previousValue)
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
			a.mu.Lock()
			a.settingSaving = false
			state.target.values[key] = previousValue
			if a.tableEditor == state {
				state.saving = false
				state.status = "Could not save: " + err.Error()
			}
			a.settingNote = "Could not save " + settingsTableLabel(key) + ": " + err.Error()
			a.mu.Unlock()
			a.invalidateSettingsWindow()
			return
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err := a.client.Post(ctx, "/setting/wox/update", map[string]string{"Key": key, "Value": coreValue}, nil)
	cancel()

	a.mu.Lock()
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
		if state.target == a.aiSettingsForm {
			a.applyAISettingsRawLocked(key, value)
		} else if state.target == a.hotkeySettingsForm {
			a.applyHotkeySettingsRawLocked(key, coreValue)
		}
		if a.tableEditor == state {
			state.saving = false
			state.status = "Saved"
		}
		a.settingNote = settingsTableLabel(key) + " saved"
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
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
		a.settings.AIProviders = raw
		a.aiModelsLoaded = false
		a.aiModelsError = ""
	case "AIMCPServers":
		a.settings.AIMCPServers = raw
	case "AISkills":
		a.settings.AISkills = raw
		a.aiSkillsLoaded = false
		a.aiSkillsError = ""
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
