//go:build windows

package screenshot

import "testing"

func TestWindowsScreenshotPointerCoordinateKeepsPixelCenterInsideTarget(t *testing.T) {
	tests := []struct {
		name   string
		origin int
		point  float32
		want   int32
	}{
		{name: "left or up half pixel", origin: 0, point: 99.5, want: 99},
		{name: "right or down half pixel", origin: 0, point: 101.5, want: 101},
		{name: "negative desktop origin", origin: -1920, point: 99.5, want: -1821},
		{name: "integer pointer coordinate", origin: 100, point: 25, want: 125},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if got := windowsScreenshotPointerCoordinate(testCase.origin, testCase.point); got != testCase.want {
				t.Fatalf("pointer coordinate = %d, want %d", got, testCase.want)
			}
		})
	}
}

func TestWindowsScreenshotArrowNudgeMovesExactlyOnePhysicalPixel(t *testing.T) {
	prepared := testScreenshotImage(t, 5, 5)
	tests := []struct {
		key   Key
		wantX int32
		wantY int32
	}{
		{key: KeyArrowLeft, wantX: 1, wantY: 2},
		{key: KeyArrowUp, wantX: 2, wantY: 1},
		{key: KeyArrowRight, wantX: 3, wantY: 2},
		{key: KeyArrowDown, wantX: 2, wantY: 3},
	}
	for _, testCase := range tests {
		point, ok := screenshotEditorNudgedPoint(prepared, Size{Width: 5, Height: 5}, Point{X: 2, Y: 2}, testCase.key)
		if !ok {
			t.Fatalf("nudge %q was rejected", testCase.key)
		}
		gotX := windowsScreenshotPointerCoordinate(0, point.X)
		gotY := windowsScreenshotPointerCoordinate(0, point.Y)
		if gotX != testCase.wantX || gotY != testCase.wantY {
			t.Fatalf("nudge %q moved pointer to (%d, %d), want (%d, %d)", testCase.key, gotX, gotY, testCase.wantX, testCase.wantY)
		}
	}
}

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
