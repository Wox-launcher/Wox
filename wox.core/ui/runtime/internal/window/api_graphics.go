package window

import (
	image "image"
	io "io"
	time "time"
	graphics "wox/ui/runtime/internal/graphics"
)

// Color stores a straight-alpha sRGB color.
type Color = graphics.Color

// Size describes an area in logical pixels.
type Size = graphics.Size

// PixelSize describes a drawable surface in physical pixels.
type PixelSize = graphics.PixelSize

// Rect describes a drawing region in logical pixels, with a top-left origin.
type Rect = graphics.Rect

// Point describes a position or delta in logical pixels.
type Point = graphics.Point

// Image stores immutable packed pixels ready for native GPU upload.
type Image = graphics.Image

// DecodeImage decodes a supported raster image into the renderer's shared pixel format.
func DecodeImage(reader io.Reader) (*Image, error) { return graphics.DecodeImage(reader) }

// DecodeImageMax decodes a raster and downscales it when either edge exceeds maxDimension.
func DecodeImageMax(reader io.Reader, maxDimension int) (*Image, error) {
	return graphics.DecodeImageMax(reader, maxDimension)
}

// NewImage copies a Go image into tightly packed, top-down premultiplied RGBA pixels.
func NewImage(source image.Image) (*Image, error) { return graphics.NewImage(source) }

// NewAnimatedImage packs playback frames the same way GIF decoding does.
// One frame, or a single retained frame after the byte budget, stays static.
func NewAnimatedImage(frames []image.Image, delays []time.Duration) (*Image, error) {
	return graphics.NewAnimatedImage(frames, delays)
}

// NewImageFromPackedRGBA retains an immutable, tightly packed RGBA buffer without copying it.
func NewImageFromPackedRGBA(source *image.RGBA) (*Image, error) {
	return graphics.NewImageFromPackedRGBA(source)
}
