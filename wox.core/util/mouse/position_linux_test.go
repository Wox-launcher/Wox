//go:build linux

package mouse

import "testing"

func TestObserveWindowPointerTracksTheActiveWindow(t *testing.T) {
	ObserveWindowPointer(1, Point{X: 10, Y: 20}, true)
	point, ok := observedWindowPointer()
	if !ok || point.X != 10 || point.Y != 20 {
		t.Fatalf("observed pointer = %+v ok=%v, want 10,20", point, ok)
	}

	ObserveWindowPointer(2, Point{X: 40, Y: 50}, true)
	point, ok = observedWindowPointer()
	if !ok || point.X != 40 || point.Y != 50 {
		t.Fatalf("second window pointer = %+v ok=%v, want 40,50", point, ok)
	}

	ObserveWindowPointer(1, Point{X: 10, Y: 20}, false)
	point, ok = observedWindowPointer()
	if !ok || point.X != 40 || point.Y != 50 {
		t.Fatalf("leave on an inactive window = %+v ok=%v, want to keep window 2", point, ok)
	}

	ObserveWindowPointer(2, Point{}, false)
	if _, ok = observedWindowPointer(); ok {
		t.Fatal("leave on the active window must clear the observed pointer")
	}
}
