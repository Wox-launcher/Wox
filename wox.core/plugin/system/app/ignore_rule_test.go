package app

import (
	"context"
	"testing"
)

func TestParseIgnoreRulesMigratesLegacyPatternOnlyRows(t *testing.T) {
	rules, err := parseIgnoreRules(`[{"Pattern":"*uninstall*"},{"Pattern":"notepad"}]`)
	if err != nil {
		t.Fatalf("parse ignore rules: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("parsed rules = %+v", rules)
	}
	for _, rule := range rules {
		if !rule.IncludeFuture || len(rule.Apps) != 0 {
			t.Fatalf("legacy row should become a dynamic rule: %+v", rule)
		}
	}
}

func TestParseIgnoreRulesKeepsExplicitApps(t *testing.T) {
	rules, err := parseIgnoreRules(`[
		{"Pattern":"Chrome","IncludeFuture":false,"Apps":[{"Name":"Chrome","Identity":"chrome.exe","Path":"C:\\\\Users\\\\me\\\\Desktop\\\\Chrome.lnk"}]}
	]`)
	if err != nil {
		t.Fatalf("parse ignore rules: %v", err)
	}
	if len(rules) != 1 || rules[0].IncludeFuture || len(rules[0].Apps) != 1 {
		t.Fatalf("parsed rules = %+v", rules)
	}
	if rules[0].Apps[0].Name != "Chrome" || rules[0].Apps[0].Identity != "chrome.exe" {
		t.Fatalf("app = %+v", rules[0].Apps[0])
	}
}

func TestParseIgnoreRulesRejectsMalformedJSON(t *testing.T) {
	if _, err := parseIgnoreRules("["); err == nil {
		t.Fatal("expected malformed setting to fail")
	}
}

func TestNormalizeAppIgnoreRulesDedupesDynamicPatternsAndApps(t *testing.T) {
	desktop := ignoredApp{Name: "Chrome", Identity: "chrome.exe", Path: `C:\Users\me\Desktop\Chrome.lnk`}
	startMenu := ignoredApp{Name: "Chrome", Identity: "chrome.exe", Path: `C:\ProgramData\Microsoft\Windows\Start Menu\Programs\Chrome.lnk`}
	rules := normalizeAppIgnoreRules([]appIgnoreRule{
		{Pattern: "Chrome", IncludeFuture: false, Apps: []ignoredApp{desktop, desktop, startMenu}},
		{Pattern: "*uninstall*", IncludeFuture: true},
		{Pattern: "*UNINSTALL*", IncludeFuture: true},
		{Pattern: "empty", IncludeFuture: false},
	})
	if len(rules) != 2 {
		t.Fatalf("normalized = %+v, want one Chrome list and one pattern", rules)
	}
	if rules[0].IncludeFuture || len(rules[0].Apps) != 2 {
		t.Fatalf("chrome rule = %+v", rules[0])
	}
	if !rules[1].IncludeFuture || rules[1].Pattern != "*uninstall*" {
		t.Fatalf("pattern rule = %+v", rules[1])
	}
}

func TestSplitIgnoreRulesDoesNotUseFixedAppsPatternAsWildcard(t *testing.T) {
	chrome := ignoredApp{Name: "Chrome", Identity: "chrome.exe", Path: `C:\Users\me\Desktop\Chrome.lnk`}
	matchers, apps := splitIgnoreRules([]appIgnoreRule{
		{Pattern: "Chrome", IncludeFuture: false, Apps: []ignoredApp{chrome}},
	})
	if len(matchers) != 0 {
		t.Fatalf("fixed-app rows must not compile as wildcards: %+v", matchers)
	}
	if len(apps) != 1 || apps[0].Path != chrome.Path {
		t.Fatalf("apps = %+v", apps)
	}
}

func TestBuildIgnoreRuleCandidatesMatchLocalizedNotepad(t *testing.T) {
	info := appInfo{
		Name:     "记事本",
		Path:     `shell:AppsFolder\Microsoft.WindowsNotepad_8wekyb3d8bbwe!App`,
		Identity: "Microsoft.WindowsNotepad_8wekyb3d8bbwe!App",
		Type:     AppTypeUWP,
	}
	if !AppMatchesIgnorePattern("notepad", buildIgnoreRuleCandidates(info, "记事本")...) {
		t.Fatal("localized UWP notepad should match pattern notepad")
	}
}

func TestIgnoreRulePreviewMatchesCoreSearchAliases(t *testing.T) {
	info := appInfo{Name: "Display", Path: "ms-settings:display", SearchableNames: []string{"brightness night light"}}
	app := &ApplicationPlugin{apps: []appInfo{info}}
	preview := app.indexedAppsForPreview(context.Background(), "brightness")
	if len(preview) != 1 || preview[0].Name != info.Name {
		t.Fatalf("alias preview = %+v", preview)
	}
	if !ignoreRuleHidesApp(info, info.Name, []appIgnoreRule{{Pattern: "brightness", IncludeFuture: true}}) {
		t.Fatal("preview and backend filtering disagree")
	}
}

func TestAppMatchesIgnorePatternTreatsBareTextAsContains(t *testing.T) {
	if AppMatchesIgnorePattern("app", "Notes", "") {
		t.Fatal("bare app must not match Notes")
	}
	if !AppMatchesIgnorePattern("app", "MyApp", "") {
		t.Fatal("bare app should contain-match MyApp")
	}
	if !AppMatchesIgnorePattern("notepad", "记事本", `shell:AppsFolder\Microsoft.WindowsNotepad_8wekyb3d8bbwe!App`) {
		t.Fatal("bare notepad should contain-match the UWP path")
	}
	if !AppMatchesIgnorePattern("*.lnk", "Notes", `C:\Apps\Notes.lnk`) {
		t.Fatal("expected path wildcard match")
	}
	if !AppMatchesIgnorePattern("*uninstall*", "Uninstall Chrome", "") {
		t.Fatal("expected contains match")
	}
}

func TestRebuildIgnoreRuleMatchersLoadsLegacyPatternWithoutWriting(t *testing.T) {
	api := newRecordingAppAPI()
	api.settings[ignoreRulesSettingKey] = `[{"Pattern":"*uninstall*"}]`
	plugin := &ApplicationPlugin{api: api}
	plugin.rebuildIgnoreRuleMatchers(context.Background())
	if len(plugin.ignoreMatchers) != 1 || plugin.ignoreMatchers[0].pattern != "*uninstall*" {
		t.Fatalf("matchers = %+v", plugin.ignoreMatchers)
	}
	if api.platformSpecific[ignoreRulesSettingKey] {
		t.Fatal("plugin load must not persist IgnoreRules migration")
	}
	if api.settings[ignoreRulesSettingKey] != `[{"Pattern":"*uninstall*"}]` {
		t.Fatalf("setting was rewritten: %s", api.settings[ignoreRulesSettingKey])
	}
}

func TestRebuildIgnoreRuleMatchersDoesNotRewriteMigratedSetting(t *testing.T) {
	api := newRecordingAppAPI()
	api.settings[ignoreRulesSettingKey] = `[{"Pattern":"Notes","IncludeFuture":false,"Apps":[{"Name":"Notes","Identity":"notes.exe","Path":"C:\\\\Apps\\\\Notes.exe"}]}]`
	plugin := &ApplicationPlugin{api: api}
	plugin.rebuildIgnoreRuleMatchers(context.Background())
	if len(plugin.ignoreMatchers) != 0 {
		t.Fatalf("matchers = %+v", plugin.ignoreMatchers)
	}
	if len(plugin.ignoredApps) != 1 || plugin.ignoredApps[0].Identity != "notes.exe" {
		t.Fatalf("ignored apps = %+v", plugin.ignoredApps)
	}
	if api.platformSpecific[ignoreRulesSettingKey] {
		t.Fatal("already migrated setting must not be rewritten")
	}
}

func TestIndexedAppsForPreviewKeepsDuplicateShortcutPaths(t *testing.T) {
	plugin := &ApplicationPlugin{
		apps: []appInfo{
			{Name: "Chrome", Identity: "chrome.exe", Path: `C:\Users\me\Desktop\Chrome.lnk`},
			{Name: "Chrome", Identity: "chrome.exe", Path: `C:\ProgramData\Microsoft\Windows\Start Menu\Programs\Chrome.lnk`},
		},
	}

	apps := plugin.indexedAppsForPreview(context.Background(), "*")
	if len(apps) != 2 {
		t.Fatalf("indexed apps = %+v, want both shortcuts", apps)
	}
}

type translatingAPI struct {
	emptyAPIImpl
	translations map[string]string
}

func (t translatingAPI) GetTranslation(_ context.Context, key string) string {
	if value, ok := t.translations[key]; ok {
		return value
	}
	return key
}

func TestIndexedAppsForPreviewTranslatesI18nNames(t *testing.T) {
	plugin := &ApplicationPlugin{
		api: translatingAPI{translations: map[string]string{"i18n:plugin_app_settings": "Display"}},
		apps: []appInfo{
			{Name: "i18n:plugin_app_settings", Identity: "ms-settings:display"},
		},
	}

	apps := plugin.indexedAppsForPreview(context.Background(), "*")
	if len(apps) != 1 || apps[0].Name != "Display" {
		t.Fatalf("indexed apps = %+v", apps)
	}
}

func TestIgnoreRulesNeedMigrationRequiresMissingKey(t *testing.T) {
	if !ignoreRulesNeedMigration(`[{"Pattern":"notepad"}]`) {
		t.Fatal("legacy pattern-only JSON should migrate")
	}
	if ignoreRulesNeedMigration(`[{"Pattern":"notepad","IncludeFuture":false,"Apps":[]}]`) {
		t.Fatal("explicit IncludeFuture must not migrate")
	}
}
