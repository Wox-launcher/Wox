package textoverlay

import (
	"runtime"
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/overlay"
)

func TestRuntimeTextWindowGrowsUntilMaximumHeight(t *testing.T) {
	if height := runtimeTextWindowHeight(240, 0, 600); height != 240 {
		t.Fatalf("short content height = %v, want 240", height)
	}
	if height := runtimeTextWindowHeight(720, 0, 600); height != 600 {
		t.Fatalf("long content height = %v, want 600", height)
	}
}

func TestTextOverlayFontSize(t *testing.T) {
	if size := textOverlayFontSize(Options{}); size != DefaultFontSize {
		t.Fatalf("default font size = %v, want %v", size, DefaultFontSize)
	}
	if size := textOverlayFontSize(Options{FontSize: 12}); size != 12 {
		t.Fatalf("custom font size = %v, want 12", size)
	}
}

func TestTextOverlayPadding(t *testing.T) {
	fallback := textOverlayPadding(Options{})
	if fallback.Left != runtimeTextPaddingX || fallback.Top != runtimeTextPaddingY {
		t.Fatalf("default padding = %+v, want %v/%v", fallback, runtimeTextPaddingX, runtimeTextPaddingY)
	}
	custom := textOverlayPadding(Options{Padding: woxwidget.Insets{Left: 11, Top: 8, Right: 11, Bottom: 8}})
	if custom.Left != 11 || custom.Top != 8 {
		t.Fatalf("custom padding = %+v, want 11/8", custom)
	}
}

func TestRuntimeTextTitleBarIsReservedForDialogStyleOverlays(t *testing.T) {
	if runtimeTextUsesTitleBar(Options{Closable: true}) {
		t.Fatal("compact auto-close notification unexpectedly uses a title bar")
	}
	if runtimeTextUsesTitleBar(Options{Closable: true, Window: overlay.WindowOptions{CloseOnEscape: true}}) {
		t.Fatal("untitled closeable overlay unexpectedly uses a title bar")
	}
	if !runtimeTextUsesTitleBar(Options{Title: "Translate", Closable: true, Window: overlay.WindowOptions{CloseOnEscape: true}}) {
		t.Fatal("titled overlay does not use a title bar")
	}
	if !runtimeTextUsesTitleBar(Options{ShowCopyButton: true}) {
		t.Fatal("copyable overlay does not use a title bar")
	}
}

func TestRuntimeTextTitleBarContainsCopyAndCloseControls(t *testing.T) {
	instance := &runtimeTextOverlay{
		options:   Options{Title: "Summarize", Closable: true, ShowCopyButton: true, Window: overlay.WindowOptions{Movable: true}},
		titleIcon: &woxui.Image{},
	}
	titleBar := instance.buildTitleBar(420, true, woxui.Color{R: 255, G: 255, B: 255, A: 255}).(woxwidget.Stack)
	controls := map[woxwidget.Key]bool{}
	hasTitle := false
	hasIcon := false
	hasCopyTooltip := false
	hasTitleDrag := false
	for _, child := range titleBar.Children {
		switch widget := child.Child.(type) {
		case woxwidget.Stateful:
			controls[widget.Key] = true
			if widget.Key == "text-overlay-copy" {
				hasCopyTooltip = widget.Widget.(woxcomponent.IconButtonProps).OnHoverAt != nil
			}
		case woxwidget.TextBlock:
			hasTitle = widget.Value == "Summarize"
		case woxwidget.Image:
			hasIcon = true
		case woxwidget.Gesture:
			if widget.ID == "text-overlay-title-drag" && widget.OnDragStart != nil {
				hasTitleDrag = true
			}
		}
	}
	for _, key := range []woxwidget.Key{"text-overlay-copy", "text-overlay-close"} {
		if !controls[key] {
			t.Fatalf("text overlay title bar missing %q", key)
		}
	}
	if !hasTitle || !hasIcon {
		t.Fatalf("text overlay title bar title/icon = %v/%v, want both", hasTitle, hasIcon)
	}
	if !hasCopyTooltip {
		t.Fatal("text overlay copy button does not expose a tooltip hover trigger")
	}
	if !hasTitleDrag {
		t.Fatal("movable text overlay title bar is not a window-drag target")
	}
}

