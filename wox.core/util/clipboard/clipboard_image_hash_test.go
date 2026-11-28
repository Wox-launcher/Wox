package clipboard

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"image"
	"image/color"
	"image/draw"
	"testing"
)

func referenceImageHash(img image.Image) string {
	if img == nil || img.Bounds().Empty() {
		return ""
	}
	bounds := img.Bounds()
	normalized := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(normalized, normalized.Bounds(), img, bounds.Min, draw.Src)
	hasher := sha256.New()
	var dimensions [8]byte
	binary.LittleEndian.PutUint32(dimensions[0:4], uint32(bounds.Dx()))
	binary.LittleEndian.PutUint32(dimensions[4:8], uint32(bounds.Dy()))
	_, _ = hasher.Write(dimensions[:])
	_, _ = hasher.Write(normalized.Pix)
	return hex.EncodeToString(hasher.Sum(nil))
}

func TestImageHashMatchesReferenceAcrossLayouts(t *testing.T) {
	if got := ImageHash(nil); got != "" {
		t.Fatalf("ImageHash(nil) = %q, want empty", got)
	}
	if got := ImageHash(image.NewRGBA(image.Rectangle{})); got != "" {
		t.Fatalf("ImageHash(empty) = %q, want empty", got)
	}

	rgbaParent := image.NewRGBA(image.Rect(-10, -8, 20, 14))
	for y := rgbaParent.Rect.Min.Y; y < rgbaParent.Rect.Max.Y; y++ {
		for x := rgbaParent.Rect.Min.X; x < rgbaParent.Rect.Max.X; x++ {
			alpha := uint8(((x + 10) * 13) % 256)
			if x == 0 {
				alpha = 0
			} else if x == 1 {
				alpha = 255
			}
			rgbaParent.SetRGBA(x, y, color.RGBA{
				R: uint8((int(alpha) * ((x + 17) & 0xff)) / 255),
				G: uint8((int(alpha) * ((y + 31) & 0xff)) / 255),
				B: uint8(alpha / 2),
				A: alpha,
			})
		}
	}
	rgbaSub := rgbaParent.SubImage(image.Rect(-5, -3, 12, 9)).(*image.RGBA)

	nrgbaParent := image.NewNRGBA(image.Rect(-10, -8, 20, 14))
	draw.Draw(nrgbaParent, nrgbaParent.Bounds(), rgbaParent, rgbaParent.Bounds().Min, draw.Src)
	nrgbaSub := nrgbaParent.SubImage(image.Rect(-5, -3, 12, 9)).(*image.NRGBA)

	ycbcr := image.NewYCbCr(image.Rect(-4, -2, 10, 8), image.YCbCrSubsampleRatio420)
	for i := range ycbcr.Y {
		ycbcr.Y[i] = uint8(i * 19)
	}
	for i := range ycbcr.Cb {
		ycbcr.Cb[i] = uint8(100 + i*7)
		ycbcr.Cr[i] = uint8(140 - i*5)
	}

	gray := image.NewGray(image.Rect(0, 0, 9, 7))
	for i := range gray.Pix {
		gray.Pix[i] = uint8(i * 11)
	}

	for name, img := range map[string]image.Image{
		"rgba_sub":  rgbaSub,
		"nrgba_sub": nrgbaSub,
		"ycbcr":     ycbcr,
		"gray":      gray,
	} {
		t.Run(name, func(t *testing.T) {
			want := referenceImageHash(img)
			got := ImageHash(img)
			if got != want {
				t.Fatalf("ImageHash mismatch: got %s, want %s", got, want)
			}
		})
	}
	if ImageHash(rgbaSub) != ImageHash(nrgbaSub) {
		t.Fatal("equivalent RGBA and NRGBA sub-images produced different hashes")
	}
}
