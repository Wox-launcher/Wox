package launcher

import (
	"runtime"
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestSettingsSearchPrimaryFSelectsExistingText(t *testing.T) {
	deps, _ := newSearchControllerDeps()
	app := &App{
		settingsSearch: newSettingsSearchController(deps),
		pluginSettings: newPluginSettingsController(deps),
		themeSettings:  newThemeSettingsController(deps),
	}
	app.settingsSearch.SetEditor(woxwidget.NewTextEditingController("existing"))

	modifier := woxui.KeyModifierControl
	if runtime.GOOS == "darwin" {
		modifier = woxui.KeyModifierMeta
	}
	if !app.onSettingsSearchKey(woxui.KeyEvent{Key: "f", Modifiers: modifier, Down: true}) {
		t.Fatal("primary+F was not handled")
	}
	state := app.settingsSearch.Editor().State()
	if !app.settingsSearch.Focused() || state.Selection != (woxui.TextSelection{Anchor: 0, Focus: 8}) {
		t.Fatalf("settings search focus/selection = %v/%+v, want focused with existing text selected", app.settingsSearch.Focused(), state.Selection)
	}
}

func TestSettingsSearchKeepsPrintableKeysAfterOCRModelManagerCloses(t *testing.T) {
	app := newApp(false, nil, woxui.NewWindowManager(), newAppInstanceRegistry(), nil, true, "", launcherWindowID)
	defer app.cancel()
	app.settingsOpen = true
	app.settingTab = "plugins"
	app.settingsSearch.SetEditor(woxwidget.NewTextEditingController(""))
	fields := newFormFieldsState([]formDefinition{{Type: "ocrModel"}}, nil, true)
	setFormFieldsFocusLocked(&fields, 0)
	app.pluginSettings.SetForm(&pluginSettingsFormState{formFieldsState: fields})
	app.settingsSearch.SetFocused(true)

	if app.onSettingsKey(woxui.KeyEvent{Key: "s", Down: true}) {
		t.Fatal("active OCR field consumed a printable key owned by settings search")
	}
}

func TestSettingsSearchPluginResultKeepsPluginIcon(t *testing.T) {
	icon := woxImage{ImageType: "emoji", ImageData: "P"}
	app := &App{}
	results := app.settingsSearchResults(settingsSnapshot{
		search: settingsSearchSnapshot{
			Query:   woxui.TextEditingState{Text: "UniqueIconPlugin"},
			Plugins: []pluginSettingsPlugin{{ID: "unique-icon-plugin", Name: "UniqueIconPlugin", Icon: icon}},
		},
	})

	if len(results) != 1 || results[0].kind != settingsSearchPlugin {
		t.Fatalf("plugin search results = %#v, want one plugin result", results)
	}
	if results[0].icon != icon {
		t.Fatalf("plugin search icon = %#v, want %#v", results[0].icon, icon)
	}
}

func TestSettingsSearchMatchesPluginEnglishName(t *testing.T) {
	app := &App{}
	results := app.settingsSearchResults(settingsSnapshot{
		search: settingsSearchSnapshot{
			Query:   woxui.TextEditingState{Text: "File Search"},
			Plugins: []pluginSettingsPlugin{{ID: "file-search", Name: "文件搜索", NameEn: "File Search"}},
		},
	})

	for _, result := range results {
		if result.kind == settingsSearchPlugin && result.pluginID == "file-search" {
			return
		}
	}
	t.Fatalf("english plugin name missing from results: %#v", results)
}

func TestSettingsSearchMatchesLocalizedBuiltInSetting(t *testing.T) {
	app := &App{translations: map[string]string{
		"ui_general": "通用",
		"ui_lang":    "语言",
	}}
	results := app.settingsSearchResults(settingsSnapshot{
		search: settingsSearchSnapshot{Query: woxui.TextEditingState{Text: "语言"}},
	})

	for _, result := range results {
		if result.kind == settingsSearchSetting && result.settingKey == "LangCode" {
			if result.title != "语言" || result.subtitle != "通用" {
				t.Fatalf("localized language result = %#v, want title 语言 and subtitle 通用", result)
			}
			return
		}
	}
	t.Fatalf("localized language setting missing from results: %#v", results)
}

func TestSettingsSearchResultIconSourcesMatchFlutterKinds(t *testing.T) {
	settingIcon := settingsSearchResultIconSource(settingsSearchSetting)
	pluginIcon := settingsSearchResultIconSource(settingsSearchPlugin)
	pluginSettingIcon := settingsSearchResultIconSource(settingsSearchPluginSetting)

	if settingIcon.ImageData == "" || pluginIcon.ImageData == "" || pluginSettingIcon.ImageData == "" {
		t.Fatalf("search result fallback icons = setting:%v plugin:%v plugin setting:%v", settingIcon.ImageData != "", pluginIcon.ImageData != "", pluginSettingIcon.ImageData != "")
	}
	if settingIcon.ImageData == pluginIcon.ImageData || settingIcon.ImageData == pluginSettingIcon.ImageData || pluginIcon.ImageData == pluginSettingIcon.ImageData {
		t.Fatal("search result kinds unexpectedly share one fallback icon")
	}
	for name, icon := range map[string]woxImage{"setting": settingIcon, "plugin": pluginIcon, "plugin setting": pluginSettingIcon} {
		if _, err := decodeWoxImage(icon); err != nil {
			t.Fatalf("decode %s search icon: %v", name, err)
		}
	}
}

func TestSettingsSearchHighlightTargetsMatchDestinationKinds(t *testing.T) {
	tests := []struct {
		result settingsSearchResult
		want   string
	}{
		{result: settingsSearchResult{kind: settingsSearchSetting, settingKey: "MainHotkey"}, want: "built-in:MainHotkey"},
		{result: settingsSearchResult{kind: settingsSearchPlugin, pluginID: "clipboard"}, want: "plugin:clipboard"},
		{result: settingsSearchResult{kind: settingsSearchPluginSetting, pluginID: "clipboard", settingKey: "HistoryLimit"}, want: "plugin-setting:clipboard\x00HistoryLimit"},
	}

	for _, test := range tests {
		if got := settingsSearchHighlightTarget(test.result); got != test.want {
			t.Fatalf("highlight target = %q, want %q", got, test.want)
		}
	}
}

func TestStartSettingsSearchHighlightReplacesPreviousCue(t *testing.T) {
	app := &App{}
	app.startSettingsSearchHighlight("built-in:MainHotkey")
	defer app.clearSettingsSearchHighlight()
	firstTimer := app.settingFlashTimer

	app.startSettingsSearchHighlight("plugin:clipboard")
	if app.settingFlash != "plugin:clipboard" {
		t.Fatalf("highlight target = %q, want latest destination", app.settingFlash)
	}
	if app.settingFlashTimer == nil || app.settingFlashTimer == firstTimer {
		t.Fatal("highlight timer was not replaced")
	}
}