func TestRuntimeTextTitleBarRemainsDraggableWhenBodyIsClickable(t *testing.T) {
	instance := &runtimeTextOverlay{
		layout: runtimeTextLayout{
			windowSize:     woxui.Size{Width: 420, Height: 160},
			contentSize:    woxui.Size{Width: 384, Height: 96},
			titleBarHeight: runtimeTextTitleBarHeight,
			viewportHeight: 96,
			textWidth:      384,
		},
		options: Options{
			Title: "Translate", Closable: true, ShowCopyButton: true,
			OnClick: func() bool { return true },
			Window:  overlay.WindowOptions{Movable: true},
		},
	}
	root := instance.build(woxui.FrameInfo{Size: woxui.Size{Width: 420, Height: 160}}).(woxwidget.Gesture)
	if root.ID != "text-overlay-click" {
		t.Fatalf("root id = %q, want text-overlay-click", root.ID)
	}
	stack := root.Child.(woxwidget.Stack)
	var titleBar woxwidget.Stack
	found := false
	for _, child := range stack.Children {
		inner, ok := child.Child.(woxwidget.Stack)
		if !ok || inner.Height != runtimeTextTitleBarHeight {
			continue
		}
		titleBar = inner
		found = true
		break
	}
	if !found {
		t.Fatal("clickable text overlay is missing its title bar")
	}
	drag, ok := titleBar.Children[0].Child.(woxwidget.Gesture)
	if !ok || drag.ID != "text-overlay-title-drag" || drag.OnDragStart == nil {
		t.Fatal("clickable text overlay title bar is not draggable")
	}
}

func TestRuntimeTextTitleBarOmitsDragWhenNotMovable(t *testing.T) {
	instance := &runtimeTextOverlay{
		options: Options{Title: "Translate", Closable: true, ShowCopyButton: true},
	}
	titleBar := instance.buildTitleBar(420, true, woxui.Color{R: 255, G: 255, B: 255, A: 255}).(woxwidget.Stack)
	if _, ok := titleBar.Children[0].Child.(woxwidget.Gesture); ok {
		t.Fatal("fixed text overlay title bar unexpectedly starts window dragging")
	}
}

func TestRuntimeTextDefaultFontIsReadable(t *testing.T) {
	if DefaultFontSize != 14 {
		t.Fatalf("default font size = %v, want 14", DefaultFontSize)
	}
}

func TestRuntimeTextCopyTooltipIsAnchoredAboveTitleButton(t *testing.T) {
	x, y := runtimeTextCopyTooltipAnchor(woxui.Rect{X: 100, Y: 200}, woxui.Rect{X: 320, Y: 4, Width: 32, Height: 32})
	if x != 436 || y != 200 {
		t.Fatalf("copy tooltip anchor = (%v, %v), want (436, 200)", x, y)
	}
}

func TestRuntimeTextWindowChromeDefersToSystemCorners(t *testing.T) {
	for _, goos := range []string{"windows", "darwin"} {
		radius, borderWidth, borderColor := runtimeTextWindowChrome(goos)
		if radius != 0 || borderWidth != 0 || borderColor.A != 0 {
			t.Fatalf("%s chrome = radius %v border %v/%#v, want system window corners only", goos, radius, borderWidth, borderColor)
		}
	}
}

func TestRuntimeTextWindowChromeStrokesLinuxAlphaCorners(t *testing.T) {
	radius, borderWidth, borderColor := runtimeTextWindowChrome("linux")
	if radius != runtimeTextSystemCornerRadius || borderWidth != 1 || borderColor.A != 30 {
		t.Fatalf("linux chrome = radius %v border %v/%#v, want 14px stroke for square GTK windows", radius, borderWidth, borderColor)
	}
}

