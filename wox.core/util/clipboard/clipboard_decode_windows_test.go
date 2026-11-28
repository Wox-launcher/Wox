//go:build windows

package clipboard

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"testing"

	"golang.org/x/image/bmp"
)

func TestDecodeNon32bppDIBMatchesBMP(t *testing.T) {
	bounds := image.Rect(0, 0, 5, 3)

	rgbImg := image.NewRGBA(bounds)
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			rgbImg.SetRGBA(x, y, color.RGBA{
				R: uint8(40 + x*35),
				G: uint8(20 + y*60),
				B: uint8(10 + x*15 + y*25),
				A: 255,
			})
		}
	}

	palettedImg := image.NewPaletted(bounds, color.Palette{
		color.RGBA{R: 10, G: 20, B: 30, A: 255},
		color.RGBA{R: 200, G: 100, B: 50, A: 255},
		color.RGBA{R: 255, G: 255, B: 255, A: 255},
	})
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			palettedImg.SetColorIndex(x, y, uint8((x+y)%3))
		}
	}

	for name, src := range map[string]image.Image{
		"24bpp":    rgbImg,
		"8bpp_pal": palettedImg,
	} {
		t.Run(name, func(t *testing.T) {
			var bmpBuf bytes.Buffer
			if err := bmp.Encode(&bmpBuf, src); err != nil {
				t.Fatalf("encode bmp: %v", err)
			}
			dib := bmpBuf.Bytes()[14:]
			headerSize := int(binary.LittleEndian.Uint32(dib[0:4]))
			width := int(int32(binary.LittleEndian.Uint32(dib[4:8])))
			height := int(int32(binary.LittleEndian.Uint32(dib[8:12])))
			bitCount := int(binary.LittleEndian.Uint16(dib[14:16]))
			compression := int(binary.LittleEndian.Uint32(dib[16:20]))
			sizeImage := int(binary.LittleEndian.Uint32(dib[20:24]))
			clrUsed := int(binary.LittleEndian.Uint32(dib[32:36]))

			decoded, err := decodeNon32bppDIB(dib, headerSize, width, height, bitCount, compression, sizeImage, clrUsed)
			if err != nil {
				t.Fatalf("decodeNon32bppDIB: %v", err)
			}
			if decoded.Bounds() != bounds {
				t.Fatalf("bounds = %v, want %v", decoded.Bounds(), bounds)
			}
			for y := 0; y < bounds.Dy(); y++ {
				for x := 0; x < bounds.Dx(); x++ {
					if decoded.At(x, y) != src.At(x, y) {
						t.Fatalf("pixel(%d,%d) = %v, want %v", x, y, decoded.At(x, y), src.At(x, y))
					}
				}
			}
		})
	}
}

func TestDecodeNon32bppDIBAvoidsDuplicateDIBAllocation(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 512, 512))
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 255
	}
	var bmpBuf bytes.Buffer
	if err := bmp.Encode(&bmpBuf, img); err != nil {
		t.Fatal(err)
	}
	dib := bmpBuf.Bytes()[14:]

	allocs := testing.AllocsPerRun(5, func() {
		decoded, err := decodeNon32bppDIB(dib, 40, 512, 512, 24, 0, len(dib)-40, 0)
		if err != nil {
			t.Fatal(err)
		}
		_ = decoded.Bounds()
	})
	// Expected output RGBA buffer is ~1 MiB; ensure no extra ~768 KiB DIB copy buffer is allocated inside bmp.Decode wrapper.
	// AllocsPerRun checks allocation count stays tiny (MultiReader + readers + bmp.Decode state).
	if allocs > 30 {
		t.Fatalf("unexpected allocation count %v for decodeNon32bppDIB", allocs)
	}
}

func TestDecode32bppDIBLayouts(t *testing.T) {
	// 2x2 pixels: (0,0)=red(255,0,0,255), (1,0)=green(0,255,0,200), (0,1)=blue(0,0,255,128), (1,1)=white(255,255,255,255)
	putPixel := func(buf []byte, off int, r, g, b, a byte) {
		buf[off+0] = b
		buf[off+1] = g
		buf[off+2] = r
		buf[off+3] = a
	}

	t.Run("bottom_up", func(t *testing.T) {
		dib := make([]byte, 40+2*2*4)
		// Bottom-up stores row y=1 first, then row y=0
		putPixel(dib, 40+0, 0, 0, 255, 128)
		putPixel(dib, 40+4, 255, 255, 255, 255)
		putPixel(dib, 40+8, 255, 0, 0, 255)
		putPixel(dib, 40+12, 0, 255, 0, 200)

		img, err := decode32bppDIB(dib, 40, 2, 2, 0)
		if err != nil {
			t.Fatal(err)
		}
		nrgba := img.(*image.NRGBA)
		if got := nrgba.NRGBAAt(0, 0); got != (color.NRGBA{R: 255, A: 255}) {
			t.Fatalf("(0,0)=%v", got)
		}
		if got := nrgba.NRGBAAt(1, 0); got != (color.NRGBA{G: 255, A: 200}) {
			t.Fatalf("(1,0)=%v", got)
		}
		if got := nrgba.NRGBAAt(0, 1); got != (color.NRGBA{B: 255, A: 128}) {
			t.Fatalf("(0,1)=%v", got)
		}
		if got := nrgba.NRGBAAt(1, 1); got != (color.NRGBA{R: 255, G: 255, B: 255, A: 255}) {
			t.Fatalf("(1,1)=%v", got)
		}
	})

	t.Run("top_down_bitfields", func(t *testing.T) {
		// BI_BITFIELDS (compression=3) with BITMAPINFOHEADER (40) adds 12 bytes of masks before pixel data
		dib := make([]byte, 40+12+2*2*4)
		putPixel(dib, 52+0, 255, 0, 0, 255)
		putPixel(dib, 52+4, 0, 255, 0, 200)
		putPixel(dib, 52+8, 0, 0, 255, 128)
		putPixel(dib, 52+12, 255, 255, 255, 255)

		img, err := decode32bppDIB(dib, 40, 2, -2, 3)
		if err != nil {
			t.Fatal(err)
		}
		nrgba := img.(*image.NRGBA)
		if got := nrgba.NRGBAAt(0, 0); got != (color.NRGBA{R: 255, A: 255}) {
			t.Fatalf("(0,0)=%v", got)
		}
		if got := nrgba.NRGBAAt(0, 1); got != (color.NRGBA{B: 255, A: 128}) {
			t.Fatalf("(0,1)=%v", got)
		}
	})
}
