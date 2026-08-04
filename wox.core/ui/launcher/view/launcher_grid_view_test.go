package view

import (
	"image"
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

func TestLauncherGridImageUsesFlutterFit(t *testing.T) {
	icon, err := woxui.NewImage(image.NewRGBA(image.Rect(0, 0, 200, 100)))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name         string
		visualWidth  float32
		visualHeight float32
		want         woxwidget.ImageFit
	}{
		{name: "square", visualWidth: 100, visualHeight: 100, want: woxwidget.ImageFitContain},
		{name: "landscape", visualWidth: 160, visualHeight: 90, want: woxwidget.ImageFitCover},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := launcherGridResultView(LauncherGridResult{ID: "wallpaper", Icon: icon}, LauncherGridProps{
				CellWidth: test.visualWidth, CellHeight: test.visualHeight, VisualWidth: test.visualWidth, VisualHeight: test.visualHeight,
			}).(woxwidget.Gesture)
			frame := result.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Container)
			if got := frame.Child.(woxwidget.Image).Fit; got != test.want {
				t.Fatalf("grid image fit = %v, want %v", got, test.want)
			}
		})
	}
}
