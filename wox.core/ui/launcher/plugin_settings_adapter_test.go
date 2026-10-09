package launcher

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPluginListEntriesGroupInstalledAndStoreCatalogs(t *testing.T) {
	app := &App{translations: map[string]string{
		"ui_update":                          "Update",
		"ui_disabled":                        "Disabled",
		"ui_setting_plugin_section_enabled":  "Enabled",
		"ui_setting_plugin_section_disabled": "Disabled",
		"ui_plugin_store_wox":                "Wox Store",
		"ui_plugin_store_flow":               "Flow Store",
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
	}, 1)
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
				{ID: "flow-b", Name: "Beta", Version: "1.0.0", Author: "A", Store: "flow"},
				{ID: "wox-z", Name: "Zinc", Version: "2.0.0", Author: "B", Store: "wox"},
				{ID: "flow-a", Name: "Alpine", Version: "1.0.0", Author: "C", Store: "flow", IsDisable: true},
				{ID: "wox-a", Name: "Alpha", Version: "1.0.0", Author: "D"},
			},
		},
	}, []filteredPlugin{
		{index: 0, plugin: pluginSettingsPlugin{ID: "flow-b", Name: "Beta", Version: "1.0.0", Author: "A", Store: "flow"}},
		{index: 1, plugin: pluginSettingsPlugin{ID: "wox-z", Name: "Zinc", Version: "2.0.0", Author: "B", Store: "wox"}},
		{index: 2, plugin: pluginSettingsPlugin{ID: "flow-a", Name: "Alpine", Version: "1.0.0", Author: "C", Store: "flow", IsDisable: true}},
		{index: 3, plugin: pluginSettingsPlugin{ID: "wox-a", Name: "Alpha", Version: "1.0.0", Author: "D"}},
	}, 1)
	if len(store) != 6 {
		t.Fatalf("store entries = %d, want 2 headers and 4 plugins", len(store))
	}
	if store[0].Header != "Wox Store" || store[0].ID != "wox" || !store[0].HasHeaderIcon {
		t.Fatalf("wox header = %#v", store[0])
	}
	if store[1].Item.ID != "wox-a" || store[2].Item.ID != "wox-z" {
		t.Fatalf("wox plugins = %s %s", store[1].Item.ID, store[2].Item.ID)
	}
	if store[3].Header != "Flow Store" || store[3].ID != "flow" || !store[3].HasHeaderIcon {
		t.Fatalf("flow header = %#v", store[3])
	}
	if store[4].Item.ID != "flow-a" || store[4].Item.Status != "1.0.0  C" || strings.Contains(store[4].Item.Status, "Disabled") {
		t.Fatalf("flow status = %#v", store[4].Item)
	}
	if store[5].Item.ID != "flow-b" {
		t.Fatalf("flow plugins = %s", store[5].Item.ID)
	}
	if installed[1].Item.Icon != nil || store[1].Item.Icon != nil {
		t.Fatal("catalog rows must leave icons unresolved until the row is built")
	}
}

func TestPluginListEntriesOmitHeaderWhenOneStoreRemains(t *testing.T) {
	app := &App{translations: map[string]string{"ui_plugin_store_wox": "Wox Store"}}
	entries := app.pluginListEntries(settingsSnapshot{
		plugins: pluginSettingsSnapshot{PluginsStore: true},
	}, []filteredPlugin{
		{index: 0, plugin: pluginSettingsPlugin{ID: "wox-z", Name: "Zinc", Store: "wox"}},
		{index: 1, plugin: pluginSettingsPlugin{ID: "wox-a", Name: "Alpha"}},
	}, 1)
	if len(entries) != 2 || entries[0].Header != "" || entries[1].Header != "" {
		t.Fatalf("single-store entries = %#v, want plugin rows without a header", entries)
	}
	if entries[0].Item.ID != "wox-a" || entries[1].Item.ID != "wox-z" {
		t.Fatalf("single-store order = %s %s", entries[0].Item.ID, entries[1].Item.ID)
	}
}

func TestPluginStorePanelLocksWoxAndListsOtherStores(t *testing.T) {
	app := &App{translations: map[string]string{
		"ui_plugin_store_wox":  "Wox Store",
		"ui_plugin_store_flow": "Flow Store",
	}}
	if panel := app.pluginStorePanelProps(settingsSnapshot{plugins: pluginSettingsSnapshot{PluginsStore: true}}, 1); panel != nil {
		t.Fatal("closed store settings must not build a panel")
	}
	panel := app.pluginStorePanelProps(settingsSnapshot{
		plugins: pluginSettingsSnapshot{
			PluginsStore: true, PluginStorePanelOpen: true,
			Plugins: []pluginSettingsPlugin{{Store: "flow"}, {Store: "wox"}, {Store: ""}},
		},
		general: generalSettingsSnapshot{Data: settingsData{HiddenPluginStores: []string{"flow", "wox"}}},
	}, 1)
	if panel == nil || len(panel.Rows) != 2 {
		t.Fatalf("rows = %#v, want Wox and Flow", panel)
	}
	if panel.Rows[0].ID != "wox" || panel.Rows[0].Label != "Wox Store" || !panel.Rows[0].Locked || !panel.Rows[0].Checked {
		t.Fatalf("wox row = %#v", panel.Rows[0])
	}
	if panel.Rows[1].ID != "flow" || panel.Rows[1].Label != "Flow Store" || panel.Rows[1].Locked || panel.Rows[1].Checked {
		t.Fatalf("flow row = %#v", panel.Rows[1])
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
		}, 1)
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
