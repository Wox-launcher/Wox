//go:build windows

package window

import "testing"

// TestWARPFloatingMaterialBlursBackdrop catches software rendering silently skipping the material graph.
func TestWARPFloatingMaterialBlursBackdrop(t *testing.T) {
	for _, scale := range []float32{1, 1.5, 2.5} {
		pixels, status := testWindowsFloatingMaterial(scale)
		if status != 0 {
			t.Fatalf("scale=%v HRESULT=%#x", scale, uint32(status))
		}
		size := int(96 * scale)
		red := func(x, y int) byte { return pixels[(int(float32(y)*scale)*size+int(float32(x)*scale))*4] }
		// The production tone graph compresses white to 64 around the black tint.
		// Both sides must mix across the edge rather than remain at 0 and 64.
		if left, right := red(45, 48), red(51, 48); left < 3 || left > 32 || right < 33 || right > 62 {
			t.Fatalf("scale=%v backdrop remains sharp or is corrupted: left=%d right=%d", scale, left, right)
		}
		if red(45, 8) != 0 || red(51, 8) != 255 {
			t.Fatalf("scale=%v blur escaped material bounds", scale)
		}
	}
}
