package woxui

import "testing"

func TestTransformSceneKeepsImageAndVectorCoordinatesAligned(t *testing.T) {
	list := &DisplayList{}
	list.StrokeRoundedRect(Rect{X: 100, Y: 60, Width: 80, Height: 40}, 8, 3, Color{A: 255})
	list.DrawText("text", Rect{X: 100, Y: 60, Width: 80, Height: 40}, TextStyle{Size: 20}, Color{A: 255})
	list.PushClipRect(Rect{X: 100, Y: 60, Width: 80, Height: 40})
	list.FillConvexPolygon([]Point{{X: 100, Y: 60}, {X: 180, Y: 60}, {X: 100, Y: 100}}, Color{A: 255})
	list.PopClipRect()
	list.TransformScene(0.5, Point{X: 7, Y: 11})
	if list.commands[0].rect != (Rect{X: 57, Y: 41, Width: 40, Height: 20}) || list.commands[0].stroke != 1.5 || list.commands[0].radius != 4 {
		t.Fatal("vector geometry was not transformed")
	}
	if list.commands[1].style.Size != 10 || list.commands[2].rect != list.commands[0].rect || list.commands[3].points[0] != (Point{X: 57, Y: 41}) {
		t.Fatal("text, clip, and polygon coordinates diverged")
	}
}
