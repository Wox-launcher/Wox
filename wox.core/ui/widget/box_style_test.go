package widget

import (
	"testing"

	ui "wox/ui/runtime"
)

// TestContainerTopBorderAndCorners checks translucent corners at negative desktop origins.
func TestContainerTopBorderAndCorners(t *testing.T) {
	color := ui.Color{R: 80, G: 160, A: 128}
	for _, floating := range []bool{false, true} {
		for _, radius := range []float32{0, 8} {
			w := Container{Width: 50, Height: 30, Radius: radius, Floating: floating,
				TopBorderColor: color, TopBorderWidth: 2, BottomBorderColor: color, BottomBorderWidth: 3,
				LeftBorderColor: color, LeftBorderWidth: 4, RightBorderColor: color, RightBorderWidth: 5}
			root := w.layout(context{}, constraints{width: 50, height: 30})
			actual, expected := &ui.DisplayList{}, &ui.DisplayList{}
			root.drawAt(actual, ui.Point{X: -120, Y: -70}, 0, 0, false, false, false, nil)
			bounds := ui.Rect{X: -120, Y: -70, Width: 50, Height: 30}
			if floating {
				expected.FloatingMaterial(bounds, radius, ui.Color{}, ui.Color{})
			}
			for _, strip := range []ui.Rect{
				{X: -120, Y: -70, Width: 50, Height: 2}, {X: -120, Y: -43, Width: 50, Height: 3},
				{X: -120, Y: -68, Width: 4, Height: 25}, {X: -75, Y: -68, Width: 5, Height: 25},
			} {
				if radius == 0 {
					expected.FillRect(strip, color)
				} else {
					expected.PushClipRect(strip)
					expected.FillRoundedRect(bounds, radius, color)
					expected.PopClipRect()
				}
			}
			if err := actual.Compare(expected); err != nil {
				t.Fatalf("floating %v radius %v: %v", floating, radius, err)
			}
		}
	}
}

// TestContainerOversizedTopBorderDoesNotDarkenBottom protects small boxes with large authored widths.
func TestContainerOversizedTopBorderDoesNotDarkenBottom(t *testing.T) {
	color := ui.Color{R: 100, A: 100}
	root := (Container{Width: 20, Height: 10, TopBorderWidth: 15, TopBorderColor: color, BottomBorderWidth: 15, BottomBorderColor: color}).layout(context{}, constraints{width: 20, height: 10})
	actual, expected := &ui.DisplayList{}, &ui.DisplayList{}
	root.draw(actual, 0, 0, false, false, false, nil)
	expected.FillRect(ui.Rect{Width: 20, Height: 10}, color)
	if err := actual.Compare(expected); err != nil {
		t.Fatal(err)
	}
}

// TestContainerForegroundEdgesSurviveOpaqueChild verifies paint order without changing input bounds.
func TestContainerForegroundEdgesSurviveOpaqueChild(t *testing.T) {
	fill, edge := ui.Color{R: 80, A: 255}, ui.Color{B: 200, A: 255}
	root := (Container{EdgeBordersOnTop: true, TopBorderColor: edge, TopBorderWidth: 2,
		Child: Gesture{ID: "child", OnTap: func() {}, Child: Container{Width: 60, Height: 20, Color: fill}},
	}).layout(context{}, constraints{width: 100, height: 100})
	actual, expected := &ui.DisplayList{}, &ui.DisplayList{}
	root.draw(actual, 0, 0, false, false, false, nil)
	expected.FillRoundedRect(ui.Rect{Width: 60, Height: 20}, 0, fill)
	expected.FillRect(ui.Rect{Width: 60, Height: 2}, edge)
	if err := actual.Compare(expected); err != nil {
		t.Fatal(err)
	}
	if hit := root.hitTest(ui.Point{X: 5, Y: 1}); hit == nil || hit.gesture == nil || hit.gesture.id != "child" {
		t.Fatal("foreground decoration stole pointer input")
	}
}

// TestClipSizeToChildKeepsIntrinsicGeometry clips overflow without creating a viewport-sized box.
func TestClipSizeToChildKeepsIntrinsicGeometry(t *testing.T) {
	root := (Clip{SizeToChild: true, Child: Container{Width: 60, Height: 30, Child: Flex{Axis: Horizontal, Children: []Widget{
		Container{Width: 40, Height: 20}, Gesture{ID: "overflow", OnTap: func() {}, Child: Container{Width: 50, Height: 20}},
	}}}}).layout(context{}, constraints{width: 200, height: 100})
	if root.bounds.Width != 60 || root.bounds.Height != 30 || !root.clip {
		t.Fatalf("clip changed layout: %+v", root.bounds)
	}
	if hit := root.hitTest(ui.Point{X: 50, Y: 10}); hit == nil || hit.gesture == nil {
		t.Fatal("visible child is not interactive")
	}
	if hit := root.hitTest(ui.Point{X: 70, Y: 10}); hit != nil {
		t.Fatal("clipped overflow remained interactive")
	}
	constrained := (Clip{SizeToChild: true, Child: Container{Width: 10, Height: 10}}).layout(context{}, constraints{minWidth: 80, width: 80, minHeight: 50, height: 50})
	if constrained.bounds.Width != 80 || constrained.bounds.Height != 50 {
		t.Fatal("clip lost the parent's tight constraints")
	}
}
