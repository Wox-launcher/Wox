package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"wox/plugin/thirdparty/migrate"
	"wox/setting"
	"wox/util/hotkey"
)

// Catalog lists Flow habits Wox can store: shortcuts, queries, general behavior, and user plugins.
// Flow themes, window coordinates, and built-in plugin keyword remaps are left out because Wox has no matching store.
func (f flowInstallation) Catalog(ctx context.Context) ([]migrate.Category, error) {
	plugins, err := f.Plugins(ctx)
	if err != nil {
		return nil, err
	}
	var categories []migrate.Category
	if items := f.hotkeyItems(); len(items) > 0 {
		categories = append(categories, migrate.Category{ID: migrate.CategoryHotkeys, Items: items})
	}
	if items := f.queryItems(); len(items) > 0 {
		categories = append(categories, migrate.Category{ID: migrate.CategoryQueries, Items: items})
	}
	if items := f.generalItems(); len(items) > 0 {
		categories = append(categories, migrate.Category{ID: migrate.CategoryGeneral, Items: items})
	}
	if items := flowPluginItems(plugins); len(items) > 0 {
		categories = append(categories, migrate.Category{ID: migrate.CategoryPlugins, Items: items})
	}
	return categories, nil
}

func (f flowInstallation) settingImport(ids []string) ([]migrate.SettingWrite, []migrate.PluginResult) {
	selected := map[string]struct{}{}
	for _, id := range ids {
		selected[id] = struct{}{}
	}
	writes := f.settingWrites(selected)
	covered := map[string]struct{}{}
	for _, write := range writes {
		for _, id := range write.ItemIDs {
			covered[id] = struct{}{}
		}
	}
	var failures []migrate.PluginResult
	for _, id := range ids {
		if _, ok := covered[id]; ok {
			continue
		}
		failures = append(failures, migrate.PluginResult{ID: id, Error: "setting cannot be imported"})
	}
	return writes, failures
}

func (f flowInstallation) settingWrites(selected map[string]struct{}) []migrate.SettingWrite {
	var writes []migrate.SettingWrite
	add := func(id, key, value string) {
		if _, ok := selected[id]; !ok || key == "" || value == "" {
			return
		}
		writes = append(writes, migrate.SettingWrite{ItemIDs: []string{id}, Key: key, Value: value})
	}
	if hotkey, ok := flowHotkey(f.settings.Hotkey); ok {
		add("setting:main_hotkey", "MainHotkey", hotkey)
	}
	if hotkey, ok := flowHotkey(f.settings.OpenContextMenuHotkey); ok {
		add("setting:action_hotkey", "ActionPanelHotkey", hotkey)
	}
	add("setting:ignore_fullscreen", "IgnoreHotkeysOnFullscreen", strconv.FormatBool(f.settings.IgnoreHotkeysOnFullscreen))
	if write, ok := f.queryHotkeyWrite(selected); ok {
		writes = append(writes, write)
	}
	if write, ok := f.queryAliasWrite(selected); ok {
		writes = append(writes, write)
	}
	if value, _, ok := flowLaunchMode(f.settings.LastQueryMode); ok {
		add("setting:launch_mode", "LaunchMode", value)
	}
	add("setting:start_page", "StartPage", flowStartPage(f.settings.ShowHistoryResultsForHomePage))
	add("setting:pinyin", "UsePinYin", strconv.FormatBool(f.settings.ShouldUsePinyin))
	add("setting:input_method", "SwitchInputMethodABC", strconv.FormatBool(f.settings.AlwaysStartEn))
	add("setting:autostart", "EnableAutostart", strconv.FormatBool(f.settings.StartFlowLauncherOnSystemStartup))
	add("setting:hide_on_start", "HideOnStart", strconv.FormatBool(f.settings.HideOnStartup))
	add("setting:hide_on_lost_focus", "HideOnLostFocus", strconv.FormatBool(f.settings.HideWhenDeactivated))
	add("setting:show_tray", "ShowTray", strconv.FormatBool(!f.settings.HideNotifyIcon))
	if f.settings.WindowSize > 0 {
		add("setting:app_width", "AppWidth", strconv.Itoa(f.settings.WindowSize))
	}
	if f.settings.MaxResultsToShow > 0 {
		add("setting:max_results", "MaxResultCount", strconv.Itoa(f.settings.MaxResultsToShow))
	}
	if value, _, ok := flowShowPosition(f.settings.SearchWindowScreen); ok {
		add("setting:show_position", "ShowPosition", value)
	}
	if code := flowLangCode(f.settings.Language); code != "" {
		add("setting:language", "LangCode", code)
	}
	if font := strings.TrimSpace(f.settings.QueryBoxFont); font != "" {
		add("setting:font", "AppFontFamily", font)
	}
	if path := strings.TrimSpace(f.settings.PluginSettings.PythonExecutablePath); path != "" {
		add("setting:python", "CustomPythonPath", path)
	}
	if path := strings.TrimSpace(f.settings.PluginSettings.NodeExecutablePath); path != "" {
		add("setting:node", "CustomNodejsPath", path)
	}
	if url := flowProxyURL(f.settings.Proxy); url != "" {
		if _, ok := selected["setting:proxy"]; ok {
			writes = append(writes,
				migrate.SettingWrite{ItemIDs: []string{"setting:proxy"}, Key: "HttpProxyUrl", Value: url},
				migrate.SettingWrite{ItemIDs: []string{"setting:proxy"}, Key: "HttpProxyEnabled", Value: "true"},
			)
		}
	}
	add("setting:auto_update", "EnableAutoUpdate", strconv.FormatBool(f.settings.AutoUpdates))
	return writes
}

