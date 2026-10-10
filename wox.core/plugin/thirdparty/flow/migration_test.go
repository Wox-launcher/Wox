package flow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wox/plugin/thirdparty/flow/manifest"
	"wox/plugin/thirdparty/migrate"
	"wox/util"
)

func TestFlowMigrationDetectsUserPluginsAndHotkey(t *testing.T) {
	isolateFlowMigrationHome(t)
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	writeMigrationFile(t, filepath.Join(root, "FlowLauncher", "Settings", "Settings.json"), `{
		"Hotkey": "Alt + Space",
		"PluginSettings": {"Plugins": {"hello": {"ID": "hello", "Disabled": true, "ActionKeywords": ["hey"]}}}
	}`)
	writeMigrationFile(t, filepath.Join(root, "FlowLauncher", "Plugins", "Hello", "plugin.json"), `{
		"ID": "hello", "Name": "Hello", "Language": "python", "ExecuteFileName": "main.py", "Description": "Says hello", "Version": "1.2.0", "ActionKeyword": "hi", "IcoPath": "icon.png"
	}`)
	writeMigrationFile(t, filepath.Join(root, "FlowLauncher", "Plugins", "Hello", "main.py"), "print('hi')\n")
	writeMigrationFile(t, filepath.Join(root, "FlowLauncher", "Plugins", "Hello", "icon.png"), "png")
	writeMigrationFile(t, filepath.Join(root, "FlowLauncher", "Plugins", "Old", "plugin.json"), `{
		"ID": "old", "Name": "Old", "Language": "unknown", "ExecuteFileName": "main.exe"
	}`)

	userData := t.TempDir()
	previous := util.GetLocation().GetUserDataDirectory()
	util.GetLocation().UpdateUserDataDirectory(userData)
	t.Cleanup(func() { util.GetLocation().UpdateUserDataDirectory(previous) })
	if err := os.MkdirAll(filepath.Join(manifest.CollectionDirectory(), "taken"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeMigrationFile(t, filepath.Join(root, "FlowLauncher", "Plugins", "Taken", "plugin.json"), `{
		"ID": "taken", "Name": "Taken", "Language": "python", "ExecuteFileName": "main.py", "IcoPath": "icon.png"
	}`)
	writeMigrationFile(t, filepath.Join(root, "FlowLauncher", "Plugins", "Taken", "main.py"), "print('taken')\n")
	writeMigrationFile(t, filepath.Join(root, "FlowLauncher", "Plugins", "Taken", "icon.png"), "png")

	installation, err := (flowMigrationSource{}).Detect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if installation == nil || installation.Name() != "Flow Launcher" || installation.Hotkey() != "Alt + Space" {
		t.Fatalf("installation %#v", installation)
	}
	plugins, err := installation.Plugins(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]migrate.Plugin{}
	for _, item := range plugins {
		byID[item.ID] = item
	}
	hello := byID["hello"]
	if !hello.Selectable || hello.Status != migrate.PluginReady || hello.Version != "1.2.0" || len(hello.Keywords) != 1 || hello.Keywords[0] != "hey" || !strings.HasSuffix(filepath.ToSlash(hello.Icon.ImageData), "Plugins/Hello/icon.png") {
		t.Fatalf("hello %#v", hello)
	}
	if byID["old"].Selectable || byID["old"].Status != migrate.PluginUnsupported {
		t.Fatalf("old %#v", byID["old"])
	}
	taken := byID["taken"]
	if taken.Selectable || taken.Status != migrate.PluginImported || taken.Icon.ImageType != "absolute" || !strings.HasSuffix(filepath.ToSlash(taken.Icon.ImageData), "Plugins/Taken/icon.png") {
		t.Fatalf("taken %#v", taken)
	}

	result, err := installation.Import(context.Background(), []string{"hello", "old"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Plugins) != 2 || result.Plugins[0].Error != "" || result.Plugins[1].Error == "" {
		t.Fatalf("import %#v", result.Plugins)
	}
	copied, err := os.ReadFile(filepath.Join(manifest.CollectionDirectory(), "hello", "main.py"))
	if err != nil {
		t.Fatal(err)
	}
	if string(copied) != "print('hi')\n" {
		t.Fatalf("copied %q", copied)
	}
}

func TestFlowMigrationDetectsScoopPortableUserData(t *testing.T) {
	home := t.TempDir()
	isolateFlowMigrationHome(t)
	t.Setenv("USERPROFILE", home)
	t.Setenv("APPDATA", t.TempDir())
	userData := filepath.Join(home, "scoop", "apps", "flow-launcher", "current", "app-2.1.4", "UserData")
	writeMigrationFile(t, filepath.Join(userData, "Settings", "Settings.json"), `{"Hotkey":"Alt + Space"}`)
	writeMigrationFile(t, filepath.Join(userData, "Plugins", "Timer", "plugin.json"), `{
		"ID": "timer", "Name": "Timer", "Language": "python", "ExecuteFileName": "main.py"
	}`)
	writeMigrationFile(t, filepath.Join(userData, "Plugins", "Timer", "main.py"), "print('timer')\n")
	older := filepath.Join(home, "scoop", "apps", "flow-launcher", "current", "app-2.0.0", "UserData")
	writeMigrationFile(t, filepath.Join(older, "Settings", "Settings.json"), `{"Hotkey":"Ctrl + Space"}`)

	installation, err := (flowMigrationSource{}).Detect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if installation == nil || installation.Location() != userData || installation.Hotkey() != "Alt + Space" {
		t.Fatalf("installation %#v", installation)
	}
	plugins, err := installation.Plugins(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 1 || plugins[0].ID != "timer" || !plugins[0].Selectable {
		t.Fatalf("plugins %#v", plugins)
	}
}

func TestFlowMigrationCatalogGroupsSettingsAndPlugins(t *testing.T) {
	isolateFlowMigrationHome(t)
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	writeMigrationFile(t, filepath.Join(root, "FlowLauncher", "Settings", "Settings.json"), `{
		"Hotkey": "Ctrl + Shift + Space",
		"OpenContextMenuHotkey": "Ctrl+O",
		"IgnoreHotkeysOnFullscreen": true,
		"CustomPluginHotkeys": [{"Hotkey": "Ctrl + Alt + V", "ActionKeyword": "cb"}],
		"CustomShortcuts": [{"Key": "wi", "Value": "wpm install"}],
		"LastQueryMode": "Selected",
		"ShowHistoryResultsForHomePage": true,
		"ShouldUsePinyin": true,
		"AlwaysStartEn": false,
		"StartFlowLauncherOnSystemStartup": false,
		"HideOnStartup": true,
		"HideWhenDeactivated": true,
		"HideNotifyIcon": false,
		"WindowSize": 580,
		"MaxResultsToShow": 5,
		"SearchWindowScreen": "Cursor",
		"Language": "zh-cn",
		"QueryBoxFont": "Noto Sans SC",
		"AutoUpdates": true,
		"Proxy": {"Enabled": true, "Server": "127.0.0.1", "Port": 7890},
		"PluginSettings": {"PythonExecutablePath": "C:\\Python\\python.exe", "NodeExecutablePath": "C:\\Node\\node.exe"}
	}`)
	writeMigrationFile(t, filepath.Join(root, "FlowLauncher", "Plugins", "Timer", "plugin.json"), `{
		"ID": "timer", "Name": "Timer", "Language": "python", "ExecuteFileName": "main.py", "ActionKeyword": "t"
	}`)
	writeMigrationFile(t, filepath.Join(root, "FlowLauncher", "Plugins", "Timer", "main.py"), "print('t')\n")

	installation, err := (flowMigrationSource{}).Detect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	categories, err := installation.Catalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(categories) != 4 || categories[0].ID != migrate.CategoryHotkeys || categories[1].ID != migrate.CategoryQueries || categories[2].ID != migrate.CategoryGeneral || categories[3].ID != migrate.CategoryPlugins {
		t.Fatalf("categories %#v", categoryIDs(categories))
	}
	hotkeys := itemsByID(categories[0].Items)
	if hotkeys["setting:main_hotkey"].Detail != "Ctrl + Shift + Space" || hotkeys["setting:query_hotkey:0"].Detail != "Ctrl + Alt + V → cb" {
		t.Fatalf("hotkeys %#v", hotkeys)
	}
	if hotkeys["setting:ignore_fullscreen"].DetailKey != migrate.ValueOn {
		t.Fatalf("fullscreen %#v", hotkeys["setting:ignore_fullscreen"])
	}
	queries := itemsByID(categories[1].Items)
	if queries["setting:query_alias:0"].Detail != "wi → wpm install" || queries["setting:launch_mode"].DetailKey != migrate.ValueLaunchContinue || queries["setting:pinyin"].DetailKey != migrate.ValueOn {
		t.Fatalf("queries %#v", queries)
	}
	general := itemsByID(categories[2].Items)
	if general["setting:app_width"].Detail != "580" || general["setting:language"].Detail != "zh_CN" || general["setting:show_position"].DetailKey != migrate.ValuePositionCursor || general["setting:proxy"].Detail != "http://127.0.0.1:7890" {
		t.Fatalf("general %#v", general)
	}
	if general["setting:show_tray"].DetailKey != migrate.ValueOn || general["setting:autostart"].DetailKey != migrate.ValueOff {
		t.Fatalf("toggles %#v", general)
	}
	plugins := categories[3].Items
	if len(plugins) != 1 || plugins[0].ID != "plugin:timer" || !plugins[0].Selectable || !strings.Contains(plugins[0].Detail, "t") {
		t.Fatalf("plugins %#v", plugins)
	}

	result, err := installation.Import(context.Background(), []string{"setting:main_hotkey", "setting:query_hotkey:0", "setting:query_alias:0", "setting:pinyin"})
	if err != nil {
		t.Fatal(err)
	}
	writes := map[string]migrate.SettingWrite{}
	for _, write := range result.Settings {
		writes[write.Key] = write
	}
	if writes["MainHotkey"].Value != "Ctrl+Shift+Space" || writes["UsePinYin"].Value != "true" {
		t.Fatalf("writes %#v", writes)
	}
	if !strings.Contains(writes["QueryHotkeys"].Value, `"Hotkey":"Ctrl+Alt+V"`) || !strings.Contains(writes["QueryAliases"].Value, `"Shortcut":"wi"`) {
		t.Fatalf("lists %#v", writes)
	}
	if len(result.Plugins) != 0 {
		t.Fatalf("plugin import %#v", result.Plugins)
	}
}

func categoryIDs(categories []migrate.Category) []string {
	ids := make([]string, len(categories))
	for index, category := range categories {
		ids[index] = category.ID
	}
	return ids
}

func itemsByID(items []migrate.Item) map[string]migrate.Item {
	byID := map[string]migrate.Item{}
	for _, item := range items {
		byID[item.ID] = item
	}
	return byID
}

func TestFlowMigrationRegistrationIgnoresAnEmptyDataDirectory(t *testing.T) {
	isolateFlowMigrationHome(t)
	t.Setenv("APPDATA", t.TempDir())
	found, err := migrate.Detect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Fatalf("found %d installations", len(found))
	}
}

func isolateFlowMigrationHome(t *testing.T) {
	t.Helper()
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("LOCALAPPDATA", t.TempDir())
}

func writeMigrationFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
