package woxui

import "testing"

// checkEdgeFadePixels verifies both ramps preserve glass and do not mask later drawing.
func checkEdgeFadePixels(t *testing.T, render func(float32, float32, float32) ([]byte, int)) {
	t.Helper()
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for _, fades := range [][2]float32{{24, 24}, {0, 24}, {24, 0}, {0, 0}} {
			pixels, status := render(scale, fades[0], fades[1])
			if status == 1 {
				t.Skip("native display unavailable")
			}
			if status != 0 {
				t.Fatalf("native fade status=%d", status)
			}
			size := int(96 * scale)
			sample := func(y float32) []byte {
				offset := (int(y*scale)*size + int(48*scale)) * 4
				return pixels[offset : offset+4]
			}
			for _, y := range []float32{9, 86} {
				c := sample(y)
				faded := fades[0] > 0
				if y > 48 {
					faded = fades[1] > 0
				}
				if faded && (c[0] > 35 || c[2] < 125 || c[3] < 128 || c[3] > 150) {
					t.Fatalf("scale=%v fades=%v y=%v lost glass: %v", scale, fades, y, c)
				}
				if !faded && c[0] < 250 {
					t.Fatalf("unfaded edge dimmed: %v", c)
				}
			}
			if c := sample(48); c[0] != 255 || c[3] != 255 {
				t.Fatalf("center dimmed: %v", c)
			}
			if c := sample(93); c[0] != 255 || c[1] != 0 || c[3] != 255 {
				t.Fatalf("mask leaked into toolbar: %v", c)
			}
		}
	}
}
