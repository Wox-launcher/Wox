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
}

func TestPluginRuntimeLabelOmitsNativeGoHost(t *testing.T) {
	if got := pluginRuntimeLabel("Go"); got != "" {
		t.Fatalf("Go runtime label = %q, want empty so native plugins hide the chip", got)
	}
	if got := pluginRuntimeLabel("python"); got != "Python" {
		t.Fatalf("python runtime label = %q, want Python", got)
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
