package widget

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"testing"
	"time"

	woxui "wox/ui/runtime"
)

func TestFittedImageBoundsPreservesAspectRatio(t *testing.T) {
	source, err := woxui.NewImage(image.NewRGBA(image.Rect(0, 0, 200, 100)))
	if err != nil {
		t.Fatal(err)
	}
	bounds := woxui.Rect{X: 10, Y: 20, Width: 100, Height: 100}

	if got := fittedImageBounds(source, bounds, ImageFitContain); got != (woxui.Rect{X: 10, Y: 45, Width: 100, Height: 50}) {
		t.Fatalf("contain bounds = %+v", got)
	}
	if got := fittedImageBounds(source, bounds, ImageFitCover); got != (woxui.Rect{X: -40, Y: 20, Width: 200, Height: 100}) {
		t.Fatalf("cover bounds = %+v", got)
	}
}

func TestImageCoverRadiusFollowsWidgetBox(t *testing.T) {
	pixels := image.NewRGBA(image.Rect(0, 0, 200, 100))
	for index := range pixels.Pix {
		pixels.Pix[index] = 255
	}
	source, err := woxui.NewImage(pixels)
	if err != nil {
		t.Fatal(err)
	}
	root := (Image{Source: source, Width: 100, Height: 100, Fit: ImageFitCover, Radius: 20}).layout(
		context{}, constraints{width: 100, height: 100},
	)

	actual := &woxui.DisplayList{}
	root.draw(actual, 0, 0, false, false, false, nil)
	expected := &woxui.DisplayList{}
	expected.DrawRotatedRoundedImage(source, woxui.Rect{Width: 100, Height: 100}, 0, 20)
	if err := actual.Compare(expected); err != nil {
		t.Fatalf("covered rounded image dest: %v", err)
	}

	scene := &woxui.DisplayList{}
	scene.Clear(woxui.Color{G: 255, A: 255})
	root.draw(scene, 0, 0, false, false, false, nil)
	renderer, err := woxui.NewSoftwareRenderer(100, 100)
	if err != nil {
		t.Fatal(err)
	}
	if err := renderer.Render(scene); err != nil {
		t.Fatal(err)
	}
	corner := renderer.RGBA().RGBAAt(0, 0)
	if corner.R != 0 || corner.G != 255 || corner.B != 0 || corner.A != 255 {
		t.Fatalf("widget corner = %+v, want the uncovered background instead of wallpaper", corner)
	}
}

func TestImagePaintsCurrentGIFFrame(t *testing.T) {
	animated := decodeTestAnimatedGIF(t)
	host := &animationHost{}
	start := time.Now()
	first := (Image{Source: animated, Width: 8, Height: 8}).layout(
		context{animation: animationFrame{host: host, generation: 1, now: start}},
		constraints{width: 8, height: 8},
	)
	second := (Image{Source: animated, Width: 8, Height: 8}).layout(
		context{animation: animationFrame{host: host, generation: 2, now: start.Add(150 * time.Millisecond)}},
		constraints{width: 8, height: 8},
	)

	firstList := &woxui.DisplayList{}
	secondList := &woxui.DisplayList{}
	first.draw(firstList, 0, 0, false, false, false, nil)
	second.draw(secondList, 0, 0, false, false, false, nil)
	if err := firstList.Compare(secondList); err == nil {
		t.Fatal("expected the painted GIF frame to change after its delay")
	}
}

func decodeTestAnimatedGIF(t *testing.T) *woxui.Image {
	t.Helper()
	red := image.NewPaletted(image.Rect(0, 0, 8, 8), color.Palette{color.RGBA{}, color.RGBA{R: 255, A: 255}})
	blue := image.NewPaletted(image.Rect(0, 0, 8, 8), color.Palette{color.RGBA{}, color.RGBA{B: 255, A: 255}})
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			red.Set(x, y, color.RGBA{R: 255, A: 255})
			blue.Set(x, y, color.RGBA{B: 255, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := gif.EncodeAll(&encoded, &gif.GIF{
		Image:     []*image.Paletted{red, blue},
		Delay:     []int{10, 10},
		LoopCount: 0,
		Config:    image.Config{ColorModel: red.Palette, Width: 8, Height: 8},
	}); err != nil {
		t.Fatalf("encode gif: %v", err)
	}
	decoded, err := woxui.DecodeImage(bytes.NewReader(encoded.Bytes()))
	if err != nil || !decoded.IsAnimated() {
		t.Fatalf("decode gif: animated=%t err=%v", decoded != nil && decoded.IsAnimated(), err)
	}
	return decoded
}
