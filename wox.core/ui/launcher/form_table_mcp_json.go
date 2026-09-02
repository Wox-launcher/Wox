package launcher

import (
	"encoding/json"
	"strings"

	"wox/ai"
	"wox/common"
	launcherview "wox/ui/launcher/view"
	woxwidget "wox/ui/widget"
)

const mcpJSONEditorMaxLines = 16

// formTableMCPJSONImportState backs the mcpServers JSON editor dialog.
type formTableMCPJSONImportState struct {
	fields *formFieldsState
	error  string
}

// formTableMCPJSONImportSnapshot copies the JSON editor for one UI frame.
type formTableMCPJSONImportSnapshot struct {
	fields *formFieldsSnapshot
	error  string
}

// openAIMCPJSONImport opens the MCP table editor on the JSON document dialog.
func (a *App) openAIMCPJSONImport() {
	form := a.aiSettings.Form()
	if form == nil {
		return
	}
	index := -1
	for i, definition := range form.definitions {
		if definition.Value.Key == "AIMCPServers" {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	a.openAISettingsTable(index)
	a.openFormTableMCPJSONImport()
}

// openFormTableMCPJSONImport shows the current MCP servers as editable mcpServers JSON.
func (a *App) openFormTableMCPJSONImport() {
	state := a.settingsTableEditor
	if state == nil || state.definition.Value.Key != "AIMCPServers" || state.invalid || state.saving || state.rowForm != nil || state.target != a.aiSettings.Form() {
		return
	}
	initial := "{}\n"
	if current, err := decodeMCPServerConfigs(state.rows); err == nil {
		if formatted, err := ai.FormatMCPServersJSON(current); err == nil {
			initial = formatted
		}
	}
	fields := newFormFieldsState([]formDefinition{
		{Type: "textbox", Value: formDefinitionValue{Key: "JSON", Label: "i18n:ui_ai_mcp_import_json_label", MaxLines: mcpJSONEditorMaxLines}},
	}, map[string]string{"JSON": initial}, true)
	setFormFieldsFocusLocked(&fields, 0)
	state.mcpJSONImport = &formTableMCPJSONImportState{fields: &fields}
	state.status = ""
	state.deletePending = -1
	state.deleteDirect = false
	a.updateFormTableTextInput(true)
	a.invalidateFormTableWindow()
}

// cancelFormTableMCPJSONImport dismisses the JSON editor.
func (a *App) cancelFormTableMCPJSONImport() {
	state := a.activeFormTableEditor()
	if state != nil && state.mcpJSONImport != nil {
		a.closeFormTableEditor()
	}
}

func (a *App) focusFormTableMCPJSONField(index int) {
	state := a.activeFormTableEditor()
	if state == nil || state.mcpJSONImport == nil || index != 0 {
		return
	}
	fields := state.mcpJSONImport.fields
	syncFormFieldsEditorLocked(fields)
	fields.active = true
	if fields.focused != index || fields.editor == nil {
		setFormFieldsFocusLocked(fields, index)
	}
	a.updateFormTableTextInput(fields.editor != nil)
	a.invalidateFormTableWindow()
}

func (a *App) setFormTableMCPJSONText(index int, value string) {
	state := a.activeFormTableEditor()
	if state == nil || state.mcpJSONImport == nil || !setFormFieldsTextLocked(state.mcpJSONImport.fields, index, value) {
		return
	}
	state.mcpJSONImport.error = ""
	a.invalidateFormTableWindow()
}

// importFormTableMCPJSON replaces the MCP table with the edited mcpServers document.
func (a *App) importFormTableMCPJSON() {
	state := a.activeFormTableEditor()
	if state == nil || state.mcpJSONImport == nil || state.saving {
		return
	}
	syncFormFieldsEditorLocked(state.mcpJSONImport.fields)
	raw := strings.TrimSpace(state.mcpJSONImport.fields.values["JSON"])
	if raw == "" {
		state.mcpJSONImport.error = a.translate("i18n:ui_ai_mcp_import_json_required")
		a.invalidateFormTableWindow()
		return
	}
	servers, err := ai.ParseMCPServersDocument(raw)
	if err != nil {
		state.mcpJSONImport.error = err.Error()
		a.invalidateFormTableWindow()
		return
	}
	encoded, err := json.Marshal(servers)
	if err != nil {
		state.mcpJSONImport.error = err.Error()
		a.invalidateFormTableWindow()
		return
	}
	previousValue := state.target.values[state.definition.Value.Key]
	state.target.values[state.definition.Value.Key] = string(encoded)
	state.rows = mcpServerRowsFromConfigs(servers)
	state.saving = true
	a.settingSaving = true
	a.closeFormTableEditor()
	a.saveSettingsTable(state, "AIMCPServers", string(encoded), previousValue)
}

func decodeMCPServerConfigs(rows []map[string]any) ([]common.AIChatMCPServerConfig, error) {
	encoded, err := json.Marshal(rows)
	if err != nil {
		return nil, err
	}
	var configs []common.AIChatMCPServerConfig
	if err := json.Unmarshal(encoded, &configs); err != nil {
		return nil, err
	}
	return configs, nil
}

func mcpServerRowsFromConfigs(configs []common.AIChatMCPServerConfig) []map[string]any {
	encoded, err := json.Marshal(configs)
	if err != nil {
		return nil
	}
	rows, err := decodeFormTableRows(attachMCPServerToolNames(string(encoded)))
	if err != nil {
		return nil
	}
	return rows
}

// attachMCPServerToolNames overlays discovered MCP tool names onto table rows.
func attachMCPServerToolNames(raw string) string {
	rows, err := decodeFormTableRows(raw)
	if err != nil || len(rows) == 0 {
		return raw
	}
	if !overlayMCPServerToolNames(rows) {
		return raw
	}
	encoded, err := json.Marshal(rows)
	if err != nil {
		return raw
	}
	return string(encoded)
}

func mcpServerRowName(row map[string]any) string {
	for _, key := range []string{"Name", "name"} {
		if name, ok := row[key].(string); ok {
			if name = strings.TrimSpace(name); name != "" {
				return name
			}
		}
	}
	return ""
}

// overlayMCPServerToolNames writes cached tool names onto MCP table rows.
func overlayMCPServerToolNames(rows []map[string]any) bool {
	changed := false
	for _, row := range rows {
		name := mcpServerRowName(row)
		if name == "" {
			continue
		}
		names, ok := ai.CachedMCPToolNames(name)
		if !ok {
			continue
		}
		row["Tools"] = names
		changed = true
	}
	return changed
}

// buildFormTableMCPJSONImportDialog maps the JSON editor onto the shared surface.
func (a *App) buildFormTableMCPJSONImportDialog(snapshot *formTableMCPJSONImportSnapshot, palette uiPalette, width, height, imageScale float32) woxwidget.Widget {
	fields := snapshot.fields
	callbacks := formFieldCallbacks{
		idPrefix:   "form-table-mcp-json",
		imageScale: imageScale,
		focus:      a.focusFormTableMCPJSONField,
		setText:    a.setFormTableMCPJSONText,
		onKey:      a.onFormTableKey,
	}
	theme := palette.componentTheme()
	cancelLabel := a.translate("i18n:ui_cancel")
	saveLabel := a.translate("i18n:ui_ai_mcp_import_json_confirm")
	fieldWidth := max(float32(0), min(float32(640), width-120))
	definition := fields.definitions[0]
	field := a.buildFormTableRowField(*fields, callbacks, palette, 0, definition, fieldWidth, a.formTableRowLabelWidth(fields.definitions), "")
	return launcherview.FormTableMCPJSONImportDialog(launcherview.FormTableMCPJSONImportDialogProps{
		Width: width, Height: height,
		Title:       a.translate("i18n:ui_ai_mcp_import_json"),
		Hint:        a.translate("i18n:ui_ai_mcp_import_json_hint"),
		Error:       snapshot.error,
		CancelLabel: cancelLabel, ImportLabel: saveLabel,
		Field: field, FieldHeight: launcherview.FormTableRowFieldHeight(definition.Type, "", mcpJSONEditorMaxLines), Theme: theme,
		OnCancel: a.cancelFormTableMCPJSONImport, OnImport: a.importFormTableMCPJSON,
	})
}
