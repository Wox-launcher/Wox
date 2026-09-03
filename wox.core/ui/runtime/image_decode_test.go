package woxui

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"testing"
	"time"
)

func TestDecodeImageMaxDownscalesLargeRasters(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 80, 40))
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatalf("encode: %v", err)
	}
	full, err := DecodeImage(bytes.NewReader(encoded.Bytes()))
	if err != nil || full.Width != 80 || full.Height != 40 {
		t.Fatalf("full decode = %#v %v", full, err)
	}
	limited, err := DecodeImageMax(bytes.NewReader(encoded.Bytes()), 20)
	if err != nil {
		t.Fatalf("limited decode: %v", err)
	}
	if limited.Width != 20 || limited.Height != 10 {
		t.Fatalf("limited size = %dx%d, want 20x10", limited.Width, limited.Height)
	}
}

func TestDecodeImageKeepsSingleFrameGIFStatic(t *testing.T) {
	encoded := encodeTestGIF(t, []*image.Paletted{solidPaletted(8, 8, color.RGBA{R: 255, A: 255})}, []int{10})
	decoded, err := DecodeImage(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.IsAnimated() || decoded.FrameCount() != 1 {
		t.Fatalf("single-frame gif animated = %t count = %d", decoded.IsAnimated(), decoded.FrameCount())
	}
}

func TestDecodeImageCompositesAnimatedGIFFrames(t *testing.T) {
	red := solidPaletted(8, 8, color.RGBA{R: 255, A: 255})
	blue := image.NewPaletted(image.Rect(0, 0, 4, 4), color.Palette{color.RGBA{}, color.RGBA{B: 255, A: 255}})
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			blue.Set(x, y, color.RGBA{B: 255, A: 255})
		}
	}
	encoded := encodeTestGIF(t, []*image.Paletted{red, blue}, []int{10, 10})
	decoded, err := DecodeImage(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !decoded.IsAnimated() || decoded.FrameCount() != 2 {
		t.Fatalf("animated gif count = %d, want 2", decoded.FrameCount())
	}
	if pixel := decoded.Frame(0).RGBAAt(0, 0); pixel.R != 255 || pixel.B != 0 {
		t.Fatalf("frame 0 origin = %+v, want red", pixel)
	}
	if pixel := decoded.Frame(1).RGBAAt(0, 0); pixel.B != 255 || pixel.R != 0 {
		t.Fatalf("frame 1 origin = %+v, want blue", pixel)
	}
	if pixel := decoded.Frame(1).RGBAAt(7, 7); pixel.R != 255 || pixel.B != 0 {
		t.Fatalf("frame 1 corner = %+v, want preserved red", pixel)
	}
	if decoded.PixelBytes() <= decoded.Width*decoded.Height*4 {
		t.Fatalf("animated pixel bytes = %d, want both frames", decoded.PixelBytes())
	}
}

func TestDecodeImageMaxDownscalesAnimatedGIF(t *testing.T) {
	encoded := encodeTestGIF(t, []*image.Paletted{
		solidPaletted(80, 40, color.RGBA{R: 255, A: 255}),
		solidPaletted(80, 40, color.RGBA{B: 255, A: 255}),
	}, []int{10, 10})
	decoded, err := DecodeImageMax(bytes.NewReader(encoded), 20)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Width != 20 || decoded.Height != 10 {
		t.Fatalf("limited size = %dx%d, want 20x10", decoded.Width, decoded.Height)
	}
	if decoded.Frame(1).Width != 20 || decoded.Frame(1).Height != 10 {
		t.Fatalf("limited frame 1 size = %dx%d, want 20x10", decoded.Frame(1).Width, decoded.Frame(1).Height)
	}
}

func TestDecodeImageSamplesLongGIFWithoutShorteningPlayback(t *testing.T) {
	frames := make([]*image.Paletted, gifMaxRetainedFrames+9)
	delays := make([]int, len(frames))
	for index := range frames {
		frames[index] = solidPaletted(8, 8, color.RGBA{R: uint8(index + 1), A: 255})
		delays[index] = index%3 + 2
	}
	decoded, err := DecodeImage(bytes.NewReader(encodeTestGIF(t, frames, delays)))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.FrameCount() != gifMaxRetainedFrames {
		t.Fatalf("retained frames = %d, want %d", decoded.FrameCount(), gifMaxRetainedFrames)
	}
	var got time.Duration
	for _, delay := range decoded.FrameDelays() {
		got += delay
	}
	var want time.Duration
	for _, delay := range delays {
		want += gifFrameDelay(delay)
	}
	if got != want {
		t.Fatalf("sampled duration = %v, want %v", got, want)
	}
}