func TestRuntimeTextCopyTooltipUsesWindowChrome(t *testing.T) {
	tooltip := runtimeTextCopyTooltip(80, "Copy date", woxui.TextStyle{Size: 11}, woxui.Color{R: 246, G: 246, B: 246, A: 255})
	radius, borderWidth, borderColor := runtimeTextWindowChrome(runtime.GOOS)
	if tooltip.Radius != radius || tooltip.BorderWidth != borderWidth || tooltip.BorderColor != borderColor {
		t.Fatalf("copy tooltip chrome = radius %v border %v/%#v, want %+v/%v/%#v", tooltip.Radius, tooltip.BorderWidth, tooltip.BorderColor, radius, borderWidth, borderColor)
	}
	fill := overlay.PanelFill(runtime.GOOS, false)
	if tooltip.Color != fill {
		t.Fatalf("copy tooltip fill = %#v, want %#v", tooltip.Color, fill)
	}
	if tooltip.Height != runtimeTextTooltipHeight {
		t.Fatalf("copy tooltip height = %v, want %v", tooltip.Height, runtimeTextTooltipHeight)
	}
}

func TestRuntimeTextOverlayBuildUsesWindowChrome(t *testing.T) {
	instance := &runtimeTextOverlay{
		layout: runtimeTextLayout{
			windowSize:  woxui.Size{Width: 160, Height: 48},
			contentSize: woxui.Size{Width: 140, Height: 28},
			textWidth:   140,
		},
		options: Options{Message: "hello"},
	}
	root := instance.build(woxui.FrameInfo{Size: woxui.Size{Width: 160, Height: 48}}).(woxwidget.Stack)
	panel, ok := root.Children[0].Child.(woxwidget.Container)
	if !ok {
		t.Fatalf("root panel type = %T, want Container", root.Children[0].Child)
	}
	radius, borderWidth, borderColor := runtimeTextWindowChrome(runtime.GOOS)
	if panel.Radius != radius || panel.BorderWidth != borderWidth || panel.BorderColor != borderColor {
		t.Fatalf("overlay panel chrome = radius %v border %v/%#v, want %v/%v/%#v", panel.Radius, panel.BorderWidth, panel.BorderColor, radius, borderWidth, borderColor)
	}
	fill := overlay.PanelFill(runtime.GOOS, false)
	if panel.Color != fill {
		t.Fatalf("overlay panel fill = %#v, want %#v", panel.Color, fill)
	}
}

func TestRuntimeTextOverlayBuildUsesAppearancePanelFill(t *testing.T) {
	instance := &runtimeTextOverlay{
		layout: runtimeTextLayout{
			windowSize:  woxui.Size{Width: 160, Height: 48},
			contentSize: woxui.Size{Width: 140, Height: 28},
			textWidth:   140,
		},
		options: Options{Message: "hello", Window: overlay.WindowOptions{LightAppearance: true}},
	}
	root := instance.build(woxui.FrameInfo{Size: woxui.Size{Width: 160, Height: 48}}).(woxwidget.Stack)
	panel := root.Children[0].Child.(woxwidget.Container)
	fill := overlay.PanelFill(runtime.GOOS, true)
	if panel.Color != fill {
		t.Fatalf("light overlay panel fill = %#v, want %#v", panel.Color, fill)
	}
}

func TestRuntimeTextOverlayBuildInvokesClick(t *testing.T) {
	clicked := false
	instance := &runtimeTextOverlay{
		layout: runtimeTextLayout{
			windowSize:  woxui.Size{Width: 160, Height: 48},
			contentSize: woxui.Size{Width: 140, Height: 28},
			textWidth:   140,
		},
		options: Options{Message: "click", OnClick: func() bool { clicked = true; return true }},
	}
	root := instance.build(woxui.FrameInfo{Size: woxui.Size{Width: 160, Height: 48}}).(woxwidget.Gesture)
	root.OnTap()
	if !clicked {
		t.Fatal("clickable text overlay did not invoke OnClick")
	}
}
