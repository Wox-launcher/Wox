package window

import (
	"image"
	"testing"
)

type darwinTransformTestRenderer struct{ target *darwinOffscreenTarget }

func (r darwinTransformTestRenderer) render(d *DisplayList, scale float32) error {
	_, err := r.target.render(d, scale)
	return err
}

// nativeTransformTarget replays the shared pixel suite through the macOS production command encoder.
func nativeTransformTarget(t *testing.T, width, height int) (transformTestRenderer, func(*DisplayList, float32) *image.RGBA) {
	t.Helper()
	target, err := newDarwinOffscreenTarget(width, height)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(target.destroy)
	return darwinTransformTestRenderer{target}, func(d *DisplayList, scale float32) *image.RGBA {
		t.Helper()
		pixels, err := target.render(d, scale)
		if err != nil {
			t.Fatal(err)
		}
		return &image.RGBA{Pix: pixels, Stride: width * 4, Rect: image.Rect(0, 0, width, height)}
	}
}
