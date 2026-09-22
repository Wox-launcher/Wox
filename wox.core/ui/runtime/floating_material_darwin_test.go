//go:build darwin

package woxui

import (
	"fmt"
	"testing"
)

// TestDarwinMaterialClipsAndStacks exercises actual CoreGraphics pixels at display scales.
func TestDarwinMaterialClipsAndStacks(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		t.Run(fmt.Sprint(scale), func(t *testing.T) {
			baseline, size := testRenderDarwinMaterial(scale, 255, 0)
			clipped, _ := testRenderDarwinMaterial(scale, 255, 1)
			stacked, _ := testRenderDarwinMaterial(scale, 255, 2)
			overlay, _ := testRenderDarwinMaterial(scale, 255, 3)
			if len(baseline) == 0 || len(clipped) != len(baseline) || len(stacked) != len(baseline) || len(overlay) != len(baseline) {
				t.Fatal("native bitmap rendering failed")
			}
			pixel := func(data []byte, x, y float32) [4]byte {
				offset := (int(y*scale)*size + int(x*scale)) * 4
				return [4]byte(data[offset : offset+4])
			}
			for _, point := range []Point{{X: 40, Y: 24}, {X: 82, Y: 40}, {X: 40, Y: 82}, {X: 79, Y: 79}} {
				if got, want := pixel(clipped, point.X, point.Y), pixel(baseline, point.X, point.Y); got != want {
					t.Fatalf("material escaped clip/corner at %+v: %v != %v", point, got, want)
				}
			}
			if pixel(clipped, 48, 48) == pixel(baseline, 48, 48) {
				t.Fatal("material did not paint")
			}
			if pixel(stacked, 40, 48) == pixel(clipped, 40, 48) {
				t.Fatal("stacked material did not sample and tint lower surface")
			}
			if got := pixel(overlay, 48, 48); got != [4]byte{100, 80, 60, 255} {
				t.Fatalf("WebView overlay tint = %v, want opaque authored color", got)
			}
			upper, lower := pixel(clipped, 16, 32), pixel(clipped, 16, 70)
			if upper[2] <= lower[2] || upper[0] >= lower[0] {
				t.Fatalf("blurred backdrop was flipped: upper=%v lower=%v", upper, lower)
			}
			// A sharp white stripe becomes a smooth backdrop rather than remaining readable.
			left, center := pixel(clipped, 40, 48), pixel(clipped, 48, 48)
			if int(center[0])-int(left[0]) > 15 {
				t.Fatalf("backdrop stripe was not blurred: left=%v center=%v", left, center)
			}
		})
	}
}

// TestDarwinMaterialPreservesBackdropAlpha catches double-compositing the sampled pixels.
func TestDarwinMaterialPreservesBackdropAlpha(t *testing.T) {
	pixels, size := testRenderDarwinMaterial(2, 96, 1)
	if len(pixels) == 0 {
		t.Fatal("native bitmap rendering failed")
	}
	alpha := pixels[(48*2*size+16*2)*4+3]
	// 96 + (255-96)*64/255 = 136, allowing premultiplied 8-bit rounding.
	if alpha < 135 || alpha > 137 {
		t.Fatalf("backdrop alpha = %d, want 136", alpha)
	}
}
