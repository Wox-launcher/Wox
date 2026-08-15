package mouse

import "testing"

func TestLogicalDesktopPointUsesMonitorScale(t *testing.T) {
	point := logicalDesktopPoint(150, 300, 1.5)
	if point.X != 100 || point.Y != 200 {
		t.Fatalf("logical point = %+v, want 100x200 at 150%% scale", point)
	}
	fallback := logicalDesktopPoint(40, 80, 0)
	if fallback.X != 40 || fallback.Y != 80 {
		t.Fatalf("invalid scale fallback = %+v, want raw pixels", fallback)
	}
}
