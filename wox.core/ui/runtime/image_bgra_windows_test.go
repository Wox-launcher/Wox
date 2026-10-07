//go:build windows

package woxui

import (
	"image"
	"image/draw"
	"testing"
)

// TestPackedBGRACopy checks physical pixel crops, row padding, and negative desktop origins against image/draw.
func TestPackedBGRACopy(t *testing.T) {
	source := &PackedBGRA{Rect: image.Rect(-12, -6, 12, 6), Stride: 28 * 4, Pix: make([]byte, 28*4*12)}
	for i := range source.Pix {
		source.Pix[i] = byte(i * 31)
	}
	for _, test := range []struct {
		bounds image.Rectangle
		origin image.Point
	}{
		{image.Rect(0, 0, 24, 12), source.Rect.Min},
		{image.Rect(-4, 2, 6, 8), image.Pt(-9, -3)},
		{image.Rect(3, -2, 17, 8), image.Pt(-18, -9)},
		{image.Rect(-8, -4, 8, 4), image.Pt(6, 1)},
		{image.Rect(0, 0, 4, 4), image.Pt(50, 50)},
	} {
		got := image.NewRGBA(test.bounds.Inset(-2)).SubImage(test.bounds).(*image.RGBA)
		want := image.NewRGBA(test.bounds)
		for i := range got.Pix {
			got.Pix[i] = 53
		}
		for i := range want.Pix {
			want.Pix[i] = 53
		}
		source.WriteRGBA(got, test.origin)
		draw.Draw(want, test.bounds, source, test.origin, draw.Src)
		for y := test.bounds.Min.Y; y < test.bounds.Max.Y; y++ {
			for x := test.bounds.Min.X; x < test.bounds.Max.X; x++ {
				if got.RGBAAt(x, y) != want.RGBAAt(x, y) {
					t.Fatalf("origin=%v pixel (%d,%d) = %v, want %v", test.origin, x, y, got.RGBAAt(x, y), want.RGBAAt(x, y))
				}
			}
		}
	}
	// A cropped capture retains its parent's stride while its buffer starts at the crop.
	crop := source.SubImage(image.Rect(-9, -4, 8, 5)).(*PackedBGRA)
	got, want := image.NewRGBA(image.Rect(0, 0, 14, 7)), image.NewRGBA(image.Rect(0, 0, 14, 7))
	crop.WriteRGBA(got, crop.Rect.Min)
	draw.Draw(want, want.Rect, crop, crop.Rect.Min, draw.Src)
	for i := range got.Pix {
		if got.Pix[i] != want.Pix[i] {
			t.Fatal("cropped BGRA capture lost its origin or row stride")
		}
	}
}

// TestPackedBGRACopyShortBuffers leaves incomplete source or destination pixels untouched at the native boundary.
func TestPackedBGRACopyShortBuffers(t *testing.T) {
	for _, lengths := range [][2]int{{7, 24}, {24, 7}, {15, 24}, {24, 15}, {23, 24}, {24, 23}} {
		source := &PackedBGRA{Rect: image.Rect(0, 0, 3, 2), Stride: 12, Pix: make([]byte, lengths[0])}
		for i := range source.Pix {
			source.Pix[i] = byte(i)
		}
		destination := &image.RGBA{Rect: source.Rect, Stride: 12, Pix: make([]byte, lengths[1])}
		source.WriteRGBA(destination, source.Rect.Min)
		for offset := range destination.Pix {
			pixel := offset / 4 * 4
			var want byte
			if pixel+4 <= len(source.Pix) && pixel+4 <= len(destination.Pix) {
				channel := offset % 4
				if channel == 3 {
					want = 255
				} else {
					want = byte(pixel + 2 - channel)
				}
			}
			if destination.Pix[offset] != want {
				t.Fatalf("lengths=%v byte %d = %d, want %d", lengths, offset, destination.Pix[offset], want)
			}
		}
	}
}

// BenchmarkPackedBGRACopy measures the desktop detachment needed before screenshot completion.
func BenchmarkPackedBGRACopy(b *testing.B) {
	bounds := image.Rect(-2560, -1000, 2576, 1792)
	source := &PackedBGRA{Rect: bounds, Stride: bounds.Dx() * 4, Pix: make([]byte, bounds.Dx()*bounds.Dy()*4)}
	destination := image.NewRGBA(image.Rectangle{Max: bounds.Size()})
	b.ReportAllocs()
	b.SetBytes(int64(len(source.Pix)))
	b.ResetTimer()
	for b.Loop() {
		source.WriteRGBA(destination, bounds.Min)
	}
}
