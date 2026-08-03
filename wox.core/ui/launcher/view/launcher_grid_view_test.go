package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestLauncherGridSelectedResultUsesFlutterFocusFrame(t *testing.T) {
	active := woxui.Color{R: 20, G: 110, B: 220, A: 255}
	result := launcherGridResultView(LauncherGridResult{ID: "wallpaper", Selected: true}, LauncherGridProps{
		CellWidth: 120, CellHeight: 90, VisualWidth: 100, VisualHeight: 70, ItemPadding: 4,
		Theme: woxcomponent.Theme{SelectedBackground: active},
	}).(woxwidget.Gesture)
	frame := result.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Container)

	if frame.Color.A != 0 || frame.BorderColor != active || frame.BorderWidth != 4 || frame.Radius != 8 {
		t.Fatalf("selected grid frame = fill %#v border %#v/%.0f radius %.0f, want transparent Flutter 4px/8px frame", frame.Color, frame.BorderColor, frame.BorderWidth, frame.Radius)
	}
}
