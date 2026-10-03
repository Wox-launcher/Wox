//go:build darwin

package woxui

import "testing"

// TestDarwinFractionalDamagePreservesPixels catches dark outlines left by subpixel cleanup clips.
func TestDarwinFractionalDamagePreservesPixels(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2, 2.5} {
		for _, origin := range []Point{{X: 8.3, Y: 10.2}, {X: -1.3, Y: -2.7}, {X: 62.2, Y: 61.8}} {
			if result := testDarwinFractionalDamage(scale, origin); result != 0 {
				t.Errorf("scale %v, origin %+v: repainting an unchanged scene altered %d pixels (negative values indicate native errors)", scale, origin, result)
			}
		}
	}
}
