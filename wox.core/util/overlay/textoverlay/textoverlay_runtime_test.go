package textoverlay

import (
	"runtime"
	"testing"

	"wox/common"
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
	titleBar := instance.buildTitleBar(420, true, overlay.ThemeChrome{Foreground: woxui.Color{R: 255, G: 255, B: 255, A: 255}}).(woxwidget.Stack)
	controls := map[woxwidget.Key]bool{}
	hasTitle := false
	hasIcon := false
	hasCopyTooltip := false
	hasTitleDrag := false
	walkTextOverlayTitleBar(titleBar, func(widget woxwidget.Widget) {
		switch typed := widget.(type) {
		case woxwidget.Stateful:
			controls[typed.Key] = true
			if typed.Key == "text-overlay-copy" {
				hasCopyTooltip = typed.Widget.(woxcomponent.IconButtonProps).OnHoverAt != nil
			}
		case woxwidget.TextBlock:
			hasTitle = typed.Value == "Summarize"
		case woxwidget.Image:
			hasIcon = true
		case woxwidget.Gesture:
			if typed.ID == "text-overlay-title-drag" && typed.OnDragStart != nil {
				hasTitleDrag = true
			}
		}
	})
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

func TestTextOverlayTitleSlotFollowsPlatformChrome(t *testing.T) {
	left, slot, horizontal := textOverlayTitleSlot("darwin", 420, 44, 300)
	if left != 0 || slot != 420 || horizontal != 0.5 {
		t.Fatalf("macOS title slot = left %v width %v align %v, want centered in the window", left, slot, horizontal)
	}
	left, slot, horizontal = textOverlayTitleSlot("windows", 420, 12, 300)
	if left != 12 || slot != 300 || horizontal != 0 {
		t.Fatalf("Windows title slot = left %v width %v align %v, want a leading cluster", left, slot, horizontal)
	}
}

func TestRuntimeTextTitleBarCentersTitleAndIcon(t *testing.T) {
	instance := &runtimeTextOverlay{
		options:   Options{Title: "翻译并显示", Closable: true},
		titleIcon: &woxui.Image{},
	}
	titleBar := instance.buildTitleBar(420, true, overlay.ThemeChrome{Foreground: woxui.Color{A: 255}}).(woxwidget.Stack)
	var cluster woxwidget.Align
	var clusterLeft float32
	found := false
	for _, child := range titleBar.Children {
		align, ok := child.Child.(woxwidget.Align)
		if !ok || align.Height != runtimeTextTitleBarHeight {
			continue
		}
		cluster = align
		clusterLeft = child.Left
		found = true
		break
	}
	if !found || cluster.Vertical != 0.5 {
		t.Fatalf("title cluster = %#v, want a full-height vertically centered row", cluster)
	}
	if runtime.GOOS == "darwin" {
		if cluster.Horizontal != 0.5 || cluster.Width != 420 || clusterLeft != 0 {
			t.Fatalf("macOS title cluster = left %v width %v align %v, want centered in the window", clusterLeft, cluster.Width, cluster.Horizontal)
		}
	} else if cluster.Horizontal != 0 || clusterLeft != 12 {
		t.Fatalf("title cluster = left %v align %v, want a leading Windows/Linux cluster", clusterLeft, cluster.Horizontal)
	}
	row, ok := cluster.Child.(woxwidget.Flex)
	if !ok || row.Axis != woxwidget.Horizontal || row.CrossAxisAlignment != woxwidget.CrossAxisCenter || len(row.Children) != 2 {
		t.Fatalf("title row = %#v, want icon and label on one centered line", cluster.Child)
	}
}

func walkTextOverlayTitleBar(widget woxwidget.Widget, visit func(woxwidget.Widget)) {
	visit(widget)
	switch typed := widget.(type) {
	case woxwidget.Stack:
		for _, child := range typed.Children {
			walkTextOverlayTitleBar(child.Child, visit)
		}
	case woxwidget.Align:
		walkTextOverlayTitleBar(typed.Child, visit)
	case woxwidget.Flex:
		for _, child := range typed.Children {
			walkTextOverlayTitleBar(child, visit)
		}
	case woxwidget.Container:
		walkTextOverlayTitleBar(typed.Child, visit)
	case woxwidget.Gesture:
		walkTextOverlayTitleBar(typed.Child, visit)
	}
}

func TestRuntimeTextTitleBarOmitsDragWhenNotMovable(t *testing.T) {
	instance := &runtimeTextOverlay{
		options: Options{Title: "Translate", Closable: true, ShowCopyButton: true},
	}
	titleBar := instance.buildTitleBar(420, true, overlay.ThemeChrome{Foreground: woxui.Color{R: 255, G: 255, B: 255, A: 255}}).(woxwidget.Stack)
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
	fill := textOverlaySurfaceFill()
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
	fill := textOverlaySurfaceFill()
	if panel.Color != fill {
		t.Fatalf("overlay panel fill = %#v, want %#v", panel.Color, fill)
	}
}

func TestApplyTextOverlayThemeFollowsLightTheme(t *testing.T) {
	overlay.SetThemeProvider(func() common.Theme {
		return common.Theme{AppBackgroundColor: "#F5F5F5", ToolbarFontColor: "#1C1C1E"}
	})
	defer overlay.SetThemeProvider(nil)
	opts := Options{Window: overlay.WindowOptions{ID: "ai-command"}}
	applyTextOverlayTheme(&opts)
	if !opts.Window.LightAppearance || !opts.Window.FollowsThemeAppearance {
		t.Fatalf("text overlay appearance = light %v follow %v, want the light Wox theme", opts.Window.LightAppearance, opts.Window.FollowsThemeAppearance)
	}
}

func TestRuntimeTextOverlayBuildPaintsThemeBackground(t *testing.T) {
	overlay.SetThemeProvider(func() common.Theme {
		return common.Theme{AppBackgroundColor: "rgba(22, 22, 26, 0.52)"}
	})
	defer overlay.SetThemeProvider(nil)
	instance := &runtimeTextOverlay{
		layout: runtimeTextLayout{
			windowSize:  woxui.Size{Width: 160, Height: 48},
			contentSize: woxui.Size{Width: 140, Height: 28},
			textWidth:   140,
		},
		options: Options{Message: "hello"},
	}
	root := instance.build(woxui.FrameInfo{Size: woxui.Size{Width: 160, Height: 48}}).(woxwidget.Stack)
	panel := root.Children[0].Child.(woxwidget.Container)
	want := overlay.CurrentThemeChrome().Background
	if panel.Color != want || panel.Color.A == 0 {
		t.Fatalf("overlay panel fill = %#v, want theme chrome %#v", panel.Color, want)
	}
	if runtime.GOOS == "linux" && panel.Color.A != 255 {
		t.Fatalf("linux overlay panel fill = %#v, want opaque because Linux has no acrylic", panel.Color)
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
