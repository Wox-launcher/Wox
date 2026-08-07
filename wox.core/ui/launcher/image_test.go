package launcher

import (
	"fmt"
	"sync"
	"testing"

	woxui "wox/ui/runtime"
)

func TestImageCacheConcurrentStoresAreSerialized(t *testing.T) {
	app := &App{
		images:         map[string]*woxui.Image{},
		imageRequested: map[string]string{},
		imageLastUsed:  map[string]uint64{},
		imageErrors:    map[string]string{},
	}
	image := &woxui.Image{Width: 1, Height: 1}

	var waitGroup sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		worker := worker
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for index := 0; index < 128; index++ {
				app.storeImage(fmt.Sprintf("image-%d-%d", worker, index), image)
			}
		}()
	}
	waitGroup.Wait()

	if len(app.images) > launcherImageCacheLimit {
		t.Fatalf("image cache size = %d, want at most %d", len(app.images), launcherImageCacheLimit)
	}
}

func TestEmbeddedAppIconUsesHighResolutionPNG(t *testing.T) {
	image, err := decodeWoxImageWithTint(appIconImageSource, nil, 256)
	if err != nil {
		t.Fatalf("decode embedded app icon: %v", err)
	}
	if image.Width < 200 || image.Height < 200 {
		t.Fatalf("embedded app icon size = %dx%d, want at least 200x200", image.Width, image.Height)
	}
}

func TestDecodeWoxImagePreservesRectangularSVGDimensions(t *testing.T) {
	image, err := decodeWoxImageWithTintDimensions(woxImage{
		ImageType: "svg",
		ImageData: `<svg xmlns="http://www.w3.org/2000/svg" width="96" height="18" viewBox="0 0 96 18"><rect width="96" height="18" fill="#ffffff"/></svg>`,
	}, nil, 192, 36)
	if err != nil {
		t.Fatalf("decode rectangular SVG: %v", err)
	}
	if image.Width != 192 || image.Height != 36 {
		t.Fatalf("decoded image size = %dx%d, want 192x36", image.Width, image.Height)
	}
}

func TestCenteredSVGTextExtractsBadgeLabel(t *testing.T) {
	text, ok := centeredSVGText(woxImage{
		ImageType: "svg",
		ImageData: `<svg xmlns="http://www.w3.org/2000/svg" width="96" height="18" viewBox="0 0 96 18"><rect width="96" height="18" fill="#ffffff"/><text x="48" y="12.4" text-anchor="middle" font-size="9.5" fill="#1f2937">周 --</text></svg>`,
	}, 96, 18)
	if !ok {
		t.Fatal("expected centered SVG badge text")
	}
	if text.Value != "周 --" || text.Size != 9.5 || text.Color != (woxui.Color{R: 31, G: 41, B: 55, A: 255}) {
		t.Fatalf("centered SVG text = %+v", text)
	}
}

func TestPhysicalImageSizeUsesBackingScale(t *testing.T) {
	tests := []struct {
		name    string
		logical int
		scale   float32
		want    int
	}{
		{name: "one x", logical: 15, scale: 1, want: 15},
		{name: "retina", logical: 15, scale: 2, want: 30},
		{name: "fractional scale", logical: 15, scale: 1.5, want: 23},
		{name: "missing scale", logical: 15, scale: 0, want: 15},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := physicalImageSize(test.logical, test.scale); got != test.want {
				t.Fatalf("physicalImageSize(%d, %v) = %d, want %d", test.logical, test.scale, got, test.want)
			}
		})
	}
}
