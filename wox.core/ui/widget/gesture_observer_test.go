package widget

import (
	"testing"
	ui "wox/ui/runtime"
)

// TestGestureObservesDescendantsPreservingActivation verifies generic event observation independently of WoxView.
func TestGestureObservesDescendantsPreservingActivation(t *testing.T) {
	taps, doubles, menus := 0, 0, 0
	host := NewHost(func(ui.FrameInfo) Widget {
		return Gesture{ID: "observer", ObserveDescendants: true,
			OnDoubleTap: func() { doubles++ }, OnSecondaryTapDown: func(ui.Point) { menus++ },
			Child: Container{Child: Gesture{ID: "child", OnTap: func() { taps++ }, Child: Container{Width: 80, Height: 30}}},
		}
	})
	host.AttachServices(&fakeHostServices{})
	defer host.Dispose()
	host.Frame(&ui.DisplayList{}, ui.FrameInfo{Size: ui.Size{Width: 200, Height: 100}, Scale: 2, PixelSize: ui.PixelSize{Width: 400, Height: 200}})
	for range 4 {
		host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 10, Y: 10}})
		host.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 10, Y: 10}})
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonSecondary, Position: ui.Point{X: 10, Y: 10}})
	if taps != 4 || doubles != 2 || menus != 1 {
		t.Fatalf("taps=%d doubles=%d menus=%d", taps, doubles, menus)
	}
}
