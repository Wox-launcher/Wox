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

func TestTerminalPreviewUsesFramelessFlutterSurface(t *testing.T) {
	theme := woxcomponent.Theme{PreviewText: woxui.Color{R: 220, G: 230, B: 240, A: 255}, PreviewSplit: woxui.Color{R: 100, G: 110, B: 120, A: 255}}
	tooltip := ""
	view := TerminalPreviewView(TerminalPreviewProps{
		Width: 500, Height: 300, SessionID: "test", Command: "ping example.com", SearchOpen: true, Theme: theme,
		SearchHotkey: "Cmd+Shift+F", FullscreenHotkey: "Cmd+B", OnTagHover: func(_ bool, text string, _ woxui.Rect) { tooltip = text },
	}).(woxwidget.Container)
	if view.BorderWidth != 0 || view.Color.A != 0 || view.Padding.Left != 10 || view.Padding.Top != 10 || view.Padding.Right != 12 {
		t.Fatalf("terminal outer surface = border %.0f fill %#v padding %#v; want Flutter frameless padding", view.BorderWidth, view.Color, view.Padding)
	}
	stack := view.Child.(woxwidget.Stack)
	if stack.Width != 478 {
		t.Fatalf("terminal content width = %.0f, want 478 after outer padding", stack.Width)
	}
	content := stack.Children[0].Child.(woxwidget.Flex)
	header := content.Children[0].(woxwidget.Container).Child.(woxwidget.Container)
	if header.BorderWidth != 1 || header.Color.A == 0 {
		t.Fatalf("terminal header = border %.0f fill %#v; want framed status bar", header.BorderWidth, header.Color)
	}
	headerStack := header.Child.(woxwidget.Stack)
	find := headerStack.Children[2].Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	if find.Label != "Find" || find.Icon == nil || find.OnHoverAt == nil {
		t.Fatalf("terminal find action = %+v; want shared icon button", find)
	}
	find.OnHoverAt(true, woxui.Rect{})
	if tooltip != "Cmd+Shift+F" {
		t.Fatalf("terminal find tooltip = %q, want Cmd+Shift+F", tooltip)
	}
	fullscreen := headerStack.Children[3].Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	if fullscreen.Label != "Toggle fullscreen" || fullscreen.Icon == nil || fullscreen.OnHoverAt == nil {
		t.Fatalf("terminal fullscreen action = %+v; want shared icon button", fullscreen)
	}
	fullscreen.OnHoverAt(true, woxui.Rect{})
	if tooltip != "Cmd+B" {
		t.Fatalf("terminal fullscreen tooltip = %q, want Cmd+B", tooltip)
	}
	search := content.Children[1].(woxwidget.Container).Child.(woxwidget.Container).Child.(woxwidget.Flex)
	if _, ok := search.Children[0].(woxwidget.Stateful).Widget.(woxcomponent.TextFieldProps); !ok {
		t.Fatal("terminal find input does not reuse WoxTextField")
	}
	count := search.Children[1].(woxwidget.Semantics)
	if count.AutomationID != "launcher.preview.terminal.search.match-count" || count.Value != "0/0" {
		t.Fatalf("terminal match count semantics = %+v, want stable 0/0 state", count)
	}
	body := content.Children[2].(woxwidget.Container)
	if body.BorderWidth != 0 || body.Color.A != 0 {
		t.Fatalf("terminal output surface = border %.0f fill %#v; want transparent output", body.BorderWidth, body.Color)
	}
}

func TestTerminalHighlightSegmentsFollowWrappedLines(t *testing.T) {
	segments := terminalHighlightSegments("first ms second ms", []string{"first ms", "second ms"}, []TerminalMatch{{Start: 6, End: 8}, {Start: 16, End: 18}})
	if len(segments) != 2 || segments[0] != (terminalHighlightSegment{line: 0, start: 6, end: 8, matchIndex: 0}) || segments[1] != (terminalHighlightSegment{line: 1, start: 7, end: 9, matchIndex: 1}) {
		t.Fatalf("terminal highlight segments = %+v, want both wrapped matches", segments)
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
	if glyph := menu.Icon.(woxwidget.Image); glyph.Source == nil || glyph.Width != 22 || glyph.Height != 22 {
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
	if glyph := exit.Icon.(woxwidget.Image); glyph.Source == nil || glyph.Width != 16 || glyph.Height != 16 {
		t.Fatalf("chat exit glyph = %vx%v, want centered 16x16", glyph.Width, glyph.Height)
	}
	if exit.Background.A != 0 || exit.HoverBackground.A == 0 {
		t.Fatal("chat exit hover background remained transparent")
	}
}
