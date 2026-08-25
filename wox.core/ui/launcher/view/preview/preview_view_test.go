package preview

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestPreviewImageOmitsOverlayGestureWithoutOnTap(t *testing.T) {
	view := builtPreviewImage(PreviewImageProps{Width: 200, Height: 100, Image: &woxui.Image{Width: 10, Height: 20}})
	if view.OnTap != nil {
		t.Fatal("preview image should not open an overlay when OnTap is unset")
	}
	if view.OnPointer == nil {
		t.Fatal("preview image should consume wheel zoom")
	}
}

func TestPreviewImageWrapsOverlayGestureWithOnTap(t *testing.T) {
	tapped := false
	view := builtPreviewImage(PreviewImageProps{
		Width: 200, Height: 100, Image: &woxui.Image{Width: 10, Height: 10}, OnTap: func() { tapped = true },
	})
	if view.ID != "preview-image-overlay" || view.OnTap == nil {
		t.Fatalf("preview image gesture = %+v, want overlay tap", view)
	}
	view.OnTap()
	if !tapped {
		t.Fatal("preview image tap did not fire")
	}
}

func builtPreviewImage(props PreviewImageProps) woxwidget.Gesture {
	view := PreviewImage(props).(woxwidget.Stateful)
	return view.CreateState().Build(woxwidget.StateContext{}, props).(woxwidget.Gesture)
}

