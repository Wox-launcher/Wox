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
		EnablePrivacyMode:        true,
	}

	data, err := settingsDataFromContract(loaded)
	if err != nil {
		t.Fatalf("settingsDataFromContract returned error: %v", err)
	}
	if data.MainHotkey != "Alt+Space" || data.LangCode != "zh_CN" || data.UIDensity != "compact" {
		t.Fatalf("basic settings = %+v", data)
	}
	if !data.EnablePrivacyMode {
		t.Fatal("private mode was not copied")
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

func TestLocalizedLanguageSettingIncludesDescription(t *testing.T) {
	app := &App{translations: map[string]string{
		"ui_lang":      "语言",
		"ui_lang_tips": "Wox 使用的界面语言",
	}}
	item := app.localizedSettingItem(settingItem{key: "LangCode", title: "Language", description: "Language used by Wox"})

	if item.title != "语言" || item.description != "Wox 使用的界面语言" {
		t.Fatalf("localized language item = %#v", item)
	}
}

func TestLocalizedDebugSettings(t *testing.T) {
	app := &App{translations: map[string]string{
		"ui_cloud_sync_server_url":                             "同步服务地址",
		"ui_cloud_sync_server_url_tips":                        "切换后退出当前同步账户",
		"ui_cloud_sync_server_url_production":                  "产线",
		"ui_cloud_sync_server_url_local":                       "本地",
		"ui_debug_show_score_tail":                             "显示分数尾标",
		"ui_debug_show_score_tail_tips":                        "显示排序分数",
		"ui_debug_show_performance_tail":                       "显示性能尾标",
		"ui_debug_show_performance_tail_tips":                  "显示查询耗时",
		"ui_debug_show_performance_tail_batch":                 "批次",
		"ui_debug_show_performance_tail_batch_tips":            "显示批次",
		"ui_debug_show_performance_tail_plugin_query":          "插件执行时间",
		"ui_debug_show_performance_tail_plugin_query_tips":     "显示插件耗时",
		"ui_debug_show_performance_tail_backend_prepared":      "后端准备发送时间",
		"ui_debug_show_performance_tail_backend_prepared_tips": "显示后端耗时",
		"ui_debug_show_performance_tail_ui_received":           "UI 收到结果时间",
		"ui_debug_show_performance_tail_ui_received_tips":      "显示 UI 接收耗时",
	}}
	items := settingItems("debug", settingsData{})
	wantTitles := []string{"同步服务地址", "显示分数尾标", "显示性能尾标", "批次", "插件执行时间", "后端准备发送时间", "UI 收到结果时间"}
	wantDescriptions := []string{"切换后退出当前同步账户", "显示排序分数", "显示查询耗时", "显示批次", "显示插件耗时", "显示后端耗时", "显示 UI 接收耗时"}
	for index := range items {
		items[index] = app.localizedSettingItem(items[index])
		if items[index].title != wantTitles[index] || items[index].description != wantDescriptions[index] {
			t.Fatalf("localized debug item %d = %#v", index, items[index])
		}
	}
	if items[0].title != "同步服务地址" || items[0].choices[0].label != "产线" || items[0].choices[1].label != "本地" {
		t.Fatalf("localized cloud sync server item = %#v", items[0])
	}
}
