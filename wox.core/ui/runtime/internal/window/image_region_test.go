//go:build windows || darwin || linux

package window

import (
	"image"
	"image/color"
	"testing"
)

// TestNativeImageRegion checks atlas sampling across DPI and shared image identities.
func TestNativeImageRegion(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 32, 32))
	colors := []color.RGBA{{R: 255, A: 255}, {G: 255, A: 255}, {B: 255, A: 255}, {R: 255, G: 255, A: 255}}
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			source.SetRGBA(x, y, colors[x/16+(y/16)*2])
		}
	}
	atlas, err := NewImage(source)
	if err != nil {
		t.Fatal(err)
	}
	_, draw := nativeTransformTarget(t, 160, 120)
	for _, scale := range []float32{1, 1.25, 1.5, 2.5} {
		d := &DisplayList{}
		for i := 0; i < 4; i++ {
			d.DrawImageRegion(atlas, Rect{X: float32(i%2) * 16, Y: float32(i/2) * 16, Width: 16, Height: 16}, Rect{X: float32(i) * 15, Y: 5, Width: 12, Height: 12})
		}
		pixels := draw(d, scale)
		for i, c := range colors {
			checkNativeTransformPixel(t, pixels, scale, float32(i)*15+6, 11, c)
		}
		software, _ := NewSoftwareRenderer(160, 120)
		if err := software.Render(d); err != nil {
			t.Fatal(err)
		}
		for i, c := range colors {
			if got := software.RGBA().RGBAAt(i*15+6, 11); got != c {
				t.Fatalf("software region %d: %v", i, got)
			}
		}
	}
}
