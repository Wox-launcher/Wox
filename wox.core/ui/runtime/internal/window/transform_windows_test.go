package window

import (
	"bytes"
	"image"
	"testing"
)

// windowsTransformTarget keeps the production native encoder alive across frame and DPI changes.
func windowsTransformTarget(t *testing.T, width, height int) (*nativeRenderer, func(*DisplayList, float32) *image.RGBA) {
	t.Helper()
	r, err := newWindowsOffscreenRenderer(width, height)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.destroy)
	return r, func(d *DisplayList, scale float32) *image.RGBA {
		t.Helper()
		if err := r.render(d, scale); err != nil {
			t.Fatal(err)
		}
		pixels, err := readWindowsOffscreenPixels(r, width, height)
		if err != nil {
			t.Fatal(err)
		}
		return &image.RGBA{Pix: pixels, Stride: width * 4, Rect: image.Rect(0, 0, width, height)}
	}
}

// TestWindowsBackdropResourcesBounded verifies retained targets survive reuse, obey the cap, and release while hidden.
func TestWindowsBackdropResourcesBounded(t *testing.T) {
	for _, size := range []int{128, 1024} {
		r, draw := windowsTransformTarget(t, size, size)
		d := &DisplayList{}
		d.Clear(Color{B: 255, A: 128})
		bounds := Rect{Width: 64, Height: 64}
		for i := 0; i < 3; i++ {
			d.BeginOpacity(bounds, 0.5)
		}
		d.FillRect(bounds, Color{R: 255, A: 255})
		d.commands = append(d.commands, displayCommand{kind: displayCommandFloatingMaterial, rect: bounds, material: FloatingMaterialStyle{Brightness: 1, Saturation: 1}})
		for i := 0; i < 3; i++ {
			d.EndOpacity(bounds, 0.5)
		}
		want := draw(d, 1)
		for i := 0; i < 2; i++ {
			if got := draw(d, 1); !bytes.Equal(got.Pix, want.Pix) {
				t.Fatal("reused composite targets contain stale drawing")
			}
			if used := r.residentBytes(); used <= 0 || used > 16*1024*1024 {
				t.Fatalf("unexpected retained composite bytes: %d", used)
			}
		}
		if err := r.clearImageCache(); err != nil {
			t.Fatal(err)
		}
		if used := r.residentBytes(); used != 0 {
			t.Fatalf("hidden renderer retained %d bytes", used)
		}
		if got := draw(d, 1); !bytes.Equal(got.Pix, want.Pix) {
			t.Fatal("cleared composite pool changed rendering")
		}
	}
}

// nativeTransformTarget supplies the shared native pixel suite with the Windows production encoder.
func nativeTransformTarget(t *testing.T, width, height int) (transformTestRenderer, func(*DisplayList, float32) *image.RGBA) {
	return windowsTransformTarget(t, width, height)
}
