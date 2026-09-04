package app

import (
	"context"
	"encoding/json"
	"testing"
	"wox/plugin"
	"wox/setting/definition"
	"wox/util"
)

type recordingAppAPI struct {
	emptyAPIImpl
	settings         map[string]string
	platformSpecific map[string]bool
	notifications    []string
}

func newRecordingAppAPI() *recordingAppAPI {
	return &recordingAppAPI{
		settings:         map[string]string{},
		platformSpecific: map[string]bool{},
	}
}

func (r *recordingAppAPI) GetSetting(_ context.Context, key string) string {
	return r.settings[key]
}

func (r *recordingAppAPI) SetSetting(_ context.Context, option plugin.SetSettingOption) plugin.SetSettingResult {
	r.settings[option.Key] = option.Value
	r.platformSpecific[option.Key] = option.PlatformSpecific
	return plugin.SetSettingResult{Success: true}
}

func (r *recordingAppAPI) Notify(_ context.Context, message string) {
	r.notifications = append(r.notifications, message)
}

func TestIgnoreRulesSettingIsPlatformSpecific(t *testing.T) {
	var found bool
	for _, item := range (&ApplicationPlugin{}).GetMetadata().SettingDefinitions {
		value, ok := item.Value.(*definition.PluginSettingValueTable)
		if !ok || value.Key != ignoreRulesSettingKey {
			continue
		}
		found = true
		if !item.IsPlatformSpecific {
			t.Fatal("ignore rules must be platform specific")
		}
		if !value.EnableSearch || value.SearchColumnKey != "Pattern" {
			t.Fatalf("ignore rules search = %+v", value)
		}
		if len(value.Columns) != 3 || !value.Columns[0].PreviewMatchedApps || !value.Columns[1].HideInTable || !value.Columns[2].HideInUpdate {
			t.Fatalf("ignore rules columns = %+v", value.Columns)
		}
		if value.Columns[0].Key != "Pattern" || value.Columns[1].Key != "IncludeFuture" || value.Columns[2].Key != "Apps" {
			t.Fatalf("ignore rules column keys = %+v", value.Columns)
		}
		if value.Columns[0].Width != 160 || value.Columns[2].Width != 0 {
			t.Fatalf("ignore rules column widths = pattern=%d apps=%d", value.Columns[0].Width, value.Columns[2].Width)
		}
	}
	if !found {
		t.Fatal("ignore rules setting not found")
	}
}

func TestIgnoreRulesSettingDoesNotAddHiddenAppsTable(t *testing.T) {
	for _, item := range (&ApplicationPlugin{}).GetMetadata().SettingDefinitions {
		value, ok := item.Value.(*definition.PluginSettingValueTable)
		if ok && value.Key == "IgnoredApps" {
			t.Fatal("hidden apps must stay inside IgnoreRules")
		}
	}
}

func TestIgnoredAppMatchingPrefersPathForDuplicateShortcuts(t *testing.T) {
	ignored := []ignoredApp{{
		Name:     "Chrome",
		Identity: "chrome.exe",
		Path:     `C:\Users\me\Desktop\Chrome.lnk`,
	}}

	if !isIgnoredApp(appInfo{Name: "Chrome", Identity: "chrome.exe", Path: `C:\Users\me\Desktop\Chrome.lnk`}, ignored) {
		t.Fatal("expected the hidden shortcut path to match")
	}
	if util.IsWindows() && !isIgnoredApp(appInfo{Name: "Chrome", Identity: "chrome.exe", Path: `c:\users\me\desktop\chrome.lnk`}, ignored) {
		t.Fatal("expected Windows shortcut paths to match case-insensitively")
	}
	if isIgnoredApp(appInfo{
		Name:     "Chrome Start Menu",
		Identity: "chrome.exe",
		Path:     `C:\ProgramData\Microsoft\Windows\Start Menu\Programs\Chrome.lnk`,
	}, ignored) {
		t.Fatal("a second shortcut to the same executable must stay visible")
	}
}

func TestIgnoredAppMatchingFallsBackToIdentityWithoutPath(t *testing.T) {
	ignored := []ignoredApp{{
		Name:     "Display",
		Identity: "ms-settings:display",
	}}

	if !isIgnoredApp(appInfo{Name: "Display", Identity: "ms-settings:display", Type: AppTypeWindowsSetting}, ignored) {
		t.Fatal("expected pathless settings entries to match by identity")
	}
	if isIgnoredApp(appInfo{Name: "Sound", Identity: "ms-settings:sound", Type: AppTypeWindowsSetting}, ignored) {
		t.Fatal("unexpected identity match")
	}
}

