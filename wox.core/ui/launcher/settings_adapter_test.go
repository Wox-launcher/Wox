package launcher

import (
	"fmt"
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
	palette := defaultPalette()
	cacheKey := func(source woxImage, tint woxui.Color, size int) string {
		return fmt.Sprintf("%s-svg-%d-tint-%02x%02x%02x%02x", imageKey(source), size, tint.R, tint.G, tint.B, tint.A)
	}
	for _, spec := range settingNavSpecs(false) {
		source := settingNavIconSource(spec.id)
		if source.ImageData == "" {
			continue
		}
		icon := &woxui.Image{}
		app.images[cacheKey(source, palette.toolbarText, 18)] = icon
	}
	selectedSource := settingNavIconSource("ui")
	normalIcon := app.images[cacheKey(selectedSource, palette.toolbarText, 18)]
	app.imageRequested[cacheKey(selectedSource, palette.selectedTitle, 18)] = selectedSource.ImageData
	searchSource := settingControlIconSource("search")
	app.images[cacheKey(searchSource, palette.resultSubtitle, 18)] = &woxui.Image{}

	rail := app.buildSettingsRail(settingsSnapshot{tab: "appearance", palette: palette}, 260, 600, 1).(woxwidget.Stack)
	railContainer := rail.Children[0].Child.(woxwidget.Container)
	railContent := railContainer.Child.(woxwidget.LayoutBuilder).Build(woxui.Size{
		Width: railContainer.Width - railContainer.Padding.Left - railContainer.Padding.Right, Height: railContainer.Height - railContainer.Padding.Top - railContainer.Padding.Bottom,
	}).(woxwidget.Flex)
	navigation := railContent.Children[1].(woxwidget.Stack)
	scroll := navigation.Children[0].Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	rows := scroll.Content.(woxwidget.Flex)
	row := rows.Children[1].(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	icon := row.Child.(woxwidget.Flex).Children[0].(woxwidget.Align).Child.(woxwidget.Image)

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
	palette := defaultPalette()
	palette.selectedTitle = woxui.Color{R: 241, G: 242, B: 243, A: 255}
	source := settingsSearchResultIconSource(settingsSearchSetting)
	key := fmt.Sprintf("%s-svg-%d-tint-%02x%02x%02x%02x", imageKey(source), 24, palette.selectedTitle.R, palette.selectedTitle.G, palette.selectedTitle.B, palette.selectedTitle.A)
	selectedIcon := &woxui.Image{}
	app.images[key] = selectedIcon
	snapshot := settingsSnapshot{palette: palette, search: settingsSearchSnapshot{Query: woxui.TextEditingState{Text: "font"}}}

	panel := app.buildSettingsSearchResultPanel(snapshot, 240, 200, 1).(woxwidget.Container)
	props := panel.Child.(woxwidget.LayoutBuilder).Build(woxui.Size{
		Width: panel.Width - panel.Padding.Left - panel.Padding.Right, Height: panel.Height - panel.Padding.Top - panel.Padding.Bottom,
	}).(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	row := props.Content.(woxwidget.Flex).Children[0].(woxwidget.Gesture).Child.(woxwidget.Container)
	icon := row.Child.(woxwidget.Flex).Children[0].(woxwidget.Align).Child.(woxwidget.Image)

	if icon.Source != selectedIcon {
		t.Fatalf("selected settings search icon = %p, want selected text tint %p", icon.Source, selectedIcon)
	}
}

func TestSettingsSectionLabelMatchesFlutterGrouping(t *testing.T) {
	app := &App{translations: map[string]string{"ui_update_section_updates": "Updates"}}

	if got := app.settingsSectionLabel("network", "HttpProxyEnabled"); got != "" {
		t.Fatalf("network section label = %q, want no group header", got)
	}
	if got := app.settingsSectionLabel("debug", "ShowScoreTail"); got != "" {
		t.Fatalf("debug section label = %q, want no group header", got)
	}
	if got := app.settingsSectionLabel("updates", "EnableAutoUpdate"); got != "Updates" {
		t.Fatalf("updates section label = %q, want %q", got, "Updates")
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
	form := newHotkeySettingsForm(settingsData{MainHotkey: "Alt+Space", SelectionHotkey: "Alt+Shift+Space", IsLinuxWaylandSession: false})
	app.hotkeySettings.SetForm(&form)

	page := app.buildSettingsPage(settingsSnapshot{tab: "general", hotkey: app.hotkeySettings.Snapshot(), palette: defaultPalette()}, nil, 800, 600, 1)
	container := page.(woxwidget.Container)
	scroll := container.Child.(woxwidget.ScrollView)
	rows := scroll.Child.(woxwidget.Flex).Children

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
	if tableSpacers != 4 {
		t.Fatalf("general table spacers = %d, want IgnoredHotkeyApps, QueryHotkeys, QueryShortcuts, TrayQueries", tableSpacers)
	}
	if lastTableSpacer.Padding.Bottom != 24 {
		t.Fatalf("last table outer bottom gap = %v, want Flutter's 24px", lastTableSpacer.Padding.Bottom)
	}
}

func TestSettingsTitleBarUsesFixedWindowTitle(t *testing.T) {
	windows := woxui.NewWindowManager()
	app := newApp(false, nil, windows, newAppInstanceRegistry(), nil, true, "", launcherWindowID)
	defer app.cancel()
	app.translations["ui_settings_window_title"] = "Wox Settings"
	titleBar := app.buildSettingsTitleBar(settingsSnapshot{tab: "general"}, 1200, 240).(woxwidget.Stateful)
	props := titleBar.Widget.(launcherview.SettingsTitleBarProps)

	if props.Title != "Wox Settings" {
		t.Fatalf("settings title bar title = %q, want fixed window title", props.Title)
	}
}