func TestGIFRetainedFrameCountBudgetsLargeCanvases(t *testing.T) {
	cases := []struct {
		name                  string
		available             int
		width, height, expect int
	}{
		{name: "icon keeps every sampled frame", available: 64, width: 40, height: 40, expect: gifMaxRetainedFrames},
		{name: "short animation keeps its own frames", available: 5, width: 40, height: 40, expect: 5},
		{name: "preview canvas trades frames for bytes", available: 64, width: 1024, height: 1024, expect: 2},
		{name: "single oversized frame falls back to one", available: 64, width: 4096, height: 4096, expect: 1},
		{name: "degenerate size keeps the frame cap", available: 64, width: 0, height: 0, expect: gifMaxRetainedFrames},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := gifRetainedFrameCount(testCase.available, testCase.width, testCase.height)
			if got != testCase.expect {
				t.Fatalf("retained frames = %d, want %d", got, testCase.expect)
			}
		})
	}
}

func TestDecodeImageBudgetsLargeGIFWithoutShorteningPlayback(t *testing.T) {
	frames := make([]*image.Paletted, 4)
	delays := make([]int, len(frames))
	for index := range frames {
		frames[index] = solidPaletted(1024, 1024, color.RGBA{R: uint8(index + 1), A: 255})
		delays[index] = index%3 + 2
	}
	decoded, err := DecodeImage(bytes.NewReader(encodeTestGIF(t, frames, delays)))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	// The frame cap would have kept all four frames for 16 MB; the byte budget keeps two.
	if decoded.FrameCount() != 2 {
		t.Fatalf("retained frames = %d, want 2", decoded.FrameCount())
	}
	if decoded.PixelBytes() > gifMaxRetainedFrameBytes {
		t.Fatalf("retained bytes = %d, want at most %d", decoded.PixelBytes(), gifMaxRetainedFrameBytes)
	}
	var got time.Duration
	for _, delay := range decoded.FrameDelays() {
		got += delay
	}
	var want time.Duration
	for _, delay := range delays {
		want += gifFrameDelay(delay)
	}
	if got != want {
		t.Fatalf("budgeted duration = %v, want %v", got, want)
	}
}

func TestDecodeImageKeepsGIFStaticWhenOneFrameExceedsTheBudget(t *testing.T) {
	encoded := encodeTestGIF(t, []*image.Paletted{
		solidPaletted(1600, 1600, color.RGBA{R: 255, A: 255}),
		solidPaletted(1600, 1600, color.RGBA{B: 255, A: 255}),
	}, []int{10, 10})
	decoded, err := DecodeImage(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.IsAnimated() || decoded.FrameCount() != 1 {
		t.Fatalf("oversized gif animated = %t count = %d, want a static frame", decoded.IsAnimated(), decoded.FrameCount())
	}
	if decoded.PixelBytes() != 1600*1600*4 {
		t.Fatalf("retained bytes = %d, want one frame", decoded.PixelBytes())
	}
}

func TestDecodeImageMaxBudgetsFramesAtTheStoredSize(t *testing.T) {
	frames := make([]*image.Paletted, 8)
	delays := make([]int, len(frames))
	for index := range frames {
		frames[index] = solidPaletted(1024, 1024, color.RGBA{R: uint8(index + 1), A: 255})
		delays[index] = 5
	}
	encoded := encodeTestGIF(t, frames, delays)
	// Downscaling shrinks the retained frame, so the same source keeps more frames than it would
	// at full size. This is what makes the budget depend on the requested size, not the file.
	decoded, err := DecodeImageMax(bytes.NewReader(encoded), 256)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Width != 256 || decoded.Height != 256 {
		t.Fatalf("stored size = %dx%d, want 256x256", decoded.Width, decoded.Height)
	}
	if decoded.FrameCount() != len(frames) {
		t.Fatalf("retained frames = %d, want %d", decoded.FrameCount(), len(frames))
	}
}

func encodeTestGIF(t *testing.T, frames []*image.Paletted, delays []int) []byte {
	t.Helper()
	if len(frames) == 0 {
		t.Fatal("gif needs at least one frame")
	}
	var encoded bytes.Buffer
	document := &gif.GIF{
		Image:     frames,
		Delay:     delays,
		LoopCount: 0,
		Config: image.Config{
			ColorModel: frames[0].Palette,
			Width:      frames[0].Bounds().Dx(),
			Height:     frames[0].Bounds().Dy(),
		},
	}
	if err := gif.EncodeAll(&encoded, document); err != nil {
		t.Fatalf("encode gif: %v", err)
	}
	return encoded.Bytes()
}

func solidPaletted(width, height int, fill color.RGBA) *image.Paletted {
	img := image.NewPaletted(image.Rect(0, 0, width, height), color.Palette{color.RGBA{}, fill})
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, fill)
		}
	}
	return img
}
