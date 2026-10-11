//go:build windows

package window

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// TestWindowsCapturePNGPreservesCroppedPixels covers native BGRX conversion with
// a negative origin, an offset crop and a stride wider than the exported image.
func TestWindowsCapturePNGPreservesCroppedPixels(t *testing.T) {
	source := &PackedBGRA{
		Pix: []byte{
			1, 2, 3, 0, 4, 5, 6, 9, 7, 8, 9, 0,
			10, 11, 12, 0, 13, 14, 15, 0, 16, 17, 18, 8,
		},
		Stride: 12,
		Rect:   image.Rect(-3, -2, 0, 0),
	}
	crop := source.SubImage(image.Rect(-2, -2, 0, 0))
	path := filepath.Join(t.TempDir(), "capture.png")
	if err := writeWindowsCapturePNG(path, crop); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	decoded, err := png.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds() != image.Rect(0, 0, 2, 2) {
		t.Fatalf("capture bounds = %v", decoded.Bounds())
	}
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			want := crop.At(x+crop.Bounds().Min.X, y+crop.Bounds().Min.Y)
			r, g, b, a := decoded.At(x, y).RGBA()
			wr, wg, wb, wa := want.RGBA()
			if r != wr || g != wg || b != wb || a != wa {
				t.Fatalf("pixel (%d,%d) = %v, want %v", x, y, decoded.At(x, y), want)
			}
		}
	}
}
