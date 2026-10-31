package launcher

import (
	"fmt"
	"strings"
	"testing"

	woxcomponent "wox/ui/launcher/component"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestSettingsRailKeepsCachedIconWhileSelectedTintLoads(t *testing.T) {
	windows := woxui.NewWindowManager()
	app := newApp(false, nil, windows, newAppInstanceRegistry(), nil, true, "", launcherWindowID)
	defer app.cancel()
	app.uiCall = func(callback func()) error {
		callback()
		return nil
	}
	palette := settingsPalette()
	cacheKey := func(source woxImage, tint woxui.Color, size int) string {
		return fmt.Sprintf("%s-svg-%d-tint-%02x%02x%02x%02x", imageKey(source), size, tint.R, tint.G, tint.B, tint.A)
	}
	for _, spec := range settingNavSpecs(false) {
		source := settingNavIconSource(spec.id)
		if source.ImageData == "" {
			continue
		}
		icon := &woxui.Image{}
		app.imageStore.images[cacheKey(source, palette.TextSecondary, 18)] = icon
	}
	selectedSource := settingNavIconSource("ui")
	normalIcon := app.imageStore.images[cacheKey(selectedSource, palette.TextSecondary, 18)]
	app.imageStore.imageRequested[cacheKey(selectedSource, palette.SelectionText, 18)] = selectedSource.ImageData
	searchSource := settingControlIconSource("search")
	app.imageStore.images[cacheKey(searchSource, palette.TextSecondary, 18)] = &woxui.Image{}

	rail := app.buildSettingsRail(settingsSnapshot{tab: "appearance", palette: palette}, 260, 600, 1).(woxwidget.Stack)
	railContainer := rail.Children[0].Child.(woxwidget.Container)
	railContent := railContainer.Child.(woxwidget.LayoutBuilder).Build(woxui.Size{
		Width: railContainer.Width - railContainer.Padding.Left - railContainer.Padding.Right, Height: railContainer.Height - railContainer.Padding.Top - railContainer.Padding.Bottom,
	}).(woxwidget.Flex)
	navigation := railContent.Children[1].(woxwidget.Stack)
	scroll := navigation.Children[0].Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	rows := scroll.Content.(woxwidget.Flex)
	stateful := rows.Children[1].(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Stateful)
	row := stateful.CreateState().Build(woxwidget.StateContext{}, stateful.Widget).(woxwidget.Gesture).Child.(woxwidget.Container)
	icon := row.Child.(woxwidget.Align).Child.(woxwidget.Flex).Children[0].(woxwidget.Align).Child.(woxwidget.Image)

	if icon.Source != normalIcon {
		t.Fatalf("selected navigation icon = %p, want cached SVG %p while the selected tint loads", icon.Source, normalIcon)
	}
}

func TestSettingsSearchSelectedBuiltInIconUsesSelectedTextColor(t *testing.T) {
	windows := woxui.NewWindowManager()
	app := newApp(false, nil, windows, newAppInstanceRegistry(), nil, true, "", launcherWindowID)
	defer app.cancel()
	app.uiCall = func(callback func()) error {
		callback()
		return nil
	}
	palette := settingsPalette()
	palette.SelectionText = woxui.Color{R: 241, G: 242, B: 243, A: 255}
	source := settingsSearchResultIconSource(settingsSearchSetting)
	key := fmt.Sprintf("%s-svg-%d-tint-%02x%02x%02x%02x", imageKey(source), 24, palette.SelectionText.R, palette.SelectionText.G, palette.SelectionText.B, palette.SelectionText.A)
	selectedIcon := &woxui.Image{}
	app.imageStore.images[key] = selectedIcon
	snapshot := settingsSnapshot{palette: palette, search: settingsSearchSnapshot{Query: woxui.TextEditingState{Text: "font"}}}

	panel := app.buildSettingsSearchResultPanel(snapshot, 240, 200, 1).(woxwidget.Container)
	props := panel.Child.(woxwidget.LayoutBuilder).Build(woxui.Size{
		Width: panel.Width - panel.Padding.Left - panel.Padding.Right, Height: panel.Height - panel.Padding.Top - panel.Padding.Bottom,
	}).(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	row := props.Content.(woxwidget.Flex).Children[0].(woxwidget.Gesture).Child.(woxwidget.Container)
	icon := row.Child.(woxwidget.Align).Child.(woxwidget.Flex).Children[0].(woxwidget.Align).Child.(woxwidget.Image)

	if icon.Source != selectedIcon {
		t.Fatalf("selected settings search icon = %p, want selected text tint %p", icon.Source, selectedIcon)
	}
}

func TestSettingNavPlacesHotkeyAboveAI(t *testing.T) {
	var ids []string
	for _, spec := range settingNavSpecs(false) {
		ids = append(ids, spec.id)
	}
	hotkey := -1
	ai := -1
	for index, id := range ids {
		if id == "hotkey" {
			hotkey = index
		}
		if id == "ai" {
			ai = index
		}
		if id == "network" {
			t.Fatal("network should not remain in the settings sidebar after proxy moved to General")
		}
	}
	if hotkey < 0 || ai < 0 || hotkey != ai-1 {
		t.Fatalf("sidebar order = %v, want hotkey immediately above AI", ids)
	}
	if settingTabForPath("/hotkeys") != "hotkey" {
		t.Fatalf("hotkeys path = %q, want hotkey", settingTabForPath("/hotkeys"))
	}
	if settingTabForPath("/network") != "general" {
		t.Fatalf("legacy network path = %q, want general", settingTabForPath("/network"))
	}
}

func TestGeneralSettingsEndWithProxy(t *testing.T) {
	items := settingItems("general", settingsData{HttpProxyEnabled: true, HttpProxyURL: "http://localhost:7890"})
	if len(items) < 2 {
		t.Fatalf("general items = %d, want proxy settings at the end", len(items))
	}
	enabled := items[len(items)-2]
	url := items[len(items)-1]
	if enabled.key != "HttpProxyEnabled" || enabled.value != "true" {
		t.Fatalf("general proxy switch = %+v, want HttpProxyEnabled=true at the end", enabled)
	}
	if url.key != "HttpProxyUrl" || url.value != "http://localhost:7890" || url.disabled {
		t.Fatalf("general proxy URL = %+v, want enabled HttpProxyUrl at the end", url)
	}
	for _, item := range items {
		if item.key == "IgnoreHotkeysOnFullscreen" {
			t.Fatal("fullscreen hotkey switch should live on the Hotkey page")
		}
		if item.key == "LangCode" {
			t.Fatal("language should live on the UI page")
		}
	}
}

func TestUISettingsStartWithLanguage(t *testing.T) {
	items := settingItems("appearance", settingsData{LangCode: "zh_CN"})
	if len(items) == 0 || items[0].key != "LangCode" || items[0].value != "zh_CN" {
		t.Fatalf("appearance first item = %+v, want LangCode=zh_CN", items)
	}
	app := bindTestApp(&App{language: languageCatalog{translations: map[string]string{"ui_general_section_language": "Language"}}})
	if got := app.settingsSectionLabel("appearance", "LangCode"); got != "Language" {
		t.Fatalf("appearance language section = %q, want Language", got)
	}
}

func TestSettingsSectionLabelMatchesFlutterGrouping(t *testing.T) {
	app := bindTestApp(&App{language: languageCatalog{translations: map[string]string{"ui_update_section_updates": "Updates"}}})

	app.language.translations["ui_network"] = "Network"
	if got := app.settingsSectionLabel("general", "HttpProxyEnabled"); got != "Network" {
		t.Fatalf("general proxy section label = %q, want Network", got)
	}
	if got := app.settingsSectionLabel("debug", "ShowScoreTail"); got != "" {
		t.Fatalf("debug section label = %q, want no group header", got)
	}
	if got := app.settingsSectionLabel("updates", "EnableAutoUpdate"); got != "Updates" {
		t.Fatalf("updates section label = %q, want %q", got, "Updates")
	}
}

func TestHotkeySettingsTablesKeepFlutterOuterGap(t *testing.T) {
	windows := woxui.NewWindowManager()
	app := newApp(false, nil, windows, newAppInstanceRegistry(), nil, true, "", launcherWindowID)
	defer app.cancel()
	app.uiCall = func(callback func()) error {
		callback()
		return nil
	}
	form := newHotkeySettingsForm(settingsData{MainHotkey: "Alt+Space", SelectionHotkey: "Alt+Shift+Space", IsLinuxWaylandSession: false})
	app.hotkeySettings.SetForm(&form)

	page := app.buildSettingsPage(settingsSnapshot{tab: "hotkey", hotkey: app.hotkeySettings.Snapshot(), palette: settingsPalette()}, nil, 800, 600, 1)
	container := page.(woxwidget.Container)
	scroll := container.Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	rows := scroll.Content.(woxwidget.Container).Child.(woxwidget.Flex).Children

	tableSpacers := 0
	var lastTableSpacer woxwidget.Container
	for _, row := range rows {
		keyed, ok := row.(woxwidget.Keyed)
		if !ok {
			continue
		}
		target, ok := keyed.Child.(woxwidget.Container)
		if !ok {
			continue
		}
		spacer, ok := target.Child.(woxwidget.Container)
		if !ok || spacer.Padding.Bottom != 24 {
			continue
		}
		tableSpacers++
		lastTableSpacer = spacer
	}
	if tableSpacers != 3 {
		t.Fatalf("hotkey table spacers = %d, want IgnoredHotkeyApps, ResultBindings, QueryHotkeys", tableSpacers)
	}
	if lastTableSpacer.Padding.Bottom != 24 {
		t.Fatalf("last table outer bottom gap = %v, want Flutter's 24px", lastTableSpacer.Padding.Bottom)
	}
}

func TestHotkeyNavigationSettingFollowsActionHotkey(t *testing.T) {
	app := newApp(false, nil, woxui.NewWindowManager(), newAppInstanceRegistry(), nil, true, "", launcherWindowID)
	defer app.cancel()
	form := newHotkeySettingsForm(settingsData{})
	app.hotkeySettings.SetForm(&form)
	items := settingItems("hotkey", settingsData{})
	page := app.buildSettingsPage(settingsSnapshot{tab: "hotkey", hotkey: app.hotkeySettings.Snapshot(), palette: settingsPalette()}, items, 800, 600, 1)
	scroll := page.(woxwidget.Container).Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	rows := scroll.Content.(woxwidget.Container).Child.(woxwidget.Flex).Children
	keys := map[string]woxwidget.Key{}
	for index, definition := range form.definitions {
		keys[definition.Value.Key] = formFieldRowKey("settings-hotkey", index)
	}
	positions := map[woxwidget.Key]int{}
	for index, row := range rows {
		if keyed, ok := row.(woxwidget.Keyed); ok {
			positions[keyed.Key] = index
		}
	}
	action, actionFound := positions[keys["ActionPanelHotkey"]]
	navigation, navigationFound := positions["setting-row-NavigationBindings"]
	if !actionFound || !navigationFound || navigation != action+1 {
		t.Fatalf("action row = %d (found %v), navigation row = %d (found %v)", action, actionFound, navigation, navigationFound)
	}
	if positions[keys["RefinementHotkey"]] <= navigation {
		t.Fatalf("refinement row = %d, want after navigation row %d", positions[keys["RefinementHotkey"]], navigation)
	}
	for _, key := range []string{"MainHotkey", "SelectionHotkey", "QueryHotkeys", "ResultBindings"} {
		if position, found := positions[keys[key]]; !found || position >= action {
			t.Fatalf("global row %s at %d (found %v), want before in-app rows", key, position, found)
		}
	}
	if fullscreen, found := positions["setting-row-IgnoreHotkeysOnFullscreen"]; found && fullscreen >= action {
		t.Fatalf("fullscreen row = %d, want in global hotkeys before action row %d", fullscreen, action)
	}
}

func TestNavigationChoiceDescriptionsAppearOnlyInMenu(t *testing.T) {
	app := bindTestApp(&App{})
	app.generalSettings.imageForTint = func(woxImage, *woxui.Color, int) *woxui.Image { return nil }
	item := app.generalSettings.localizedSettingItem(settingItems("hotkey", settingsData{NavigationBindings: "vim"})[0])
	if got := settingValueLabel(item); got != "Vim" || item.trailers[item.value] != "" {
		t.Fatalf("collapsed value = %q, trailing = %q", got, item.trailers[item.value])
	}
	widget := app.generalSettings.buildSettingChoicePickerOverlay(&settingChoicePickerSnapshot{item: item}, settingsPalette(), 800, 600, 1)
	props := widget.(woxwidget.Stateful).Widget.(launcherview.SettingsChoiceProps)
	for _, choice := range props.Choices[1:] {
		if choice.Trailing == "" || choice.Trailing != item.choiceDescriptions[choice.Value] {
			t.Fatalf("menu choice %q description = %q", choice.Label, choice.Trailing)
		}
	}
}

func TestGeneralSettingsTablesKeepFlutterOuterGap(t *testing.T) {
	windows := woxui.NewWindowManager()
	app := newApp(false, nil, windows, newAppInstanceRegistry(), nil, true, "", launcherWindowID)
	defer app.cancel()
	app.uiCall = func(callback func()) error {
		callback()
		return nil
	}
	form := newGeneralQuerySettingsForm(settingsData{IsLinuxWaylandSession: false})
	app.generalSettings.SetForm(&form)

	page := app.buildSettingsPage(settingsSnapshot{tab: "general", general: app.generalSettings.Snapshot(), palette: settingsPalette()}, settingItems("general", settingsData{}), 800, 600, 1)
	container := page.(woxwidget.Container)
	scroll := container.Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	rows := scroll.Content.(woxwidget.Container).Child.(woxwidget.Flex).Children

	tableSpacers := 0
	var lastTableSpacer woxwidget.Container
	for _, row := range rows {
		keyed, ok := row.(woxwidget.Keyed)
		if !ok {
			continue
		}
		target, ok := keyed.Child.(woxwidget.Container)
		if !ok {
			continue
		}
		spacer, ok := target.Child.(woxwidget.Container)
		if !ok || spacer.Padding.Bottom != 24 {
			continue
		}
		tableSpacers++
		lastTableSpacer = spacer
	}
	if tableSpacers != 2 {
		t.Fatalf("general table spacers = %d, want QueryAliases, TrayQueries", tableSpacers)
	}
	if lastTableSpacer.Padding.Bottom != 24 {
		t.Fatalf("last table outer bottom gap = %v, want Flutter's 24px", lastTableSpacer.Padding.Bottom)
	}

	var lastSettingRow string
	for _, row := range rows {
		keyed, ok := row.(woxwidget.Keyed)
		if !ok {
			continue
		}
		if strings.HasPrefix(string(keyed.Key), "setting-row-") {
			lastSettingRow = string(keyed.Key)
		}
	}
	if lastSettingRow != "setting-row-HttpProxyUrl" {
		t.Fatalf("general page last built-in row = %q, want Network proxy URL at the bottom", lastSettingRow)
	}
}

func TestSettingsTitleBarUsesFixedWindowTitle(t *testing.T) {
	windows := woxui.NewWindowManager()
	app := newApp(false, nil, windows, newAppInstanceRegistry(), nil, true, "", launcherWindowID)
	defer app.cancel()
	app.language.translations["ui_settings_window_title"] = "Wox Settings"
	titleBar := app.buildSettingsTitleBar(settingsSnapshot{tab: "general"}, 1200, 240, true, 1).(woxwidget.Stateful)
	props := titleBar.Widget.(launcherview.SettingsTitleBarProps)

	if props.Title != "Wox Settings" {
		t.Fatalf("settings title bar title = %q, want fixed window title", props.Title)
	}
	if !props.Active {
		t.Fatal("settings title bar should receive the host window focus state")
	}
}