func (f flowInstallation) hotkeyItems() []migrate.Item {
	var items []migrate.Item
	if label, ok := flowHotkeyLabel(f.settings.Hotkey); ok {
		items = append(items, flowSettingItem("setting:main_hotkey", "main_hotkey", "", label))
	}
	if label, ok := flowHotkeyLabel(f.settings.OpenContextMenuHotkey); ok {
		items = append(items, flowSettingItem("setting:action_hotkey", "action_hotkey", "", label))
	}
	items = append(items, flowBoolItem("setting:ignore_fullscreen", "ignore_fullscreen", f.settings.IgnoreHotkeysOnFullscreen))
	for index, item := range f.settings.CustomPluginHotkeys {
		label, ok := flowHotkeyLabel(item.Hotkey)
		query := strings.TrimSpace(item.ActionKeyword)
		if !ok || query == "" {
			continue
		}
		items = append(items, flowSettingItem(flowQueryHotkeyID(index), "query_hotkey", "", label+" → "+query))
	}
	return items
}

func (f flowInstallation) queryItems() []migrate.Item {
	var items []migrate.Item
	for index, item := range f.settings.CustomShortcuts {
		key := strings.TrimSpace(item.Key)
		value := strings.TrimSpace(item.Value)
		if key == "" || value == "" {
			continue
		}
		items = append(items, flowSettingItem(flowQueryAliasID(index), "query_alias", "", key+" → "+value))
	}
	if _, detail, ok := flowLaunchMode(f.settings.LastQueryMode); ok {
		items = append(items, flowSettingItem("setting:launch_mode", "launch_mode", detail, ""))
	}
	detail := migrate.ValueStartBlank
	if f.settings.ShowHistoryResultsForHomePage {
		detail = migrate.ValueStartMRU
	}
	items = append(items,
		flowSettingItem("setting:start_page", "start_page", detail, ""),
		flowBoolItem("setting:pinyin", "pinyin", f.settings.ShouldUsePinyin),
		flowBoolItem("setting:input_method", "input_method", f.settings.AlwaysStartEn),
	)
	return items
}

