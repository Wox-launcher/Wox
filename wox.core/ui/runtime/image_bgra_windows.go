//go:build windows

package woxui

import (
	"fmt"
	"image"
	"image/color"
)

// PackedBGRA exposes an immutable Windows desktop buffer without swapping every pixel before preview.
type PackedBGRA struct {
	Pix    []byte
	Stride int
	Rect   image.Rectangle
}

func (source *PackedBGRA) ColorModel() color.Model {
	return color.RGBAModel
}

func (source *PackedBGRA) Bounds() image.Rectangle {
	if source == nil {
		return image.Rectangle{}
	}
	return source.Rect
}

func (source *PackedBGRA) At(x int, y int) color.Color {
	return source.RGBAAt(x, y)
}

func (source *PackedBGRA) RGBAAt(x int, y int) color.RGBA {
	if source == nil || !image.Pt(x, y).In(source.Rect) {
		return color.RGBA{}
	}
	offset := (y-source.Rect.Min.Y)*source.Stride + (x-source.Rect.Min.X)*4
	return color.RGBA{R: source.Pix[offset+2], G: source.Pix[offset+1], B: source.Pix[offset], A: 255}
}

// SubImage retains the shared capture buffer while preserving desktop-relative image coordinates.
func (source *PackedBGRA) SubImage(bounds image.Rectangle) image.Image {
	bounds = bounds.Intersect(source.Rect)
	if bounds.Empty() {
		return &PackedBGRA{}
	}
	offset := (bounds.Min.Y-source.Rect.Min.Y)*source.Stride + (bounds.Min.X-source.Rect.Min.X)*4
	return &PackedBGRA{Pix: source.Pix[offset:], Stride: source.Stride, Rect: bounds}
}

// WriteRGBA copies origin..origin+dst.Size from the BGRX capture into dst as packed RGBA.
// Screenshot export uses this instead of image/draw so a selected crop does not walk every
// virtual-desktop pixel through At().
func (source *PackedBGRA) WriteRGBA(dst *image.RGBA, origin image.Point) {
	if source == nil || dst == nil {
		return
	}
	width, height := dst.Rect.Dx(), dst.Rect.Dy()
	if width <= 0 || height <= 0 {
		return
	}
	for y := 0; y < height; y++ {
		srcY := origin.Y + y
		if srcY < source.Rect.Min.Y || srcY >= source.Rect.Max.Y {
			continue
		}
		srcX := origin.X
		count := width
		if srcX < source.Rect.Min.X {
			skip := source.Rect.Min.X - srcX
			srcX += skip
			count -= skip
		}
		if srcX+count > source.Rect.Max.X {
			count = source.Rect.Max.X - srcX
		}
		if count <= 0 {
			continue
		}
		srcOff := (srcY-source.Rect.Min.Y)*source.Stride + (srcX-source.Rect.Min.X)*4
		dstOff := y*dst.Stride + (srcX-origin.X)*4
		for x := 0; x < count; x++ {
			si := srcOff + x*4
			di := dstOff + x*4
			if si+3 >= len(source.Pix) || di+3 >= len(dst.Pix) {
				break
			}
			dst.Pix[di+0] = source.Pix[si+2]
			dst.Pix[di+1] = source.Pix[si+1]
			dst.Pix[di+2] = source.Pix[si+0]
			dst.Pix[di+3] = 255
		}
	}
}

// RetainedRendererImage lets the screenshot editor upload Windows' native BGRA pixels directly.
func (source *PackedBGRA) RetainedRendererImage() (*Image, error) {
	if source == nil {
		return nil, fmt.Errorf("image source is nil")
	}
	width, height := source.Rect.Dx(), source.Rect.Dy()
	if width <= 0 || height <= 0 || width > 16384 || height > 16384 || source.Stride != width*4 {
		return nil, fmt.Errorf("image dimensions or stride are invalid: %dx%d stride=%d", width, height, source.Stride)
	}
	pixelCount := width * height * 4
	if len(source.Pix) < pixelCount {
		return nil, fmt.Errorf("image pixel buffer is too small")
	}
	return &Image{Width: width, Height: height, id: nextImageID.Add(1), pixels: source.Pix[:pixelCount], format: imagePixelFormatBGRAOpaque}, nil
}
