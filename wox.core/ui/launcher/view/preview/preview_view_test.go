package preview

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestPreviewSurfaceUsesFlutterTranslucentFill(t *testing.T) {
	theme := woxcomponent.Theme{
		Background:   woxui.Color{R: 12, G: 18, B: 24, A: 180},
		PreviewText:  woxui.Color{R: 220, G: 230, B: 240, A: 64},
		PreviewSplit: woxui.Color{R: 100, G: 110, B: 120, A: 32},
	}
	surface := previewSurface(woxwidget.Container{}, theme, 320, 180).(woxwidget.Container)

	if surface.BorderColor.A != 115 || surface.BorderWidth != 1 {
		t.Fatalf("preview border = %#v at %v, want Flutter 0.45 alpha 1px stroke", surface.BorderColor, surface.BorderWidth)
	}
	if surface.Color != (woxui.Color{R: 220, G: 230, B: 240, A: 9}) {
		t.Fatalf("preview fill = %#v, want preview text color at Flutter 0.035 alpha", surface.Color)
	}
	if _, nestedFill := surface.Child.(woxwidget.Container); nestedFill {
		t.Fatal("preview uses a nested fill to simulate its border")
	}
}

func TestPreviewTagHoverUsesTooltip(t *testing.T) {
	var hovered bool
	var tooltip string
	var anchor woxui.Rect
	view := PreviewTags([]PreviewTag{{Label: "51 chars", Tooltip: "Character count"}}, woxcomponent.Theme{}, &woxui.Window{}, 300, func(inside bool, text string, bounds woxui.Rect) {
		hovered, tooltip, anchor = inside, text, bounds
	}).(woxwidget.ScrollView)
	row := view.Child.(woxwidget.Flex)
	wrapper := row.Children[0].(woxwidget.Container)
	gesture := wrapper.Child.(woxwidget.Gesture)
	wantAnchor := woxui.Rect{X: 2, Y: 3, Width: 40, Height: 26}
	gesture.OnHoverAt(true, wantAnchor)

	if !hovered || tooltip != "Character count" || anchor != wantAnchor {
		t.Fatalf("hover = %v, %q, %#v; want tooltip and anchor", hovered, tooltip, anchor)
	}
}