func TestRebuildQueryEntriesSkipsIgnoredApps(t *testing.T) {
	plugin := &ApplicationPlugin{
		apps: []appInfo{
			{Name: "Chrome", Path: `C:\Users\me\Desktop\Chrome.lnk`, Identity: "chrome.exe"},
			{Name: "Notes", Path: `C:\Apps\Notes.exe`, Identity: "notes.exe"},
		},
		ignoredApps: []ignoredApp{
			{Name: "Chrome", Path: `C:\Users\me\Desktop\Chrome.lnk`, Identity: "chrome.exe"},
		},
	}

	plugin.rebuildQueryEntries(context.Background())
	if len(plugin.queryEntries) != 1 || plugin.queryEntries[0].info.Name != "Notes" {
		t.Fatalf("query entries = %+v, want only Notes", plugin.queryEntries)
	}
}

func TestBuildAppActionsIncludesHideAction(t *testing.T) {
	actions := (&ApplicationPlugin{}).buildAppActions(appInfo{Name: "Notes", Path: `C:\Apps\Notes.exe`}, "Notes", nil)
	for _, action := range actions {
		if action.Name == "i18n:plugin_app_hide" {
			return
		}
	}
	t.Fatal("expected hide from search action")
}

func TestHideAppFromSearchSavesPlatformSpecificSetting(t *testing.T) {
	api := newRecordingAppAPI()
	plugin := &ApplicationPlugin{api: api}
	info := appInfo{Name: "Notes", Path: `C:\Apps\Notes.exe`, Identity: "notes.exe"}

	plugin.hideAppFromSearch(context.Background(), info, "Notes")

	if !api.platformSpecific[ignoreRulesSettingKey] {
		t.Fatal("hidden apps must be saved as a platform-specific ignore rule")
	}
	rules, err := parseIgnoreRules(api.settings[ignoreRulesSettingKey])
	if err != nil {
		t.Fatalf("parse saved setting: %v", err)
	}
	if len(rules) != 1 || rules[0].IncludeFuture || len(rules[0].Apps) != 1 || rules[0].Apps[0].Path != info.Path || rules[0].Apps[0].Identity != info.Identity {
		t.Fatalf("saved rules = %+v", rules)
	}
	if rules[0].Pattern != "Notes" {
		t.Fatalf("saved pattern = %q, want Notes", rules[0].Pattern)
	}
	if len(api.notifications) != 1 || api.notifications[0] != "i18n:plugin_app_hide_completed" {
		t.Fatalf("notifications = %v", api.notifications)
	}

	plugin.hideAppFromSearch(context.Background(), info, "Notes")
	if len(api.notifications) != 2 || api.notifications[1] != "i18n:plugin_app_already_hidden" {
		t.Fatalf("duplicate hide notifications = %v", api.notifications)
	}

	var rows []appIgnoreRule
	if err := json.Unmarshal([]byte(api.settings[ignoreRulesSettingKey]), &rows); err != nil {
		t.Fatalf("decode saved rows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("duplicate hide should not append another row: %+v", rows)
	}
}

func TestHideAppFromSearchSkipsAppsAlreadyCoveredByPattern(t *testing.T) {
	api := newRecordingAppAPI()
	api.settings[ignoreRulesSettingKey] = `[{"Pattern":"*.lnk"}]`
	plugin := &ApplicationPlugin{api: api}
	info := appInfo{Name: "Notes", Path: `C:\Apps\Notes.lnk`, Identity: "notes.exe"}

	plugin.hideAppFromSearch(context.Background(), info, "Notes")
	if len(api.notifications) != 1 || api.notifications[0] != "i18n:plugin_app_already_hidden" {
		t.Fatalf("notifications = %v", api.notifications)
	}
	rules, err := parseIgnoreRules(api.settings[ignoreRulesSettingKey])
	if err != nil {
		t.Fatalf("parse setting: %v", err)
	}
	if len(rules) != 1 || !rules[0].IncludeFuture || len(rules[0].Apps) != 0 {
		t.Fatalf("pattern hide should not add an app row: %+v", rules)
	}
}
