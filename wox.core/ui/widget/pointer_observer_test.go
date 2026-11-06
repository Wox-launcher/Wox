package widget

import (
	"fmt"
	"reflect"
	"testing"

	ui "wox/ui/runtime"
)

// TestPointerObserverPreservesInput observes logical coordinates even when raw
// descendants consume events, without adding taps or stealing text selection.
func TestPointerObserverPreservesInput(t *testing.T) {
	for _, raw := range []bool{false, true} {
		for _, scale := range []float32{1, 1.5, 2} {
			t.Run(fmt.Sprintf("raw=%v/DPI=%g", raw, scale), func(t *testing.T) {
				var observed []ui.PointerEvent
				taps, rawDown, starts, extends, ends := 0, 0, 0, 0, 0
				host := NewHost(func(ui.FrameInfo) Widget {
					child := Gesture{ID: "child", OnTap: func() { taps++ }, Child: Container{Width: 80, Height: 30},
						OnSelectionStart: func(ui.Point, ui.KeyModifiers) { starts++ }, OnSelectionExtend: func(ui.Point) { extends++ }, OnSelectionEnd: func() { ends++ }}
					if raw {
						child.OnPointer = func(e ui.PointerEvent) bool {
							if e.Kind == ui.PointerDown {
								rawDown++
							}
							return true
						}
					}
					return Stack{Children: []StackChild{{Left: -10, Top: 20, Child: Gesture{ID: "observer", OnPointerObserve: func(e ui.PointerEvent) { observed = append(observed, e) }, Child: Container{Padding: Insets{Left: 10}, Child: child}}}}}
				})
				host.AttachServices(&fakeHostServices{})
				defer host.Dispose()
				host.Frame(&ui.DisplayList{}, ui.FrameInfo{Size: ui.Size{Width: 160, Height: 100}, Scale: scale})
				for _, e := range []ui.PointerEvent{
					{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 10, Y: 25}},
					{Kind: ui.PointerMove, Position: ui.Point{X: 40, Y: 25}},
					{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 40, Y: 25}},
				} {
					host.Pointer(e)
				}
				if len(observed) != 3 || observed[0].Position != (ui.Point{X: 20, Y: 5}) || observed[2].Position != (ui.Point{X: 50, Y: 5}) {
					t.Fatalf("observed=%+v", observed)
				}
				if taps != 0 || raw && rawDown != 1 || !raw && (starts != 1 || extends == 0 || ends != 1) {
					t.Fatalf("routing changed: taps=%d raw=%d selection=%d/%d/%d", taps, rawDown, starts, extends, ends)
				}
			})
		}
	}
}

// TestPointerObserverRefresh uses current callbacks after layout and respects
// clips and an occluding modal even while the physical pointer is stationary.
func TestPointerObserverRefresh(t *testing.T) {
	var points []ui.Point
	var versions []int
	var hovered []bool
	left, version, visible := float32(10), 1, true
	observing := true
	host := NewHost(func(ui.FrameInfo) Widget {
		if !visible {
			return Container{}
		}
		v := version
		g := Gesture{ID: "observer", CoverHover: true, OnHover: func(in bool) { hovered = append(hovered, in) }, Child: Container{Width: 80, Height: 40}}
		if observing {
			g.OnPointerObserve = func(e ui.PointerEvent) { points = append(points, e.Position); versions = append(versions, v) }
		}
		return Clip{Width: 60, Height: 40, Child: Stack{Children: []StackChild{{Left: left, Child: g}}}}
	})
	host.AttachServices(&fakeHostServices{})
	defer host.Dispose()
	renderTestFrame(host)
	host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 30, Y: 10}})
	left, version = 20, 2
	renderTestFrame(host)
	if !reflect.DeepEqual(points, []ui.Point{{X: 20, Y: 10}, {X: 10, Y: 10}}) || !reflect.DeepEqual(versions, []int{1, 2}) {
		t.Fatalf("refresh=%v versions=%v", points, versions)
	}
	host.SetOverlay("", FocusScope{Key: "modal", Modal: true, Child: Container{Width: 100, Height: 80}})
	renderTestFrame(host)
	host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 30, Y: 10}})
	if len(points) != 2 || !reflect.DeepEqual(hovered, []bool{true, false}) {
		t.Fatalf("occlusion points=%v hovered=%v", points, hovered)
	}
	host.SetOverlay("", nil)
	observing = false
	renderTestFrame(host)
	visible = false
	renderTestFrame(host)
	if len(points) != 2 {
		t.Fatal("removed observer received stale callback")
	}
	visible, observing = true, true
	renderTestFrame(host)
	points = nil
	host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 70, Y: 10}})
	if len(points) != 0 {
		t.Fatal("observer received input beyond ancestor clip")
	}
}
