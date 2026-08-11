//go:build windows

package screenshot

import "testing"

func TestWindowsScrollingCaptureBorderStaysOutsideSelection(t *testing.T) {
	selection := Rect{X: 100, Y: 200, Width: 300, Height: 400}
	rects := windowsScrollingCaptureBorderRects(selection, 3)

	for index, rect := range rects {
		if screenshotEditorRectsOverlap(rect, selection) {
			t.Fatalf("border %d overlaps capture selection: border=%+v selection=%+v", index, rect, selection)
		}
	}
	if rects[0] != (Rect{X: 97, Y: 197, Width: 306, Height: 3}) {
		t.Fatalf("top border = %+v", rects[0])
	}
	if rects[1] != (Rect{X: 400, Y: 200, Width: 3, Height: 400}) {
		t.Fatalf("right border = %+v", rects[1])
	}
	if rects[2] != (Rect{X: 97, Y: 600, Width: 306, Height: 3}) {
		t.Fatalf("bottom border = %+v", rects[2])
	}
	if rects[3] != (Rect{X: 97, Y: 200, Width: 3, Height: 400}) {
		t.Fatalf("left border = %+v", rects[3])
	}
}
