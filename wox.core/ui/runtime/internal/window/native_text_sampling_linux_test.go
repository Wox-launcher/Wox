//go:build linux

package window

import (
	"fmt"
	"testing"
)

// TestLinuxTextSampling keeps glyph pixels independent of fractional layout slots
// and subpixel translations when the line otherwise fits inside its bounds.
func TestLinuxTextSampling(t *testing.T) {
	target, skip, err := newLinuxOffscreenTarget(600, 240)
	if skip {
		t.Skip("Linux offscreen rendering requires a display")
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(target.destroy)
	for _, scale := range []float32{1, 1.25, 1.5, 2, 3, 1} {
		for _, size := range []float32{12, 15, 28} {
			t.Run(fmt.Sprintf("scale=%g/size=%g", scale, size), func(t *testing.T) {
				style := TextStyle{Size: size}
				color := Color{R: 255, G: 255, B: 255, A: 255}
				var reference DisplayList
				reference.DrawText("H5-TEST", Rect{X: 20 / scale, Y: 20 / scale, Width: 500 / scale, Height: 160 / scale}, style, color)
				want, err := target.render(&reference, scale)
				if err != nil {
					t.Fatal(err)
				}
				visible := false
				for i := 3; i < len(want); i += 4 {
					visible = visible || want[i] > 0
				}
				if !visible {
					t.Fatal("reference text did not paint any glyphs")
				}
				for _, test := range []struct {
					name        string
					fractional  bool
					translation Point
				}{
					{name: "fractional_slot", fractional: true},
					{name: "fractional_origin", translation: Point{X: 0.25 / scale, Y: -0.25 / scale}},
					{name: "translated_negative_origin", fractional: true, translation: Point{X: 80.25 / scale, Y: 60.25 / scale}},
				} {
					t.Run(test.name, func(t *testing.T) {
						bounds := Rect{X: 20 / scale, Y: 20 / scale, Width: 480 / scale, Height: 150 / scale}
						if test.fractional {
							bounds.Width -= 0.25 / scale
							bounds.Height -= 0.25 / scale
						}
						if test.translation.X > bounds.X {
							bounds.X -= 80 / scale
							bounds.Y -= 60 / scale
						}
						var d DisplayList
						d.PushTransform(TranslationTransform(test.translation.X, test.translation.Y))
						d.DrawText("H5-TEST", bounds, style, color)
						d.PopTransform()
						// Repeat to cover both a new raster and the texture cache hit.
						for range 2 {
							got, err := target.render(&d, scale)
							if err != nil {
								t.Fatal(err)
							}
							different := 0
							for i, value := range got {
								if delta := int(value) - int(want[i]); delta < -1 || delta > 1 {
									different++
								}
							}
							if different > 0 {
								t.Fatalf("layout slot or translation changed %d glyph channels", different)
							}
						}
					})
				}
			})
		}
	}
}
