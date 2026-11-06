package widget

import (
	"testing"

	ui "wox/ui/runtime"
)

// TestOpacityKeepsOverflowAndLayout detects an accidental clip at the group's own bounds.
func TestOpacityKeepsOverflowAndLayout(t *testing.T) {
	red, green := ui.Color{R: 255, A: 255}, ui.Color{G: 255, A: 255}
	content := Container{Width: 40, Height: 20, Child: Flex{Axis: Horizontal, Children: []Widget{
		Container{Width: 40, Height: 20, Color: red}, Container{Width: 40, Height: 20, Color: green},
	}}}
	for _, alpha := range []float32{0, 0.5, 1} {
		root := (Opacity{Value: alpha, Child: content}).layout(context{}, constraints{width: 100, height: 100})
		if root.bounds.Width != 40 || root.bounds.Height != 20 {
			t.Fatalf("opacity changed intrinsic layout: %+v", root.bounds)
		}
		actual, expected := &ui.DisplayList{}, &ui.DisplayList{}
		root.drawAt(actual, ui.Point{X: -10, Y: 10}, 0, 0, false, false, false, nil)
		if alpha > 0 {
			bounds := ui.Rect{X: -10, Y: 10, Width: 80, Height: 20}
			if alpha < 1 {
				expected.BeginOpacity(bounds, alpha)
			}
			expected.FillRoundedRect(ui.Rect{X: -10, Y: 10, Width: 40, Height: 20}, 0, red)
			expected.FillRoundedRect(ui.Rect{X: 30, Y: 10, Width: 40, Height: 20}, 0, green)
			if alpha < 1 {
				expected.EndOpacity(bounds, alpha)
			}
		}
		if err := actual.Compare(expected); err != nil {
			t.Fatal(err)
		}
	}
}

// TestOpacityPreservesFocusAndInput verifies the property changes only painting, at changing display scales.
func TestOpacityPreservesFocusAndInput(t *testing.T) {
	alpha, taps := float32(0.5), 0
	host := NewHost(func(ui.FrameInfo) Widget {
		return Opacity{Value: alpha, Child: Focusable{Key: "control", OnKey: func(e ui.KeyEvent) bool {
			if e.Down && e.Key == ui.KeyEnter {
				taps++
				return true
			}
			return false
		}, Child: Gesture{OnTap: func() { taps++ }, Child: Container{Width: 80, Height: 32}}}}
	})
	host.AttachServices(&fakeHostServices{})
	defer host.Dispose()
	for i, scale := range []float32{1, 1.5, 2, 1} {
		alpha = []float32{0.5, 0, 1, 0.25}[i]
		host.Frame(&ui.DisplayList{}, ui.FrameInfo{Size: ui.Size{Width: 100, Height: 60}, PixelSize: ui.PixelSize{Width: int(100 * scale), Height: int(60 * scale)}, Scale: scale})
		if i == 0 {
			host.Key(ui.KeyEvent{Key: ui.KeyTab, Down: true})
		}
		if host.FocusedKey() != "control" {
			t.Fatal("opacity update lost focus")
		}
		host.Key(ui.KeyEvent{Key: ui.KeyEnter, Down: true})
		host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 10, Y: 10}})
		host.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 10, Y: 10}})
		if taps != (i+1)*2 {
			t.Fatal("opacity changed keyboard or pointer routing")
		}
	}
}
