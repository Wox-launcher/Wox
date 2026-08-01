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

func TestChatHeaderExitKeepsGlyphVisible(t *testing.T) {
	dragged := false
	theme := woxcomponent.Theme{ResultSubtitle: woxui.Color{R: 120, G: 130, B: 140, A: 255}}
	header := ChatHeader(ChatHeaderProps{Width: 500, Height: 48, Key: "test", ShowExit: true, ExitLabel: "Close", Theme: theme, OnExit: func() {}, OnDrag: func() { dragged = true }}).(woxwidget.Container)
	stack := header.Child.(woxwidget.Stack)
	title := stack.Children[1]
	if title.Top != 5 {
		t.Fatalf("chat title top = %v, want 5 to share the icon center line", title.Top)
	}
	titleDrag := title.Child.(woxwidget.Gesture)
	titleDrag.OnDragStart()
	if !dragged {
		t.Fatal("chat title drag did not start window dragging")
	}
	menu := stack.Children[0].Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	if menu.Width != 36 || menu.Height != 36 || menu.HoverBackground.A == 0 {
		t.Fatalf("chat menu props = %+v, want hoverable 36x36 icon button", menu)
	}
	if glyph := menu.Icon.(woxwidget.Painter); glyph.Width != 22 || glyph.Height != 22 {
		t.Fatalf("chat menu glyph = %vx%v, want centered 22x22", glyph.Width, glyph.Height)
	}
	exitChild := stack.Children[len(stack.Children)-1]
	if exitChild.Top != 9 {
		t.Fatalf("chat exit top = %v, want 9", exitChild.Top)
	}
	exit := exitChild.Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	if exit.OnHoverAt == nil || exit.OnTap == nil || exit.Width != 28 || exit.Height != 28 {
		t.Fatalf("chat exit props = %+v, want hoverable 28x28 icon button", exit)
	}
	if glyph := exit.Icon.(woxwidget.Painter); glyph.Width != 16 || glyph.Height != 16 {
		t.Fatalf("chat exit glyph = %vx%v, want centered 16x16", glyph.Width, glyph.Height)
	}
	if exit.Background.A != 0 || exit.HoverBackground.A == 0 {
		t.Fatal("chat exit hover background remained transparent")
	}
}
