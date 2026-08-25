package widget

import "testing"

func TestScrollControllerLeavesOversizedIntersectingRange(t *testing.T) {
	controller := NewScrollController(120)
	controller.setGeometry(80, 280)
	if controller.EnsureVisible(0, 200) {
		t.Fatal("an already-visible tall range must not move the offset")
	}
	if controller.Offset() != 120 {
		t.Fatalf("offset = %v, want 120", controller.Offset())
	}
}

func TestScrollControllerStillRevealsClippedSmallRange(t *testing.T) {
	controller := NewScrollController(0)
	controller.setGeometry(40, 200)
	if !controller.EnsureVisible(70, 80) {
		t.Fatal("a clipped small range should scroll into view")
	}
	if controller.Offset() != 40 {
		t.Fatalf("offset = %v, want 40", controller.Offset())
	}
}

func TestScrollControllerRevealsOversizedRangeWhenCompletelyHidden(t *testing.T) {
	controller := NewScrollController(0)
	controller.setGeometry(80, 400)
	if !controller.EnsureVisible(200, 360) {
		t.Fatal("a tall range entirely below the viewport should scroll")
	}
	if controller.Offset() != 280 {
		t.Fatalf("offset = %v, want 280 so the trailing edge is visible", controller.Offset())
	}
}
