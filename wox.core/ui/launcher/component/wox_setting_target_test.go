package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestWoxSettingTargetUsesFlutterSearchHighlightCue(t *testing.T) {
	active := woxui.Color{R: 80, G: 100, B: 120, A: 255}
	child := woxwidget.Painter{Width: 320, Height: 62}
	target := WoxSettingTarget(SettingTargetProps{
		Width: 320, Height: 62, Highlighted: true, Child: child, Theme: Theme{SelectedBackground: active},
	}).(woxwidget.Container)

	if target.Radius != 6 || target.BorderWidth != 1 {
		t.Fatalf("highlight geometry = radius %v border %v, want Flutter 6px radius and 1px border", target.Radius, target.BorderWidth)
	}
	if target.Color != (woxui.Color{R: 80, G: 100, B: 120, A: 31}) {
		t.Fatalf("highlight fill = %#v, want active color at 0.12 alpha", target.Color)
	}
	if target.BorderColor != (woxui.Color{R: 80, G: 100, B: 120, A: 87}) {
		t.Fatalf("highlight border = %#v, want active color at 0.34 alpha", target.BorderColor)
	}
	gotChild, ok := target.Child.(woxwidget.Painter)
	if !ok || gotChild.Width != child.Width || gotChild.Height != child.Height {
		t.Fatalf("highlight child = %#v, want original setting row geometry", target.Child)
	}
}
