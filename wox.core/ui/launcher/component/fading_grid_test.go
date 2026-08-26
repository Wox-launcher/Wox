package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestFadingGridFadesBeforeItsBounds(t *testing.T) {
	radius := fadingGridRadius(188, 332)
	if radius != 88 {
		t.Fatalf("fade radius = %.0f, want 88 with an invisible outer buffer", radius)
	}
	if center := fadingGridAlpha(0, 0, radius, radius); center != 72 {
		t.Fatalf("center alpha = %d, want 72", center)
	}
	if edge := fadingGridAlpha(0, 132, radius, radius); edge != 0 {
		t.Fatalf("buffer alpha = %d, want fully transparent before the bounds", edge)
	}
	if nearEdge := fadingGridAlpha(0, 80, radius, radius); nearEdge > 2 {
		t.Fatalf("near-edge alpha = %d, want an imperceptible transition", nearEdge)
	}
}

func TestFadingGridSubdividesLinesForSmoothEdgeFade(t *testing.T) {
	grid := FadingGrid(FadingGridProps{Width: 56, Height: 56, RadiusX: 28, RadiusY: 28, Color: woxui.Color{A: 255}}).(woxwidget.Painter)
	displayList := &woxui.DisplayList{}
	grid.Paint(displayList, woxui.Rect{Width: 56, Height: 56})
	if displayList.CommandCount() != 40 {
		t.Fatalf("grid commands = %d, want 7-unit line segments instead of abrupt 28-unit cells", displayList.CommandCount())
	}
}