func (f flowInstallation) generalItems() []migrate.Item {
	items := []migrate.Item{
		flowBoolItem("setting:autostart", "autostart", f.settings.StartFlowLauncherOnSystemStartup),
		flowBoolItem("setting:hide_on_start", "hide_on_start", f.settings.HideOnStartup),
		flowBoolItem("setting:hide_on_lost_focus", "hide_on_lost_focus", f.settings.HideWhenDeactivated),
		flowBoolItem("setting:show_tray", "show_tray", !f.settings.HideNotifyIcon),
	}
	if f.settings.WindowSize > 0 {
		items = append(items, flowSettingItem("setting:app_width", "app_width", "", strconv.Itoa(f.settings.WindowSize)))
	}
	if f.settings.MaxResultsToShow > 0 {
		items = append(items, flowSettingItem("setting:max_results", "max_results", "", strconv.Itoa(f.settings.MaxResultsToShow)))
	}
	if _, detail, ok := flowShowPosition(f.settings.SearchWindowScreen); ok {
		items = append(items, flowSettingItem("setting:show_position", "show_position", detail, ""))
	}
	if code := flowLangCode(f.settings.Language); code != "" {
		items = append(items, flowSettingItem("setting:language", "language", "", code))
	}
	if font := strings.TrimSpace(f.settings.QueryBoxFont); font != "" {
		items = append(items, flowSettingItem("setting:font", "font", "", font))
	}
	if path := strings.TrimSpace(f.settings.PluginSettings.PythonExecutablePath); path != "" {
		items = append(items, flowSettingItem("setting:python", "python", "", path))
	}
	if path := strings.TrimSpace(f.settings.PluginSettings.NodeExecutablePath); path != "" {
		items = append(items, flowSettingItem("setting:node", "node", "", path))
	}
	if url := flowProxyURL(f.settings.Proxy); url != "" {
		items = append(items, flowSettingItem("setting:proxy", "proxy", "", url))
	}
	items = append(items, flowBoolItem("setting:auto_update", "auto_update", f.settings.AutoUpdates))
	return items
}

func flowPluginItems(plugins []migrate.Plugin) []migrate.Item {
	items := make([]migrate.Item, 0, len(plugins))
	for _, plugin := range plugins {
		if strings.TrimSpace(plugin.ID) == "" {
			continue
		}
		detail := plugin.Description
		if label := flowVisibleKeywords(plugin.Keywords); label != "" {
			if detail != "" {
				detail += " · "
			}
			detail += label
		}
		items = append(items, migrate.Item{
			ID: "plugin:" + plugin.ID, Title: plugin.Name, Detail: detail, DetailCode: plugin.Detail,
			Icon: plugin.Icon, Selectable: plugin.Selectable, Status: plugin.Status,
		})
	}
	return items
}

func (f flowInstallation) queryHotkeyWrite(selected map[string]struct{}) (migrate.SettingWrite, bool) {
	var hotkeys []setting.QueryHotkey
	var ids []string
	for index, item := range f.settings.CustomPluginHotkeys {
		id := flowQueryHotkeyID(index)
		if _, ok := selected[id]; !ok {
			continue
		}
		combo, ok := flowHotkey(item.Hotkey)
		query := strings.TrimSpace(item.ActionKeyword)
		if !ok || query == "" {
			continue
		}
		hotkeys = append(hotkeys, setting.QueryHotkey{Hotkey: combo, Query: query})
		ids = append(ids, id)
	}
	if len(hotkeys) == 0 {
		return migrate.SettingWrite{}, false
	}
	raw, err := json.Marshal(hotkeys)
	if err != nil {
		return migrate.SettingWrite{}, false
	}
	return migrate.SettingWrite{ItemIDs: ids, Key: "QueryHotkeys", Value: string(raw)}, true
}

