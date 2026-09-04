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
		CellWidth: 120, CellHeight: 110, VisualWidth: 100, VisualHeight: 70, ItemPadding: 4, ShowTitle: true, TitleHeight: 22,
		Theme: woxcomponent.Theme{SelectedBackground: active},
	}).(woxwidget.Semantics).Child.(woxwidget.Gesture)
	children := result.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children
	visual := children[0].(woxwidget.Stack)
	frameBoundary := visual.Children[0].Child.(woxwidget.Boundary[launcherGridFrameProps])
	iconBoundary := visual.Children[1].Child.(woxwidget.Container).Child.(woxwidget.Boundary[launcherGridIconProps])
	titleBoundary := children[1].(woxwidget.Container).Child.(woxwidget.Boundary[launcherResultTextProps])
	frame := frameBoundary.Build(frameBoundary.Props).(woxwidget.Container)

	if frame.Color.A != 0 || frame.BorderColor != active || frame.BorderWidth != 4 || frame.Radius != 8 {
		t.Fatalf("selected grid frame = fill %#v border %#v/%.0f radius %.0f, want transparent Flutter 4px/8px frame", frame.Color, frame.BorderColor, frame.BorderWidth, frame.Radius)
	}
	if frameBoundary.Key != LauncherResultBackgroundBoundaryKey("wallpaper") || iconBoundary.Key != LauncherResultIconBoundaryKey("wallpaper") || titleBoundary.Key != LauncherResultTitleBoundaryKey("wallpaper") {
		t.Fatalf("grid boundary keys = %q/%q/%q, want independent frame/icon/title keys", frameBoundary.Key, iconBoundary.Key, titleBoundary.Key)
	}
}

func TestLauncherGridResultWiresSecondaryTap(t *testing.T) {
	tapped := false
	result := launcherGridResultView(LauncherGridResult{ID: "wallpaper", OnSecondaryTapDown: func() { tapped = true }}, LauncherGridProps{
		CellWidth: 120, CellHeight: 110, VisualWidth: 100, VisualHeight: 70,
	}).(woxwidget.Semantics).Child.(woxwidget.Gesture)

	result.OnSecondaryTapDown(woxui.Point{})
	if !tapped {
		t.Fatal("secondary tap callback was not wired to the grid result")
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
			}).(woxwidget.Semantics).Child.(woxwidget.Gesture)
			visual := result.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Stack)
			iconBoundary := visual.Children[1].Child.(woxwidget.Container).Child.(woxwidget.Boundary[launcherGridIconProps])
			if got := iconBoundary.Build(iconBoundary.Props).(woxwidget.Image).Fit; got != test.want {
				t.Fatalf("grid image fit = %v, want %v", got, test.want)
			}
		})
	}
}

func TestLauncherGridShowsQuickSelectBadge(t *testing.T) {
	fill := woxui.Color{R: 180, G: 180, B: 190, A: 255}
	text := woxui.Color{R: 24, G: 29, B: 38, A: 255}
	semantics := launcherGridResultView(LauncherGridResult{ID: "app", QuickSelectNumber: "2"}, LauncherGridProps{
		CellWidth: 120, CellHeight: 110, VisualWidth: 100, VisualHeight: 70, ItemPadding: 4,
		TailColor: fill, Theme: woxcomponent.Theme{Background: text},
	}).(woxwidget.Semantics)
	if semantics.Value != "2" {
		t.Fatalf("grid quick select value = %q, want the visible number", semantics.Value)
	}
	result := semantics.Child.(woxwidget.Gesture)
	visual := result.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Stack)
	if len(visual.Children) != 3 {
		t.Fatalf("grid visual children = %d, want frame, icon, and quick select badge", len(visual.Children))
	}
	badge := visual.Children[2].Child.(woxwidget.Align).Child.(woxwidget.Container).Child.(woxwidget.Container)
	chip := badge.Child.(woxwidget.Container)
	label := chip.Child.(woxwidget.Align).Child.(woxwidget.Text)
	if chip.Width != 20 || label.Value != "2" || chip.Color != fill || label.Color != text {
		t.Fatalf("grid quick select badge = size %.0f text %q fill %#v color %#v", chip.Width, label.Value, chip.Color, label.Color)
	}
}

func TestLauncherGridLoadingResultAnimatesSpinner(t *testing.T) {
	result := launcherGridResultView(LauncherGridResult{ID: "ai-match", Loading: true}, LauncherGridProps{
		CellWidth: 120, CellHeight: 110, VisualWidth: 100, VisualHeight: 100, Theme: woxcomponent.Theme{Cursor: woxui.Color{R: 10, G: 20, B: 30, A: 255}},
	}).(woxwidget.Semantics).Child.(woxwidget.Gesture)
	visual := result.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Stack)
	iconBoundary := visual.Children[1].Child.(woxwidget.Container).Child.(woxwidget.Boundary[launcherGridIconProps])
	aligned := iconBoundary.Build(iconBoundary.Props).(woxwidget.Align)
	if _, ok := aligned.Child.(woxwidget.LoopAnimation); !ok {
		t.Fatal("loading grid result does not animate the shared loading indicator")
	}
}

func TestLauncherGridBoundaryEqualCoversAllFields(t *testing.T) {
	woxwidget.AssertEqualCoversAllFields(t, launcherGridFrameProps{})
	woxwidget.AssertEqualCoversAllFields(t, launcherGridIconProps{})
}
