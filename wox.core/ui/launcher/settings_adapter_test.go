package launcher

import (
	"fmt"
	"testing"

	woxcomponent "wox/ui/launcher/component"
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
	navigation := railContainer.Child.(woxwidget.Flex).Children[1].(woxwidget.Stack)
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
	scroll := panel.Child.(woxwidget.Gesture).Child.(woxwidget.Stack).Children[0].Child.(woxwidget.ScrollView)
	row := scroll.Child.(woxwidget.Flex).Children[0].(woxwidget.Gesture).Child.(woxwidget.Container)
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
