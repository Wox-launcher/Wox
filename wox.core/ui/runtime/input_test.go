package woxui

import "testing"

func TestPointerPositionChanged(t *testing.T) {
	position := Point{X: 100, Y: 200}
	if pointerPositionChanged(position, position, true) {
		t.Fatal("unchanged known pointer position reported movement")
	}
	if !pointerPositionChanged(position, Point{X: 101, Y: 200}, true) {
		t.Fatal("changed pointer position did not report movement")
	}
	if !pointerPositionChanged(position, position, false) {
		t.Fatal("first pointer position did not report movement")
	}
}