func (f flowInstallation) queryAliasWrite(selected map[string]struct{}) (migrate.SettingWrite, bool) {
	var aliases []setting.QueryAlias
	var ids []string
	for index, item := range f.settings.CustomShortcuts {
		id := flowQueryAliasID(index)
		if _, ok := selected[id]; !ok {
			continue
		}
		key := strings.TrimSpace(item.Key)
		value := strings.TrimSpace(item.Value)
		if key == "" || value == "" {
			continue
		}
		aliases = append(aliases, setting.QueryAlias{Alias: key, Query: value})
		ids = append(ids, id)
	}
	if len(aliases) == 0 {
		return migrate.SettingWrite{}, false
	}
	raw, err := json.Marshal(aliases)
	if err != nil {
		return migrate.SettingWrite{}, false
	}
	return migrate.SettingWrite{ItemIDs: ids, Key: "QueryAliases", Value: string(raw)}, true
}

func flowSettingItem(id, titleKey, detailKey, detail string) migrate.Item {
	return migrate.Item{
		ID: id, TitleKey: titleKey, DetailKey: detailKey, Detail: detail,
		Selectable: true, Status: migrate.PluginReady,
	}
}

func flowBoolItem(id, titleKey string, value bool) migrate.Item {
	detail := migrate.ValueOff
	if value {
		detail = migrate.ValueOn
	}
	return flowSettingItem(id, titleKey, detail, "")
}

func flowQueryHotkeyID(index int) string {
	return fmt.Sprintf("setting:query_hotkey:%d", index)
}

func flowQueryAliasID(index int) string {
	return fmt.Sprintf("setting:query_alias:%d", index)
}

func flowHotkey(value string) (string, bool) {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, " + ", "+")
	value = strings.ReplaceAll(value, " ", "")
	if value == "" {
		return "", false
	}
	if _, err := hotkey.ParseCombo(value); err != nil {
		return "", false
	}
	return value, true
}

func flowHotkeyLabel(value string) (string, bool) {
	normalized, ok := flowHotkey(value)
	if !ok {
		return "", false
	}
	return strings.ReplaceAll(normalized, "+", " + "), true
}

func flowLaunchMode(value string) (string, string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "empty":
		return string(setting.LaunchModeFresh), migrate.ValueLaunchFresh, true
	case "selected", "preserved":
		return string(setting.LaunchModeContinue), migrate.ValueLaunchContinue, true
	default:
		return "", "", false
	}
}

func flowStartPage(showHistory bool) string {
	if showHistory {
		return string(setting.StartPageMRU)
	}
	return string(setting.StartPageBlank)
}

func flowShowPosition(value string) (string, string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "cursor":
		return string(setting.PositionTypeMouseScreen), migrate.ValuePositionCursor, true
	case "focus":
		return string(setting.PositionTypeActiveScreen), migrate.ValuePositionFocus, true
	default:
		return "", "", false
	}
}

func flowLangCode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "zh-cn", "zh_cn", "zh-hans", "zh":
		return "zh_CN"
	case "en", "en-us", "en_us":
		return "en_US"
	case "ja", "ja-jp", "ja_jp":
		return "ja_JP"
	case "ko", "ko-kr", "ko_kr":
		return "ko_KR"
	case "pt-br", "pt_br":
		return "pt_BR"
	case "ru", "ru-ru", "ru_ru":
		return "ru_RU"
	default:
		return ""
	}
}

func flowProxyURL(proxy flowProxySettings) string {
	server := strings.TrimSpace(proxy.Server)
	if !proxy.Enabled || server == "" {
		return ""
	}
	if strings.Contains(server, "://") {
		return server
	}
	if proxy.Port > 0 && !strings.Contains(server, ":") {
		server = fmt.Sprintf("%s:%d", server, proxy.Port)
	}
	return "http://" + server
}

func flowVisibleKeywords(keywords []string) string {
	var visible []string
	for _, keyword := range keywords {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" || keyword == "*" {
			continue
		}
		visible = append(visible, keyword)
	}
	return strings.Join(visible, ", ")
}
