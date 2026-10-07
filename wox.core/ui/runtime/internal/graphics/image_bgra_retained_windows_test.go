//go:build windows

package graphics

import (
	"image"
	"testing"
)

func TestWindowsPackedBGRAPreservesNativePixels(t *testing.T) {
	pixels := []byte{30, 20, 10, 0, 90, 80, 70, 12}
	source := &PackedBGRA{Pix: pixels, Stride: 8, Rect: image.Rect(0, 0, 2, 1)}
	if got := source.RGBAAt(0, 0); got.R != 10 || got.G != 20 || got.B != 30 || got.A != 255 {
		t.Fatalf("first pixel = %+v", got)
	}
	if got := source.RGBAAt(1, 0); got.R != 70 || got.G != 80 || got.B != 90 || got.A != 255 {
		t.Fatalf("second pixel = %+v", got)
	}
	if pixels[0] != 30 || pixels[3] != 0 {
		t.Fatalf("native pixels were modified: %v", pixels)
	}
	retained, err := source.RetainedRendererImage()
	if err != nil {
		t.Fatal(err)
	}
	if retained.format != imagePixelFormatBGRAOpaque || &retained.pixels[0] != &pixels[0] {
		t.Fatal("renderer image did not retain the BGRA capture")
	}
}

func TestWindowsPackedBGRAWriteRGBACopiesCrop(t *testing.T) {
	pixels := []byte{
		1, 2, 3, 0, 4, 5, 6, 0, 7, 8, 9, 0, 10, 11, 12, 0,
		13, 14, 15, 0, 16, 17, 18, 0, 19, 20, 21, 0, 22, 23, 24, 0,
	}
	source := &PackedBGRA{Pix: pixels, Stride: 16, Rect: image.Rect(0, 0, 4, 2)}
	dst := image.NewRGBA(image.Rect(1, 1, 3, 2))
	source.WriteRGBA(dst, image.Pt(1, 1))
	if got := dst.RGBAAt(1, 1); got.R != 18 || got.G != 17 || got.B != 16 || got.A != 255 {
		t.Fatalf("cropped BGRA pixel = %+v, want RGB 18,17,16", got)
	}
	if pixels[20] != 16 {
		t.Fatalf("source pixels were modified: %v", pixels)
	}
}
