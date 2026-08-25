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
