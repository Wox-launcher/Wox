package widget

import (
	"image"
	"testing"

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
