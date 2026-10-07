package launcher

import (
	"testing"
	"wox/network"
)

// TestPluginOfflineStatusAfterStartup does not depend on a settings-toggle notification.
func TestPluginOfflineStatusAfterStartup(t *testing.T) {
	previous := network.IsOffline()
	network.Default.SetOffline(true)
	defer network.Default.SetOffline(previous)
	app := &App{}
	for _, disabled := range []bool{false, true} {
		plugin := pluginSettingsPlugin{ID: "offline-plugin", IsInstalled: true, IsDisable: disabled}
		actions := app.pluginManagementActions(settingsSnapshot{}, plugin)
		found := false
		for _, action := range actions {
			if action.ID == "plugin-enable" || action.ID == "plugin-disable" {
				found = true
				if action.Enabled || action.OnTap != nil || action.Label != app.translate("i18n:ui_plugin_offline_paused") {
					t.Fatal("offline plugin lifecycle action did not explain the pause")
				}
			}
		}
		if !found {
			t.Fatal("missing lifecycle status")
		}
		if plugin.IsDisable != disabled {
			t.Fatal("saved preference changed")
		}
	}
	network.Default.SetOffline(false)
	for _, action := range app.pluginManagementActions(settingsSnapshot{}, pluginSettingsPlugin{IsInstalled: true}) {
		if action.ID == "plugin-disable" && !action.Enabled {
			t.Fatal("online lifecycle action remained disabled")
		}
	}
}
