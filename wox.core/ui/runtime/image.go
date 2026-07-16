package woxui

import (
	"fmt"
	"image"
	"image/draw"
	"io"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

// Image stores immutable premultiplied RGBA pixels ready for native GPU upload.
type Image struct {
	Width  int
	Height int
	pixels []byte
}

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
	return &Image{Width: rgba.Rect.Dx(), Height: rgba.Rect.Dy(), pixels: rgba.Pix}, nil
}
