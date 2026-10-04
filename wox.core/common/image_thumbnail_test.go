package common

import (
	"github.com/disintegration/imaging"
	"image/color"
	"testing"
)

func TestImageThumbnailPreservesContent(t *testing.T) {
	for _, size := range [][2]int{{576, 120}, {120, 576}, {100, 100}} {
		thumb := NewImageThumbnail(imaging.New(size[0], size[1], color.NRGBA{R: 255, A: 255}))
		bounds := thumb.Bounds()
		if max(bounds.Dx(), bounds.Dy()) != ImageThumbnailSize || bounds.Dx()*size[1] != bounds.Dy()*size[0] {
			t.Fatalf("unexpected thumbnail bounds: %v", bounds)
		}
		for _, point := range [][2]int{{0, 0}, {bounds.Dx() - 1, bounds.Dy() - 1}} {
			r, g, b, a := thumb.At(point[0], point[1]).RGBA()
			if r != 65535 || g != 0 || b != 0 || a != 65535 {
				t.Fatal("thumbnail cache must not add padding, background, or clipping")
			}
		}
	}
}
