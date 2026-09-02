package woxui

import (
	"bufio"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"io"
	"math"
	"sync/atomic"
	"time"

	xdraw "golang.org/x/image/draw"

	_ "image/jpeg"
	_ "image/png"
)

const (
	// Browsers treat GIF delays under 20ms as 100ms to avoid flickering historical files.
	gifMinimumDelayCentiseconds = 2
	gifDefaultDelayCentiseconds = 10
	// Keep animated search-result grids within the launcher image-cache budget.
	gifMaxRetainedFrames = 32
)

// Image stores immutable packed pixels ready for native GPU upload.
type Image struct {
	Width  int
	Height int
	id     uint64
	pixels []byte
	format imagePixelFormat
	// animation is set only on the decoded head image. Individual frames stay
	// static so widget paint can swap them without re-entering GIF playback.
	animation *imageAnimation
}

// imageAnimation holds composited GIF frames that share one cache entry.
type imageAnimation struct {
	frames []*Image
	delays []time.Duration
}

type imagePixelFormat uint8

const (
	imagePixelFormatRGBA imagePixelFormat = iota
	imagePixelFormatBGRAOpaque
)

var nextImageID atomic.Uint64

// DecodeImage decodes a supported raster image into the renderer's shared pixel format.
func DecodeImage(reader io.Reader) (*Image, error) {
	return DecodeImageMax(reader, 0)
}

// DecodeImageMax decodes a raster and downscales it when either edge exceeds maxDimension.
func DecodeImageMax(reader io.Reader, maxDimension int) (*Image, error) {
	buffered := asBufferedReader(reader)
	if header, err := buffered.Peek(6); err == nil && isGIFSignature(header) {
		return decodeAnimatedGIF(buffered, maxDimension)
	}
	source, _, err := image.Decode(buffered)
	if err != nil {
		return nil, err
	}
	return NewImage(constrainDecodedImage(source, maxDimension))
}

func asBufferedReader(reader io.Reader) *bufio.Reader {
	if buffered, ok := reader.(*bufio.Reader); ok {
		return buffered
	}
	return bufio.NewReader(reader)
}

func isGIFSignature(header []byte) bool {
	return len(header) >= 6 && (string(header[:6]) == "GIF87a" || string(header[:6]) == "GIF89a")
}

// decodeAnimatedGIF composites every frame onto the GIF canvas so disposal methods stay correct.
func decodeAnimatedGIF(reader io.Reader, maxDimension int) (*Image, error) {
	decoded, err := gif.DecodeAll(reader)
	if err != nil {
		return nil, err
	}
	if len(decoded.Image) == 0 {
		return nil, fmt.Errorf("gif has no frames")
	}
	if len(decoded.Image) == 1 {
		return NewImage(constrainDecodedImage(decoded.Image[0], maxDimension))
	}

	canvasWidth, canvasHeight := decoded.Config.Width, decoded.Config.Height
	if canvasWidth <= 0 || canvasHeight <= 0 {
		bounds := decoded.Image[0].Bounds()
		canvasWidth, canvasHeight = bounds.Dx(), bounds.Dy()
	}
	canvas := image.NewRGBA(image.Rect(0, 0, canvasWidth, canvasHeight))
	frameCount := min(len(decoded.Image), gifMaxRetainedFrames)
	frames := make([]*Image, 0, frameCount)
	delays := make([]time.Duration, 0, frameCount)
	var previous *image.RGBA
	retainedIndex := 0
	for index := range decoded.Image {
		frame := decoded.Image[index]
		disposal := gifDisposalAt(decoded, index)
		if disposal == gif.DisposalPrevious {
			previous = cloneRGBA(canvas)
		}
		draw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, draw.Over)
		if retainedIndex < frameCount && index == retainedIndex*len(decoded.Image)/frameCount {
			converted, err := NewImage(constrainDecodedImage(cloneRGBA(canvas), maxDimension))
			if err != nil {
				return nil, err
			}
			frames = append(frames, converted)
			delays = append(delays, 0)
			retainedIndex++
		}
		delays[len(delays)-1] += gifFrameDelay(gifDelayAt(decoded, index))
		switch disposal {
		case gif.DisposalBackground:
			draw.Draw(canvas, frame.Bounds(), image.Transparent, image.Point{}, draw.Src)
		case gif.DisposalPrevious:
			if previous != nil {
				canvas = previous
				previous = nil
			}
		}
	}

	head := frames[0]
	// Keep the displayed first frame static so Image.layout can swap frames
	// without treating the painted source as another animated GIF.
	frames[0] = &Image{Width: head.Width, Height: head.Height, id: head.id, pixels: head.pixels, format: head.format}
	head.animation = &imageAnimation{frames: frames, delays: delays}
	return head, nil
}

