package launcher

import (
	"context"
	"strings"
	"testing"

	woxai "wox/ai"
	"wox/common"
)

func mcpJSONTestApp(t *testing.T) *App {
	t.Helper()
	form := newAISettingsForm(settingsData{AIMCPServers: []byte(`[{"Name":"docs","Type":"streamable-http","Url":"https://example.com/mcp"}]`)})
	deps := CommonDeps{}
	ai := newAISettingsController(deps)
	ai.SetForm(&form)
	return &App{
		settingsOpen:    true,
		settingTab:      "ai",
		aiSettings:      ai,
		pluginSettings:  newPluginSettingsController(deps),
		hotkeySettings:  newHotkeySettingsController(deps),
		generalSettings: newGeneralSettingsController(deps, newSharedEditState()),
		services:        &skillAddTestServices{},
		lifecycleCtx:    context.Background(),
	}
}

func TestOpenFormTableMCPJSONPrefillsDocument(t *testing.T) {
	app := mcpJSONTestApp(t)
	app.openAIMCPJSONImport()

	state := app.settingsTableEditor
	if state == nil || state.mcpJSONImport == nil {
		t.Fatal("MCP JSON editor did not open")
	}
	if state.definition.Value.Key != "AIMCPServers" {
		t.Fatalf("editor key = %q, want AIMCPServers", state.definition.Value.Key)
	}
	got := state.mcpJSONImport.fields.values["JSON"]
	if !strings.Contains(got, `"mcpServers"`) || !strings.Contains(got, `"docs"`) {
		t.Fatalf("initial JSON = %q", got)
	}
}

func TestImportFormTableMCPJSONRequiresText(t *testing.T) {
	app := mcpJSONTestApp(t)
	app.openAIMCPJSONImport()
	app.setFormTableMCPJSONText(0, "")
	app.importFormTableMCPJSON()

	state := app.settingsTableEditor
	if state == nil || state.mcpJSONImport == nil {
		t.Fatal("dialog should stay open after an empty save")
	}
	if state.mcpJSONImport.error == "" {
		t.Fatal("empty JSON should report an error")
	}
}

func TestAttachMCPServerToolNamesUsesCache(t *testing.T) {
	woxai.ResetMCPClients()
	t.Cleanup(woxai.ResetMCPClients)
	woxai.SetCachedMCPToolsForTest("ddg-search", []common.MCPTool{{Name: "search"}, {Name: "fetch"}})

	got := attachMCPServerToolNames(`[{"Name":"ddg-search","Type":"stdio","Command":"uvx"}]`)
	if !strings.Contains(got, `"search"`) || !strings.Contains(got, `"fetch"`) {
		t.Fatalf("attached tools = %s", got)
	}
}

func TestImportFormTableMCPJSONReplacesDocument(t *testing.T) {
	app := mcpJSONTestApp(t)
	app.openAIMCPJSONImport()
	app.setFormTableMCPJSONText(0, `{
		"mcpServers": {
			"local": {"command": "uvx", "args": ["duckduckgo-mcp-server"]}
		}
	}`)
	app.importFormTableMCPJSON()

	form := app.aiSettings.Form()
	if form == nil {
		t.Fatal("AI settings form missing")
	}
	raw := form.values["AIMCPServers"]
	if strings.Contains(raw, "docs") || !strings.Contains(raw, "local") {
		t.Fatalf("saved rows = %s", raw)
	}
}
