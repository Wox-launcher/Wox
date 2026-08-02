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
	palette := defaultPalette()
	cacheKey := func(source woxImage, tint woxui.Color, size int) string {
		return fmt.Sprintf("%s-svg-%d-tint-%02x%02x%02x%02x", imageKey(source), size, tint.R, tint.G, tint.B, tint.A)
	}
	var normalIcon *woxui.Image
	for _, spec := range settingNavSpecs(false) {
		source := settingNavIconSource(spec.id)
		if source.ImageData == "" {
			continue
		}
		icon := &woxui.Image{}
		app.images[cacheKey(source, palette.toolbarText, 24)] = icon
		if spec.id == "ui" {
			normalIcon = icon
			app.imageRequested[cacheKey(source, palette.selectedTitle, 24)] = source.ImageData
		}
	}
	searchSource := settingControlIconSource("search")
	app.images[cacheKey(searchSource, palette.resultSubtitle, 18)] = &woxui.Image{}

	rail := app.buildSettingsRail(settingsSnapshot{tab: "appearance", palette: palette}, 260, 600, 1).(woxwidget.Stack)
	railContainer := rail.Children[0].Child.(woxwidget.Container)
	navigation := railContainer.Child.(woxwidget.Flex).Children[1].(woxwidget.Stack)
	rows := navigation.Children[0].Child.(woxwidget.ScrollView).Child.(woxwidget.Flex)
	row := rows.Children[1].(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	icon := row.Child.(woxwidget.Flex).Children[0].(woxwidget.Align).Child.(woxwidget.Image)

	if icon.Source != normalIcon {
		t.Fatal("selected navigation item replaced its cached SVG while the selected tint was loading")
	}
}
