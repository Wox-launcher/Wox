package widget

import (
	"testing"
	ui "wox/ui/runtime"
)

// TestAnchoredOverlayClampsAndFlips tests logical placement independently of feature styling.
func TestAnchoredOverlayClampsAndFlips(t *testing.T) {
	for _, test := range []struct{ anchor, want ui.Rect }{
		{ui.Rect{X: 10, Y: 10, Width: 30, Height: 20}, ui.Rect{X: 10, Y: 34, Width: 60, Height: 30}},
		{ui.Rect{X: 180, Y: 80, Width: 20, Height: 20}, ui.Rect{X: 132, Y: 46, Width: 60, Height: 30}},
		{ui.Rect{X: -100, Y: -100, Width: 20, Height: 20}, ui.Rect{X: 8, Y: 8, Width: 60, Height: 30}},
	} {
		n := (AnchoredOverlay{Anchor: test.anchor, Gap: 4, Margin: 8, Child: Container{Width: 60, Height: 30}}).layout(context{}, constraints{width: 200, height: 100})
		if n.children[0].bounds != test.want {
			t.Fatalf("got %+v want %+v", n.children[0].bounds, test.want)
		}
		if n.hitTest(ui.Point{X: 10, Y: 10}) != nil {
			t.Fatal("noninteractive overlay intercepted pointer input")
		}
	}
}
