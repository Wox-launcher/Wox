package window

import "testing"

// TestFloatingMaterialStyleRecordsIndependentControls guards alpha and damage dependencies.
func TestFloatingMaterialStyleRecordsIndependentControls(t *testing.T) {
	if nativeFloatingMaterialMode() != floatingMaterialRendered {
		t.Skip("no renderer material")
	}
	tint := Color{R: 22, G: 22, B: 26}
	style := FloatingMaterialStyle{Sigma: 24, Brightness: .8, Saturation: 0}
	d := &DisplayList{}
	d.FloatingMaterial(Rect{Width: 100, Height: 40}, 8, tint, Color{}, &style)
	if len(d.commands) != 1 || d.commands[0].material != style || d.commands[0].color != tint {
		t.Fatal("material controls changed tint or were not recorded")
	}
	if d.FloatingMaterialSampleMargin() != 72 {
		t.Fatal("sample halo did not grow with sigma")
	}
	style.Sigma = 2
	if d.commands[0].material.Sigma != 24 {
		t.Fatal("recorded material captured mutable options")
	}
	defaults := &DisplayList{}
	defaults.FloatingMaterial(Rect{Width: 100, Height: 40}, 8, tint, Color{})
	if defaults.commands[0].material != DefaultFloatingMaterialStyle() {
		t.Fatal("omitted controls changed defaults")
	}
	if displayCommandsEqual(d.commands[0], defaults.commands[0]) {
		t.Fatal("material change was invisible to retained display list")
	}
}

func TestFloatingMaterialZeroAndBounds(t *testing.T) {
	zero := FloatingMaterialStyle{}
	if zero.Resolved() != zero {
		t.Fatal("explicit zero was treated as omission")
	}
	excessive := FloatingMaterialStyle{Sigma: 1000, Brightness: -1, Saturation: 9}
	if got := excessive.Resolved(); got != (FloatingMaterialStyle{Sigma: 64, Brightness: 0, Saturation: 2}) {
		t.Fatalf("unbounded style: %+v", got)
	}
}
