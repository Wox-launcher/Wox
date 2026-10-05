package imageencode

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"math/rand/v2"
	"testing"
)

// TestPNGRoundTrip compares against the standard codec across alpha, precision, origins and row padding.
func TestPNGRoundTrip(t *testing.T) {
	parent := image.Rect(-12, -8, 192, 107)
	crop := image.Rect(-9, -5, 183, 100)
	rgba := image.NewRGBA(parent)
	nrgba := image.NewNRGBA(parent)
	nrgba64 := image.NewNRGBA64(parent)
	opaque := image.NewRGBA(parent)
	gray := image.NewGray(parent)
	random := rand.New(rand.NewPCG(19, 71))
	for y := parent.Min.Y; y < parent.Max.Y; y++ {
		for x := parent.Min.X; x < parent.Max.X; x++ {
			a := byte(random.Uint32())
			rgba.SetRGBA(x, y, color.RGBA{R: byte(random.UintN(uint(a) + 1)), G: a / 2, B: a, A: a})
			nrgba.SetNRGBA(x, y, color.NRGBA{R: byte(random.Uint32()), G: 39, B: 180, A: a})
			nrgba64.SetNRGBA64(x, y, color.NRGBA64{R: uint16(random.Uint32()), G: 291, B: 59991, A: uint16(random.Uint32())})
			opaque.SetRGBA(x, y, color.RGBA{R: byte(random.Uint32()), G: 123, B: 219, A: 255})
			gray.SetGray(x, y, color.Gray{Y: byte(random.Uint32())})
		}
	}
	for name, source := range map[string]image.Image{
		"premultiplied": rgba.SubImage(crop), "straight": nrgba.SubImage(crop),
		"sixteen_bit": nrgba64.SubImage(crop), "opaque": opaque.SubImage(crop), "fallback": gray.SubImage(crop),
	} {
		t.Run(name, func(t *testing.T) {
			var standard, fast bytes.Buffer
			if err := png.Encode(&standard, source); err != nil {
				t.Fatal(err)
			}
			if err := PNG(&fast, source); err != nil {
				t.Fatal(err)
			}
			want, err := png.Decode(&standard)
			if err != nil {
				t.Fatal(err)
			}
			got, err := png.Decode(&fast)
			if err != nil {
				t.Fatal(err)
			}
			if got.Bounds() != want.Bounds() {
				t.Fatalf("bounds: got %v want %v", got.Bounds(), want.Bounds())
			}
			for y := 0; y < crop.Dy(); y++ {
				for x := 0; x < crop.Dx(); x++ {
					if color.NRGBA64Model.Convert(got.At(x, y)) != color.NRGBA64Model.Convert(want.At(x, y)) {
						t.Fatalf("pixel (%d,%d): got %v want %v", x, y, got.At(x, y), want.At(x, y))
					}
				}
			}
		})
	}
}

// TestPNGWriteFailures ensures partial files are reported even when failure occurs during IDAT flushing.
func TestPNGWriteFailures(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 192, 100))
	random := rand.New(rand.NewPCG(28, 49))
	for i := range source.Pix {
		source.Pix[i] = byte(random.Uint32())
	}
	for _, limit := range []int{0, 8, 50, 33000, 75000} {
		writer := &failingPNGWriter{remaining: limit}
		if err := PNG(writer, source); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("limit %d: got %v", limit, err)
		}
	}
	if err := PNG(io.Discard, image.NewRGBA(image.Rectangle{})); err == nil {
		t.Fatal("empty image accepted")
	}
}

type failingPNGWriter struct {
	remaining int
}

func (writer *failingPNGWriter) Write(data []byte) (int, error) {
	if len(data) > writer.remaining {
		written := writer.remaining
		writer.remaining = 0
		return written, io.ErrClosedPipe
	}
	writer.remaining -= len(data)
	return len(data), nil
}
