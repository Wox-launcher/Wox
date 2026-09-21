package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TestLauncherContentInsetMovesEverySectionTogether uses logical bounds across DPI transitions.
func TestLauncherContentInsetMovesEverySectionTogether(t *testing.T) {
	for _, layout := range []struct{ bottom, overlay bool }{{}, {bottom: true}, {overlay: true}} {
		bounds := map[string]woxui.Rect{}
		section := func(name string, width, height float32) woxwidget.Widget {
			return woxwidget.Painter{Width: width, Height: height, Paint: func(_ *woxui.DisplayList, rect woxui.Rect) { bounds[name] = rect }}
		}
		contentHeight := float32(126)
		if layout.overlay {
			contentHeight += 30
		}
		host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
			return LauncherView(LauncherViewProps{Width: 400, Height: 220,
				Theme:  woxcomponent.Theme{AppContentInset: 12, AppContentBorderRadius: 8, AppContentBackground: woxui.Color{A: 180}},
				Header: section("header", 376, 40), Content: section("content", 376, contentHeight), Footer: section("footer", 376, 30),
				QueryAtBottom: layout.bottom, FooterOverlay: layout.overlay,
				Floating: &LauncherFloatingView{Left: 220, Bottom: 35, AnchorBottom: true, Child: section("floating", 140, 60)},
				Overlay:  section("overlay", 376, 196),
			})
		})
		host.AttachServices(actionSearchHostServices{})
		for _, scale := range []float32{1, 1.25, 1.5, 2, 1} {
			clear(bounds)
			host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 400, Height: 220}, Scale: scale, PixelSize: woxui.PixelSize{Width: int(400 * scale), Height: int(220 * scale)}})
			headerY, contentY := float32(12), float32(52)
			if layout.bottom {
				headerY, contentY = 138, 12
			}
			for name, want := range map[string]woxui.Rect{
				"header": {X: 12, Y: headerY, Width: 376, Height: 40}, "content": {X: 12, Y: contentY, Width: 376, Height: contentHeight},
				"footer": {X: 12, Y: 178, Width: 376, Height: 30}, "floating": {X: 232, Y: 113, Width: 140, Height: 60}, "overlay": {X: 12, Y: 12, Width: 376, Height: 196},
			} {
				if bounds[name] != want {
					t.Fatalf("scale %v layout %+v %s: got %+v want %+v", scale, layout, name, bounds[name], want)
				}
			}
		}
	}
}

func TestBorderDragMoveAreaProvidesFourEdgeDragGestures(t *testing.T) {
	dragged := 0
	area := BorderDragMoveArea(100, 80, woxwidget.UniformInsets(5), woxwidget.Container{}, func() { dragged++ }).(woxwidget.Stack)
	if len(area.Children) != 5 {
		t.Fatalf("border drag child count = %d, want content plus four edges", len(area.Children))
	}

	wantPositions := []struct {
		top           float32
		bottom        float32
		anchorRight   bool
		anchorBottom  bool
		stretchWidth  bool
		stretchHeight bool
	}{
		{stretchWidth: true},
		{anchorBottom: true, stretchWidth: true},
		{top: 5, bottom: 5, stretchHeight: true},
		{top: 5, bottom: 5, anchorRight: true, stretchHeight: true},
	}
	for index, want := range wantPositions {
		child := area.Children[index+1]
		if child.Top != want.top || child.Bottom != want.bottom || child.AnchorRight != want.anchorRight || child.AnchorBottom != want.anchorBottom || child.StretchWidth != want.stretchWidth || child.StretchHeight != want.stretchHeight {
			t.Fatalf("edge %d layout = %+v, want top/bottom %.0f/%.0f anchors %v/%v stretch %v/%v", index, child, want.top, want.bottom, want.anchorRight, want.anchorBottom, want.stretchWidth, want.stretchHeight)
		}
		gesture, ok := child.Child.(woxwidget.Gesture)
		if !ok || gesture.OnDragStart == nil {
			t.Fatalf("edge %d does not expose a drag gesture", index)
		}
		gesture.OnDragStart()
	}
	if dragged != 4 {
		t.Fatalf("drag callback count = %d, want four", dragged)
	}
}

func TestPreviewHoverCloseRevealsCloseButton(t *testing.T) {
	state := &previewHoverCloseState{}
	props := PreviewHoverCloseProps{Width: 500, Height: 300, Child: woxwidget.Container{}, Label: "Close", Theme: woxcomponent.Theme{}, OnClose: func() {}}

	hidden := state.Build(woxwidget.StateContext{}, props).(woxwidget.Gesture).Child.(woxwidget.Stack)
	if len(hidden.Children) != 1 {
		t.Fatalf("hidden child count = %d, want preview only", len(hidden.Children))
	}

	state.hovered = true
	shown := state.Build(woxwidget.StateContext{}, props).(woxwidget.Gesture).Child.(woxwidget.Stack)
	if len(shown.Children) != 2 || !shown.Children[1].AnchorRight || shown.Children[1].Right != 20 || shown.Children[1].Top != 20 {
		t.Fatalf("shown close placement = %#v", shown.Children)
	}
	button := shown.Children[1].Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	if button.OnHoverAt == nil || button.OnTap == nil || button.Width != 28 || button.Height != 28 {
		t.Fatalf("close icon button props = %+v, want hoverable 28x28 button", button)
	}
}

// TestThemeChromeDragLeavesContentInteractive exercises asymmetric insets across DPI changes.
func TestThemeChromeDragLeavesContentInteractive(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		drags, taps := 0, 0
		host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
			return LauncherView(LauncherViewProps{Width: 400, Height: 300,
				Theme:       woxcomponent.Theme{Surfaces: &woxcomponent.ThemeSurfaceSet{ContentInsets: woxwidget.Insets{Top: 80, Left: 30, Right: 40, Bottom: 25}}},
				OnDragStart: func() { drags++ },
				Content:     woxwidget.Gesture{OnTap: func() { taps++ }, Child: woxwidget.Container{Width: 330, Height: 195}},
			})
		})
		host.AttachServices(actionSearchHostServices{})
		host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 400, Height: 300}, Scale: scale, PixelSize: woxui.PixelSize{Width: int(400 * scale), Height: int(300 * scale)}})
		for _, point := range []woxui.Point{{X: 200, Y: 40}, {X: 15, Y: 150}, {X: 380, Y: 150}, {X: 200, Y: 285}} {
			host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: point})
			host.Pointer(woxui.PointerEvent{Kind: woxui.PointerMove, Button: woxui.PointerButtonPrimary, Position: woxui.Point{X: point.X + 10, Y: point.Y}})
			host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: point})
		}
		point := woxui.Point{X: 200, Y: 150}
		host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: point})
		host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: point})
		if drags != 4 || taps != 1 {
			t.Fatalf("scale %v: drags=%d taps=%d", scale, drags, taps)
		}
	}
}
