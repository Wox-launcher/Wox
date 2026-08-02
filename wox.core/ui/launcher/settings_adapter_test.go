package launcher

import (
	"fmt"
	"testing"

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
		app.images[cacheKey(source, palette.toolbarText, 24)] = icon
	}
	selectedSource := settingNavIconSource("ui")
	normalIcon := app.images[cacheKey(selectedSource, palette.toolbarText, 24)]
	app.imageRequested[cacheKey(selectedSource, palette.selectedTitle, 24)] = selectedSource.ImageData
	searchSource := settingControlIconSource("search")
	app.images[cacheKey(searchSource, palette.resultSubtitle, 18)] = &woxui.Image{}

	rail := app.buildSettingsRail(settingsSnapshot{tab: "appearance", palette: palette}, 260, 600, 1).(woxwidget.Stack)
	railContainer := rail.Children[0].Child.(woxwidget.Container)
	navigation := railContainer.Child.(woxwidget.Flex).Children[1].(woxwidget.Stack)
	rows := navigation.Children[0].Child.(woxwidget.ScrollView).Child.(woxwidget.Flex)
	row := rows.Children[1].(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	icon := row.Child.(woxwidget.Flex).Children[0].(woxwidget.Align).Child.(woxwidget.Image)

	if icon.Source != normalIcon {
		t.Fatalf("selected navigation icon = %p, want cached SVG %p while the selected tint loads", icon.Source, normalIcon)
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