func TestPreviewSurfaceUsesFlutterTranslucentFill(t *testing.T) {
	theme := woxcomponent.Theme{
		Background:   woxui.Color{R: 12, G: 18, B: 24, A: 180},
		PreviewText:  woxui.Color{R: 220, G: 230, B: 240, A: 64},
		PreviewSplit: woxui.Color{R: 100, G: 110, B: 120, A: 32},
	}
	surface := previewSurface(woxwidget.Container{}, theme, 320, 180).(woxwidget.Container)

	if surface.Radius != previewSurfaceRadius || surface.BorderWidth != previewSurfaceBorderWidth || surface.Padding != woxwidget.UniformInsets(previewSurfaceBorderWidth) {
		t.Fatalf("preview shell = radius %v border %v padding %#v, want concentric %v/%v inset", surface.Radius, surface.BorderWidth, surface.Padding, previewSurfaceRadius, previewSurfaceBorderWidth)
	}
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

func TestPreviewTagsScrollHorizontallyWhenOverflowing(t *testing.T) {
	tags := make([]PreviewTag, 8)
	for index := range tags {
		tags[index] = PreviewTag{Label: "tag"}
	}
	view := PreviewTags(tags, woxcomponent.Theme{}, &woxui.Window{}, 120, nil).(woxwidget.ScrollView)
	if view.Key != "preview-tags" || !view.Horizontal || !view.MapVerticalWheel {
		t.Fatalf("preview tags = key %q horizontal %v map-wheel %v, want a retained horizontal strip that accepts a vertical wheel", view.Key, view.Horizontal, view.MapVerticalWheel)
	}
	if view.Width != 120 || view.ContentWidth <= view.Width {
		t.Fatalf("preview tags geometry = viewport %.0f content %.0f, want overflowing content", view.Width, view.ContentWidth)
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
	semantics := wrapper.Child.(woxwidget.Semantics)
	if semantics.AutomationID != "preview-tag-0" || semantics.Role != woxui.AccessibilityRoleText || semantics.Label != "51 chars" || semantics.Description != "Character count" {
		t.Fatalf("preview tag semantics = %+v, want a stable hoverable tooltip target", semantics)
	}
	gesture := semantics.Child.(woxwidget.Gesture)
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
	command := headerStack.Children[1]
	if command.Left != 17 || command.Right != 79 || !command.StretchWidth {
		t.Fatalf("terminal command layout = left/right %.0f/%.0f stretch %v, want 17/79/true", command.Left, command.Right, command.StretchWidth)
	}
	commandAlign := command.Child.(woxwidget.Align)
	if command.Top != 0 || commandAlign.Height != 34 || commandAlign.Vertical != 0.5 {
		t.Fatalf("terminal command alignment = top %.0f child %#v, want full-height vertical center", command.Top, command.Child)
	}
	findAlign := headerStack.Children[2].Child.(woxwidget.Align)
	find := findAlign.Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	if !headerStack.Children[2].AnchorRight || headerStack.Children[2].Right != 34 || !headerStack.Children[3].AnchorRight {
		t.Fatalf("terminal action anchors = find %v/%.0f fullscreen %v, want true/34/true", headerStack.Children[2].AnchorRight, headerStack.Children[2].Right, headerStack.Children[3].AnchorRight)
	}
	if findAlign.Height != 34 || findAlign.Vertical != 0.5 {
		t.Fatalf("terminal find alignment = %#v, want full-height vertical center", findAlign)
	}
	if find.Label != "Find" || find.Icon == nil || find.OnHoverAt == nil {
		t.Fatalf("terminal find action = %+v; want shared icon button", find)
	}
	find.OnHoverAt(true, woxui.Rect{})
	if tooltip != "Cmd+Shift+F" {
		t.Fatalf("terminal find tooltip = %q, want Cmd+Shift+F", tooltip)
	}
	fullscreen := headerStack.Children[3].Child.(woxwidget.Align).Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
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
	titleAlign := title.Child.(woxwidget.Align)
	if title.Top != 0 || title.Right != 44 || !title.StretchWidth || titleAlign.Height != 48 || titleAlign.Vertical != 0.5 {
		t.Fatalf("chat title layout = top/right %.0f/%.0f stretch %v alignment %#v", title.Top, title.Right, title.StretchWidth, titleAlign)
	}
	titleDrag := titleAlign.Child.(woxwidget.Gesture)
	titleDrag.OnDragStart()
	if !dragged {
		t.Fatal("chat title drag did not start window dragging")
	}
	menu := stack.Children[0].Child.(woxwidget.Align).Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	if menu.Width != 36 || menu.Height != 36 || menu.Background.A != 0 || menu.HoverBackground.A == 0 {
		t.Fatalf("chat sidebar props = %+v, want transparent hoverable 36x36 icon button", menu)
	}
	if glyph := menu.Icon.(woxwidget.Image); glyph.Source == nil || glyph.Width != 20 || glyph.Height != 20 {
		t.Fatalf("chat sidebar glyph = %vx%v, want centered 20x20", glyph.Width, glyph.Height)
	}
	exitChild := stack.Children[len(stack.Children)-1]
	exitAlign := exitChild.Child.(woxwidget.Align)
	if exitChild.Top != 0 || exitChild.Right != 6 || !exitChild.AnchorRight || exitAlign.Height != 48 || exitAlign.Vertical != 0.5 {
		t.Fatalf("chat exit layout = top/right %.0f/%.0f anchor %v alignment %#v", exitChild.Top, exitChild.Right, exitChild.AnchorRight, exitAlign)
	}
	exit := exitAlign.Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
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

func TestChatModelSelectorUsesFlutterIconsAndHoverSurface(t *testing.T) {
	theme := woxcomponent.Theme{
		ResultTitle:        woxui.Color{R: 220, G: 225, B: 230, A: 255},
		SelectedBackground: woxui.Color{R: 80, G: 90, B: 100, A: 255},
	}
	props := ChatInputProps{Key: "test", Model: "deepseek-v4-pro", ModelWidth: 160, Theme: theme, OnModels: func() {}}
	state := chatModelSelectorState{hovered: true}
	semantics := state.Build(woxwidget.StateContext{}, props).(woxwidget.Semantics)
	focusable := semantics.Child.(woxwidget.Focusable)
	gesture := focusable.Child.(woxwidget.Gesture)
	chip := gesture.Child.(woxwidget.Container)
	row := chip.Child.(woxwidget.Flex)

	if chip.Height != 20 || chip.Radius != 4 || chip.Color.A != 40 || gesture.OnHover == nil {
		t.Fatalf("model chip = height %.0f radius %.0f color %#v; want Flutter compact hover surface", chip.Height, chip.Radius, chip.Color)
	}
	modelIcon := row.Children[0].(woxwidget.Image)
	modelText := row.Children[2].(woxwidget.Expanded).Child.(woxwidget.Align)
	arrowIcon := row.Children[4].(woxwidget.Image)
	if modelIcon.Source == nil || modelIcon.Width != 16 || modelText.Height != 20 || modelText.Vertical != 0.5 || arrowIcon.Source == nil || arrowIcon.Width != 14 {
		t.Fatalf("model chip icons = model %.0f arrow %.0f; want Flutter 16px and 14px SVGs", modelIcon.Width, arrowIcon.Width)
	}
	input := ChatInput(ChatInputProps{Width: 400, Height: 98, Key: "test", Model: "deepseek-v4-pro", ModelWidth: 160, Theme: theme}).(woxwidget.Container)
	card := input.Child.(woxwidget.Container)
	toolbar := card.Child.(woxwidget.Flex).Children[2].(woxwidget.Stack)
	modelAlign := toolbar.Children[0].Child.(woxwidget.Align)
	if toolbar.Children[0].Top != 0 || modelAlign.Height != 42 || modelAlign.Vertical != 0.5 {
		t.Fatalf("model toolbar alignment = top %.0f height %.0f vertical %.1f; want native vertical centering", toolbar.Children[0].Top, modelAlign.Height, modelAlign.Vertical)
	}
}

func TestChatCatalogModelRowHighlightsOnHover(t *testing.T) {
	theme := woxcomponent.Theme{
		PreviewText:        woxui.Color{R: 220, G: 225, B: 230, A: 255},
		SelectedBackground: woxui.Color{R: 80, G: 90, B: 100, A: 255},
	}
	row := chatCatalogItem(ChatCatalogItemProps{SelectID: "model", Kind: "models", Title: "pro", OnSelect: func() {}}, 400, 38, theme, true, func(bool) {}).(woxwidget.Gesture)
	container := row.Child.(woxwidget.Container)
	stack := container.Child.(woxwidget.Stack)
	icon := stack.Children[0].Child.(woxwidget.Image)

	if row.OnHover == nil || container.Color.A != 40 || icon.Source == nil || icon.Width != 18 {
		t.Fatalf("hovered model row = color %#v icon %.0f; want Flutter hover and 18px model icon", container.Color, icon.Width)
	}
}
