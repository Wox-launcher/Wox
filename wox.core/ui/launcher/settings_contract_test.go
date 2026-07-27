package launcher

import (
	"encoding/json"
	"testing"

	"wox/i18n"
	"wox/setting"
	"wox/ui/contract"
)

func TestSettingsDataFromContract(t *testing.T) {
	loaded := contract.GeneralSettings{
		MainHotkey: "Alt+Space",
		LangCode:   i18n.LangCodeZhCn,
		QueryHotkeys: []setting.QueryHotkey{{
			Name: "Docs", Hotkey: "Ctrl+D", Query: "docs", Position: setting.QueryHotkeyPositionTopCenter, MaxResultCount: 8,
		}},
		QueryShortcuts:           []setting.QueryShortcut{{Shortcut: "g", Query: "google {0}"}},
		TrayQueries:              []setting.TrayQuery{{Query: "clipboard", HideQueryBox: true}},
		CloudSyncDisabledPlugins: []string{"plugin-a"},
		PrimaryGlance:            setting.GlanceRef{PluginId: "plugin-a", GlanceId: "weather"},
		UIDensity:                setting.UiDensityCompact,
	}

	data, err := settingsDataFromContract(loaded)
	if err != nil {
		t.Fatalf("settingsDataFromContract returned error: %v", err)
	}
	if data.MainHotkey != "Alt+Space" || data.LangCode != "zh_CN" || data.UIDensity != "compact" {
		t.Fatalf("basic settings = %+v", data)
	}
	if len(data.QueryHotkeys) != 1 || data.QueryHotkeys[0].Position != "top_center" || data.QueryHotkeys[0].MaxResultCount != 8 {
		t.Fatalf("query hotkeys = %+v", data.QueryHotkeys)
	}
	if data.PrimaryGlance.PluginID != "plugin-a" || data.PrimaryGlance.GlanceID != "weather" {
		t.Fatalf("primary glance = %+v", data.PrimaryGlance)
	}
	var trayQueries []setting.TrayQuery
	if err := json.Unmarshal(data.TrayQueries, &trayQueries); err != nil {
		t.Fatalf("decode tray queries: %v", err)
	}
	if len(trayQueries) != 1 || trayQueries[0].Query != "clipboard" || !trayQueries[0].HideQueryBox {
		t.Fatalf("tray queries = %+v", trayQueries)
	}
}
