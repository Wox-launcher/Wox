package window

import (
	"image"
	"testing"
)

type linuxTransformTestRenderer struct{ target *linuxOffscreenTarget }

func (r linuxTransformTestRenderer) render(d *DisplayList, scale float32) error {
	_, err := r.target.render(d, scale)
	return err
}

// nativeTransformTarget replays shared pixel cases through the production Linux encoder.
func nativeTransformTarget(t *testing.T, width, height int) (transformTestRenderer, func(*DisplayList, float32) *image.RGBA) {
	t.Helper()
	target, skip, err := newLinuxOffscreenTarget(width, height)
	if skip {
		t.Skip("Linux offscreen rendering requires a display; use GDK_BACKEND=x11 xvfb-run")
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(target.destroy)
	return linuxTransformTestRenderer{target}, func(d *DisplayList, scale float32) *image.RGBA {
		t.Helper()
		pixels, err := target.render(d, scale)
		if err != nil {
			t.Fatal(err)
		}
		return &image.RGBA{Pix: pixels, Stride: width * 4, Rect: image.Rect(0, 0, width, height)}
	}
}
