package component

import (
	"testing"

	woxwidget "wox/ui/widget"
)

func TestSingleLineTextFieldDefaultsToVerticalCenter(t *testing.T) {
	singleLine := WoxTextField(TextFieldProps{ID: "single"}).(woxwidget.Stateful).Widget.(TextFieldProps)
	if singleLine.TextAlignmentY != 0.5 {
		t.Fatalf("single-line vertical alignment = %v, want 0.5", singleLine.TextAlignmentY)
	}

	multiline := WoxTextField(TextFieldProps{ID: "multi", MaxLines: 2}).(woxwidget.Stateful).Widget.(TextFieldProps)
	if multiline.TextAlignmentY != 0 {
		t.Fatalf("multiline vertical alignment = %v, want 0", multiline.TextAlignmentY)
	}
}

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
