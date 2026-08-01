package component

import "testing"

func TestTextFieldScrolledOffsetConsumesOnlyMovableWheelDeltas(t *testing.T) {
	if next, changed := textFieldScrolledOffset(0, 240, -40); next != 40 || !changed {
		t.Fatalf("scroll down = %.0f, %v, want 40, true", next, changed)
	}
	if next, changed := textFieldScrolledOffset(240, 240, -20); next != 240 || changed {
		t.Fatalf("scroll at bottom = %.0f, %v, want 240, false", next, changed)
	}
	if next, changed := textFieldScrolledOffset(40, 240, 20); next != 20 || !changed {
		t.Fatalf("scroll up = %.0f, %v, want 20, true", next, changed)
	}
}