func gifDisposalAt(decoded *gif.GIF, index int) byte {
	if decoded == nil || index < 0 || index >= len(decoded.Disposal) {
		return 0
	}
	return decoded.Disposal[index]
}

func gifDelayAt(decoded *gif.GIF, index int) int {
	if decoded == nil || index < 0 || index >= len(decoded.Delay) {
		return gifDefaultDelayCentiseconds
	}
	return decoded.Delay[index]
}

func gifFrameDelay(centiseconds int) time.Duration {
	if centiseconds < gifMinimumDelayCentiseconds {
		centiseconds = gifDefaultDelayCentiseconds
	}
	return time.Duration(centiseconds) * 10 * time.Millisecond
}

func cloneRGBA(source *image.RGBA) *image.RGBA {
	if source == nil {
		return nil
	}
	clone := image.NewRGBA(source.Rect)
	copy(clone.Pix, source.Pix)
	return clone
}

// constrainDecodedImage keeps decoded rasters inside the caller's cache budget.
func constrainDecodedImage(source image.Image, maxDimension int) image.Image {
	if source == nil {
		return source
	}
	width, height := source.Bounds().Dx(), source.Bounds().Dy()
	if maxDimension <= 0 || width <= maxDimension && height <= maxDimension {
		return source
	}
	scale := float64(maxDimension) / float64(max(width, height))
	nextWidth := max(1, int(math.Round(float64(width)*scale)))
	nextHeight := max(1, int(math.Round(float64(height)*scale)))
	dst := image.NewRGBA(image.Rect(0, 0, nextWidth, nextHeight))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), source, source.Bounds(), xdraw.Src, nil)
	return dst
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

// IsAnimated reports whether this image has more than one GIF playback frame.
func (i *Image) IsAnimated() bool {
	return i != nil && i.animation != nil && len(i.animation.frames) > 1
}

// FrameCount returns the number of composited playback frames.
func (i *Image) FrameCount() int {
	if i == nil {
		return 0
	}
	if !i.IsAnimated() {
		return 1
	}
	return len(i.animation.frames)
}

// Frame returns one static playback frame. Out-of-range indexes clamp to the nearest frame.
func (i *Image) Frame(index int) *Image {
	if i == nil {
		return nil
	}
	if !i.IsAnimated() {
		return i
	}
	if index < 0 {
		index = 0
	}
	if index >= len(i.animation.frames) {
		index = len(i.animation.frames) - 1
	}
	if frame := i.animation.frames[index]; frame != nil {
		return frame
	}
	return i
}

// FrameDelays returns per-frame display durations used by widget GIF playback.
func (i *Image) FrameDelays() []time.Duration {
	if !i.IsAnimated() {
		return nil
	}
	return i.animation.delays
}

// PixelBytes accounts for every decoded frame stored with this image.
func (i *Image) PixelBytes() int {
	if i == nil {
		return 0
	}
	bytes := i.Width * i.Height * 4
	if i.animation == nil {
		return bytes
	}
	for _, frame := range i.animation.frames {
		if frame == nil || frame.id == i.id {
			continue
		}
		bytes += frame.Width * frame.Height * 4
	}
	return bytes
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
