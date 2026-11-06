package widget

import (
	"fmt"
	"reflect"
	"testing"

	ui "wox/ui/runtime"
)

// TestObservedPressPreservesDescendantInput covers normal taps and consumed raw input.
func TestObservedPressPreservesDescendantInput(t *testing.T) {
	for _, raw := range []bool{false, true} {
		for _, scale := range []float32{1, 1.5, 2} {
			generation, taps, rawEvents := 1, 0, 0
			var changes []string
			host := NewHost(func(ui.FrameInfo) Widget {
				current := generation
				child := Gesture{ID: "child", OnTap: func() { taps++ }, Child: Container{Width: 80, Height: 30}}
				if raw {
					child.OnPointer = func(e ui.PointerEvent) bool {
						if e.Kind == ui.PointerDown || e.Kind == ui.PointerUp {
							rawEvents++
						}
						return true
					}
				}
				return Gesture{ID: "observer", ObserveDescendants: true, OnPressChange: func(pressed bool) { changes = append(changes, fmt.Sprintf("%d:%v", current, pressed)) }, Child: Container{Child: child}}
			})
			host.AttachServices(&fakeHostServices{})
			frame := func() {
				host.Frame(&ui.DisplayList{}, ui.FrameInfo{Size: ui.Size{Width: 200, Height: 100}, Scale: scale, PixelSize: ui.PixelSize{Width: int(200 * scale), Height: int(100 * scale)}})
			}
			frame()
			host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 10, Y: 10}})
			generation = 2
			frame()
			host.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 10, Y: 10}})
			if !reflect.DeepEqual(changes, []string{"1:true", "2:false"}) || !raw && taps != 1 || raw && (taps != 0 || rawEvents != 2) {
				t.Fatalf("raw=%v DPI=%v: changes=%v taps=%d rawEvents=%d", raw, scale, changes, taps, rawEvents)
			}
			host.Dispose()
		}
	}
}

// TestObservedPressCancellation prevents stuck card state after input ownership changes.
func TestObservedPressCancellation(t *testing.T) {
	for _, reason := range []string{"leave", "blur", "drag", "remove-target", "remove-observer", "stop-observing"} {
		t.Run(reason, func(t *testing.T) {
			var changes []bool
			childVisible, observerVisible, observing := true, true, true
			drags := 0
			host := NewHost(func(ui.FrameInfo) Widget {
				var child Widget = Container{Width: 80, Height: 30}
				if childVisible {
					child = Gesture{ID: "child", OnTap: func() {}, OnDragStart: func() { drags++ }, Child: child}
				}
				if !observerVisible {
					return child
				}
				return Gesture{ID: "observer", ObserveDescendants: observing, OnPressChange: func(value bool) { changes = append(changes, value) }, Child: Container{Child: child}}
			})
			host.AttachServices(&fakeHostServices{})
			defer host.Dispose()
			host.SetWindowFocused(true)
			renderTestFrame(host)
			host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 10, Y: 10}})
			switch reason {
			case "leave":
				host.Pointer(ui.PointerEvent{Kind: ui.PointerLeave})
			case "blur":
				host.SetWindowFocused(false)
			case "drag":
				host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 20, Y: 10}})
			case "remove-target":
				childVisible = false
				renderTestFrame(host)
			case "remove-observer":
				observerVisible = false
				renderTestFrame(host)
			case "stop-observing":
				observing = false
				renderTestFrame(host)
			}
			host.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 20, Y: 10}})
			want := []bool{true, false}
			if reason == "remove-observer" {
				want = []bool{true}
			}
			if !reflect.DeepEqual(changes, want) || reason == "drag" && drags != 1 {
				t.Fatalf("%s: changes=%v, drags=%d", reason, changes, drags)
			}
		})
	}
}

// TestObservedPressOnOwnSurface avoids double notifications and ignores other mouse buttons.
func TestObservedPressOnOwnSurface(t *testing.T) {
	var changes []bool
	taps := 0
	host := NewHost(func(ui.FrameInfo) Widget {
		return Gesture{ID: "card", ObserveDescendants: true, OnPressChange: func(value bool) { changes = append(changes, value) }, OnTap: func() { taps++ }, Child: Container{Width: 80, Height: 30}}
	})
	host.AttachServices(&fakeHostServices{})
	defer host.Dispose()
	renderTestFrame(host)
	for _, button := range []ui.PointerButton{ui.PointerButtonSecondary, ui.PointerButtonPrimary} {
		host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: button, Position: ui.Point{X: 10, Y: 10}})
		renderTestFrame(host)
		host.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: button, Position: ui.Point{X: 10, Y: 10}})
	}
	if !reflect.DeepEqual(changes, []bool{true, false}) || taps != 1 {
		t.Fatalf("changes=%v taps=%d", changes, taps)
	}
}
