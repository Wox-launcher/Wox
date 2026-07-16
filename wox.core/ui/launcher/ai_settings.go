package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	woxui "wox/ui/runtime"
)

type aiProviderInfo struct {
	Name        string
	DefaultHost string
}

// newAISettingsForm maps the core settings arrays onto the shared portable table editor.
func newAISettingsForm(data settingsData) formFieldsState {
	definitions := []formDefinition{
		{
			Type: "table",
			Value: formDefinitionValue{
				Key:   "AIProviders",
				Title: "AI providers",
				Columns: []formTableColumn{
					{Key: "Name", Label: "Provider", Type: "select", Validators: []formValidator{{Type: "not_empty"}}},
					{Key: "Alias", Label: "Alias", Type: "text"},
					{Key: "Host", Label: "Host", Type: "text"},
					{Key: "ApiKey", Label: "API key", Type: "text", HideInTable: true},
				},
			},
		},
		{
			Type: "table",
			Value: formDefinitionValue{
				Key:   "AIMCPServers",
				Title: "MCP servers",
				Columns: []formTableColumn{
					{Key: "Name", Label: "Name", Type: "text", Validators: []formValidator{{Type: "not_empty"}}},
					{Key: "Disabled", Label: "Disabled", Type: "checkbox"},
					{Key: "Type", Label: "Type", Type: "select", SelectOptions: []formOption{{Label: "STDIO", Value: "stdio"}, {Label: "Streamable HTTP", Value: "streamable-http"}}, Validators: []formValidator{{Type: "not_empty"}}},
					{Key: "Command", Label: "Command", Type: "text"},
					{Key: "EnvironmentVariables", Label: "Environment variables", Type: "textList", TextMaxLines: 6},
					{Key: "Url", Label: "URL", Type: "text", TextMaxLines: 4},
				},
			},
		},
		{
			Type: "table",
			Value: formDefinitionValue{
				Key:   "AISkills",
				Title: "Skills",
				Columns: []formTableColumn{
					{Key: "Name", Label: "Name", Type: "text", HideInUpdate: true},
					{Key: "Source", Label: "Source", Type: "text", HideInUpdate: true},
					{Key: "Description", Label: "Description", Type: "text", HideInUpdate: true},
					{Key: "SourceUrl", Label: "Source URL", Type: "text", HideInUpdate: true, HideInTable: true},
					{Key: "SourceName", Type: "text", HideInUpdate: true, HideInTable: true},
					{Key: "ManifestPath", Type: "text", HideInUpdate: true, HideInTable: true},
					{Key: "Enabled", Type: "checkbox", HideInUpdate: true, HideInTable: true},
					{Key: "Error", Type: "text", HideInUpdate: true, HideInTable: true},
					{Key: "Path", Label: "Local directory", Type: "dirPath", Validators: []formValidator{{Type: "not_empty"}}},
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
	_ = a.window.Invalidate()

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
	_ = a.window.Invalidate()
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
	active := a.mode == viewSettings && a.settingTab == "ai" && a.aiSettingsForm != nil && a.tableEditor == nil
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
	_ = a.window.Invalidate()
}

// moveAISettingsTable wraps table-card selection without entering the modal editor.
func (a *App) moveAISettingsTable(delta int) {
	a.mu.Lock()
	if a.aiSettingsForm != nil && len(a.aiSettingsForm.definitions) > 0 {
		a.settingRow = (a.settingRow + delta + len(a.aiSettingsForm.definitions)) % len(a.aiSettingsForm.definitions)
		setFormFieldsFocusLocked(a.aiSettingsForm, a.settingRow)
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
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
	if a.mode == viewSettings && a.settingTab == "ai" && a.aiSettingsForm != nil {
		a.settingRow = index
		a.openFormTableLocked(a.aiSettingsForm, index)
	}
	a.mu.Unlock()
	a.finishOpeningFormTable()
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
	a.updateFormTextInput(true)
	_ = a.window.Invalidate()
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
		_ = a.window.Invalidate()
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
		_ = a.window.Invalidate()
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
			_ = a.window.Invalidate()
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
	_ = a.window.Invalidate()
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
