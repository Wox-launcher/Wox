package woxui

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"sync/atomic"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

// Image stores immutable packed pixels ready for native GPU upload.
type Image struct {
	Width  int
	Height int
	id     uint64
	pixels []byte
	format imagePixelFormat
}

type imagePixelFormat uint8

const (
	imagePixelFormatRGBA imagePixelFormat = iota
	imagePixelFormatBGRAOpaque
)

var nextImageID atomic.Uint64

// DecodeImage decodes a supported raster image into the renderer's shared pixel format.
func DecodeImage(reader io.Reader) (*Image, error) {
	source, _, err := image.Decode(reader)
	if err != nil {
		return nil, err
	}
	return NewImage(source)
}

// NewImage copies a Go image into tightly packed, top-down premultiplied RGBA pixels.
func NewImage(source image.Image) (*Image, error) {
	if source == nil {
		return nil, fmt.Errorf("image source is nil")
	}
	bounds := source.Bounds()
	if bounds.Empty() || bounds.Dx() > 16384 || bounds.Dy() > 16384 {
		return nil, fmt.Errorf("image dimensions are invalid: %dx%d", bounds.Dx(), bounds.Dy())
	}
	rgba := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(rgba, rgba.Bounds(), source, bounds.Min, draw.Src)
	return &Image{Width: rgba.Rect.Dx(), Height: rgba.Rect.Dy(), id: nextImageID.Add(1), pixels: rgba.Pix, format: imagePixelFormatRGBA}, nil
}

// NewImageFromPackedRGBA retains an immutable, tightly packed RGBA buffer without copying it.
func NewImageFromPackedRGBA(source *image.RGBA) (*Image, error) {
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
	return &Image{Width: width, Height: height, id: nextImageID.Add(1), pixels: source.Pix[:pixelCount], format: imagePixelFormatRGBA}, nil
}

// ID is the stable native cache key for this immutable pixel buffer.
func (i *Image) ID() uint64 {
	if i == nil {
		return 0
	}
	return i.id
}

// RGBAAt returns one pixel from the image's zero-based renderer coordinate space.
func (i *Image) RGBAAt(x, y int) color.RGBA {
	if i == nil || x < 0 || y < 0 || x >= i.Width || y >= i.Height {
		return color.RGBA{}
	}
	offset := (y*i.Width + x) * 4
	if i.format == imagePixelFormatBGRAOpaque {
		return color.RGBA{R: i.pixels[offset+2], G: i.pixels[offset+1], B: i.pixels[offset], A: 255}
	}
	return color.RGBA{R: i.pixels[offset], G: i.pixels[offset+1], B: i.pixels[offset+2], A: i.pixels[offset+3]}
}
