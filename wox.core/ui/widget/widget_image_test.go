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
