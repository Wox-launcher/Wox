package launcher

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPluginListEntriesGroupInstalledAndStayFlatInStore(t *testing.T) {
	app := &App{translations: map[string]string{
		"ui_update":                          "Update",
		"ui_disabled":                        "Disabled",
		"ui_setting_plugin_section_enabled":  "Enabled",
		"ui_setting_plugin_section_disabled": "Disabled",
	}}
	installed := app.pluginListEntries(settingsSnapshot{
		plugins: pluginSettingsSnapshot{
			Plugins: []pluginSettingsPlugin{
				{ID: "off", Name: "Off", Version: "1.0.0", Author: "A", IsDisable: true, IsUpgradable: true},
				{ID: "on", Name: "On", Version: "2.0.0", Author: "B"},
			},
			PluginSelected: 1,
		},
	}, []filteredPlugin{
		{index: 0, plugin: pluginSettingsPlugin{ID: "off", Name: "Off", Version: "1.0.0", Author: "A", IsDisable: true, IsUpgradable: true}},
		{index: 1, plugin: pluginSettingsPlugin{ID: "on", Name: "On", Version: "2.0.0", Author: "B"}},
	})
	if len(installed) != 4 {
		t.Fatalf("installed entries = %d, want 4", len(installed))
	}
	if installed[0].Header != "Enabled" || installed[0].ID != pluginSectionEnabled {
		t.Fatalf("enabled header = %#v", installed[0])
	}
	if installed[1].Item.ID != "on" || !installed[1].Item.Selected || installed[1].Item.Disabled || installed[1].Item.Status != "2.0.0  B" {
		t.Fatalf("enabled plugin = %#v", installed[1].Item)
	}
	if installed[2].Header != "Disabled" || installed[2].ID != pluginSectionDisabled {
		t.Fatalf("disabled header = %#v", installed[2])
	}
	if installed[3].Item.ID != "off" || !installed[3].Item.Disabled || installed[3].Item.Status != "Update  1.0.0  A" || strings.Contains(installed[3].Item.Status, "Disabled") {
		t.Fatalf("disabled status = %q, want Update prefix without Disabled", installed[3].Item.Status)
	}

	store := app.pluginListEntries(settingsSnapshot{
		plugins: pluginSettingsSnapshot{
			PluginsStore: true,
			Plugins: []pluginSettingsPlugin{
				{ID: "off", Name: "Off", Version: "1.0.0", Author: "A", IsDisable: true},
				{ID: "on", Name: "On", Version: "2.0.0", Author: "B"},
			},
		},
	}, []filteredPlugin{
		{index: 0, plugin: pluginSettingsPlugin{ID: "off", Name: "Off", Version: "1.0.0", Author: "A", IsDisable: true}},
		{index: 1, plugin: pluginSettingsPlugin{ID: "on", Name: "On", Version: "2.0.0", Author: "B"}},
	})
	if len(store) != 2 || store[0].Header != "" || store[1].Header != "" {
		t.Fatalf("store entries = %#v, want a flat catalog", store)
	}
	if store[0].Item.ID != "off" || store[0].Item.Status != "1.0.0  A" || strings.Contains(store[0].Item.Status, "Disabled") {
		t.Fatalf("store disabled status = %q", store[0].Item.Status)
	}
	if installed[1].Item.Icon != nil || store[0].Item.Icon != nil {
		t.Fatal("catalog rows must leave icons unresolved until the row is built")
	}
}

func TestPluginListEntriesOmitScriptBadge(t *testing.T) {
	app := &App{translations: map[string]string{
		"ui_setting_plugin_script_tag": "Script",
		"ui_setting_plugin_system_tag": "System",
		"ui_plugin_dev_tag":            "Dev",
	}}
	for _, store := range []bool{false, true} {
		entries := app.pluginListEntries(settingsSnapshot{plugins: pluginSettingsSnapshot{PluginsStore: store}}, []filteredPlugin{
			{plugin: pluginSettingsPlugin{ID: "script", Runtime: "Script"}},
			{plugin: pluginSettingsPlugin{ID: "system", IsSystem: true}},
			{plugin: pluginSettingsPlugin{ID: "dev", Runtime: "Script", IsDev: true}},
		})
		badges := map[string]string{}
		for _, entry := range entries {
			if entry.Header == "" {
				badges[entry.ID] = entry.Item.Badge
			}
		}
		assert.Equal(t, map[string]string{"script": "", "system": "System", "dev": "Dev"}, badges)
	}
}

func TestPluginRuntimeLabelOmitsNativeGoHost(t *testing.T) {
	if got := pluginRuntimeLabel("Go"); got != "" {
		t.Fatalf("Go runtime label = %q, want empty so native plugins hide the chip", got)
	}
	if got := pluginRuntimeLabel("python"); got != "Wox Python" {
		t.Fatalf("python runtime label = %q, want Wox Python", got)
	}
	if got := pluginRuntimeLabel("nodejs"); got != "Wox Node.js" {
		t.Fatalf("nodejs runtime label = %q, want Wox Node.js", got)
	}
	if got := pluginRuntimeLabel("script"); got != "Wox Script" {
		t.Fatalf("script runtime label = %q, want Wox Script", got)
	}
	if got := pluginRuntimeLabel("FLOWJSONRPC"); got != "Flow JSON-RPC" {
		t.Fatalf("jsonrpc runtime label = %q, want Flow JSON-RPC", got)
	}
	if got := pluginRuntimeLabel("FLOWDOTNET"); got != "Flow .NET" {
		t.Fatalf("dotnet runtime label = %q, want Flow .NET", got)
	}
}

func TestPluginPrivacyAccessesIncludesFolderOnlyDialogState(t *testing.T) {
	features := []pluginFeature{
		{
			Name: "queryEnv",
			Params: map[string]any{
				"requireActiveWindowIsOpenSaveDialog":             true,
				"requireActiveWindowIsOpenSaveDialogSelectFolder": true,
			},
		},
	}

	assert.Equal(t, []string{
		"requireActiveWindowIsOpenSaveDialog",
		"requireActiveWindowIsOpenSaveDialogSelectFolder",
	}, pluginPrivacyAccesses(features))
}
