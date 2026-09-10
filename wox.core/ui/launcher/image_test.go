package launcher

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"strings"
	"sync"
	"testing"

	"wox/common/icons"
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
	if image.Width < 200 || image.Height < 200 || image.Width > 256 || image.Height > 256 {
		t.Fatalf("embedded app icon size = %dx%d, want both dimensions between 200 and 256", image.Width, image.Height)
	}
}

func TestDecodeWoxImagePreservesRectangularSVGDimensions(t *testing.T) {
	image, err := decodeWoxImageWithTintDimensions(woxImage{
		ImageType: "svg",
		ImageData: `<svg xmlns="http://www.w3.org/2000/svg" width="96" height="18" viewBox="0 0 96 18"><rect width="96" height="18" fill="#ffffff"/></svg>`,
	}, nil, 192, 36, false)
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

func TestPreviewImageRequestSizeUsesPreviewSurfaceDimensions(t *testing.T) {
	tests := []struct {
		name   string
		width  float32
		height float32
		want   int
	}{
		{name: "wide preview", width: 400, height: 180, want: 800},
		{name: "tall preview", width: 320, height: 700, want: 1400},
		{name: "minimum resolution", width: 120, height: 80, want: 512},
		{name: "maximum resolution", width: 1400, height: 1200, want: 2048},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := previewImageRequestSize(test.width, test.height); got != test.want {
				t.Fatalf("previewImageRequestSize(%v, %v) = %d, want %d", test.width, test.height, got, test.want)
			}
		})
	}
}

func TestImageForSizeKeepsPreviousResolutionWhileLoadingNewOne(t *testing.T) {
	source := woxImage{ImageType: "svg", ImageData: `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"><rect width="16" height="16" fill="#ffffff"/></svg>`}
	variantKey := imageVariantKey(source, nil)
	oldKey := variantKey + "-svg-32"
	newKey := variantKey + "-svg-48"
	oldImage := &woxui.Image{Width: 32, Height: 32}
	app := &App{
		palette:          uiPalette{background: woxui.Color{R: 255, G: 255, B: 255, A: 255}},
		images:           map[string]*woxui.Image{oldKey: oldImage},
		imageRequested:   map[string]string{newKey: source.ImageData},
		imageVariants:    map[string]string{variantKey: oldKey},
		imageVariantKeys: map[string]string{},
		imageLastUsed:    map[string]uint64{},
		imageErrors:      map[string]string{},
	}

	if got := app.imageForSize(source, 48); got != oldImage {
		t.Fatalf("imageForSize returned %p, want cached image %p while new resolution loads", got, oldImage)
	}
}

func TestImageCacheEvictsByItemAndByteBudget(t *testing.T) {
	app := &App{images: map[string]*woxui.Image{}, imageLastUsed: map[string]uint64{}}
	for index := 0; index < 8; index++ {
		app.imageUseSequence++
		app.imageLastUsed[fmt.Sprintf("small-%d", index)] = app.imageUseSequence
		app.insertImageLocked(fmt.Sprintf("small-%d", index), &woxui.Image{Width: 1, Height: 1})
	}
	if len(app.images) != 8 {
		t.Fatalf("small cache size = %d, want 8", len(app.images))
	}

	app.imageUseSequence++
	app.imageLastUsed["hot"] = app.imageUseSequence
	app.insertImageLocked("hot", &woxui.Image{Width: 1, Height: 1})
	wide := &woxui.Image{Width: 2048, Height: 2048}
	app.imageUseSequence++
	app.imageLastUsed["wide"] = app.imageUseSequence
	app.insertImageLocked("wide", wide)
	if _, ok := app.images["wide"]; !ok {
		t.Fatal("expected the large in-use image to stay cached")
	}
	if app.imageCacheByteSizeLocked() > launcherImageCacheMaxBytes && len(app.images) != 1 {
		t.Fatalf("over-budget cache = %d items / %d bytes, want eviction down to the in-use image", len(app.images), app.imageCacheByteSizeLocked())
	}
}

func TestImageCacheOversizeImageMonopolizesCache(t *testing.T) {
	app := &App{images: map[string]*woxui.Image{"icon": {Width: 32, Height: 32}}, imageLastUsed: map[string]uint64{"icon": 1}}
	oversize := &woxui.Image{Width: 4096, Height: 4096}
	app.imageLastUsed["preview"] = 2
	app.insertImageLocked("preview", oversize)
	if len(app.images) != 1 || app.images["preview"] != oversize {
		t.Fatalf("oversize cache = %d items, want only the in-use preview", len(app.images))
	}
}

func TestImageCacheReplaceAppliesByteBudget(t *testing.T) {
	app := &App{images: map[string]*woxui.Image{}, imageLastUsed: map[string]uint64{}}
	for index := 0; index < 8; index++ {
		key := fmt.Sprintf("small-%d", index)
		app.imageUseSequence++
		app.imageLastUsed[key] = app.imageUseSequence
		app.insertImageLocked(key, &woxui.Image{Width: 1, Height: 1})
	}
	app.imageUseSequence++
	app.imageLastUsed["photo"] = app.imageUseSequence
	app.insertImageLocked("photo", &woxui.Image{Width: 1, Height: 1})
	oversize := &woxui.Image{Width: 4096, Height: 4096}
	app.insertImageLocked("photo", oversize)
	if len(app.images) != 1 || app.images["photo"] != oversize {
		t.Fatalf("replaced cache = %d items, want only the decoded photo", len(app.images))
	}
}

func TestImageCacheHiddenTrimEvictsSingleOversizeImage(t *testing.T) {
	app := &App{images: map[string]*woxui.Image{}, imageLastUsed: map[string]uint64{"preview": 1}}
	app.insertImageLocked("preview", &woxui.Image{Width: 4096, Height: 4096})
	app.trimIdleImageCache()
	if len(app.images) != 0 || app.imageCacheByteSizeLocked() != 0 {
		t.Fatalf("hidden oversize cache = %d items / %d bytes, want empty", len(app.images), app.imageCacheByteSizeLocked())
	}
}

func TestImageCacheHiddenTrimUsesCountAndByteBudget(t *testing.T) {
	app := &App{images: map[string]*woxui.Image{}, imageLastUsed: map[string]uint64{}}
	for index := 0; index < 80; index++ {
		key := fmt.Sprintf("icon-%d", index)
		app.imageLastUsed[key] = uint64(index + 1)
		app.insertImageLocked(key, &woxui.Image{Width: 64, Height: 64})
	}
	app.trimIdleImageCache()
	if len(app.images) > hiddenImageCacheKeepCount {
		t.Fatalf("hidden cache count = %d, want at most %d", len(app.images), hiddenImageCacheKeepCount)
	}
	if app.imageCacheByteSizeLocked() > hiddenImageCacheMaxBytes {
		t.Fatalf("hidden cache bytes = %d, want at most %d", app.imageCacheByteSizeLocked(), hiddenImageCacheMaxBytes)
	}
	if got := app.imageCacheByteSizeLocked(); got != len(app.images)*imageCacheBytes(&woxui.Image{Width: 64, Height: 64}) {
		t.Fatalf("hidden cache byte counter = %d, want %d", got, len(app.images)*imageCacheBytes(&woxui.Image{Width: 64, Height: 64}))
	}
	if _, ok := app.images["icon-79"]; !ok {
		t.Fatal("expected the most recently used hidden image to be kept")
	}
}

func TestImageCacheCountsAnimatedFrames(t *testing.T) {
	animated := decodeLauncherTestGIF(t)
	app := &App{images: map[string]*woxui.Image{}, imageLastUsed: map[string]uint64{}}
	app.insertImageLocked("gif", animated)
	if got := app.imageCacheByteSizeLocked(); got != animated.PixelBytes() {
		t.Fatalf("animated cache bytes = %d, want %d", got, animated.PixelBytes())
	}
	if animated.PixelBytes() <= animated.Width*animated.Height*4 {
		t.Fatal("expected GIF cache bytes to include every decoded frame")
	}
}

func decodeLauncherTestGIF(t *testing.T) *woxui.Image {
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

func TestDecodeSVGImageCurrentColorDefaultsToBlack(t *testing.T) {
	source := woxImage{ImageType: "svg", ImageData: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10" fill="currentColor"/></svg>`}
	image, err := decodeWoxImageWithTintDimensions(source, nil, 10, 10, true)
	if err != nil {
		t.Fatalf("decode currentColor SVG: %v", err)
	}
	pixel := image.RGBAAt(5, 5)
	if pixel.R != 0 || pixel.G != 0 || pixel.B != 0 || pixel.A != 255 {
		t.Fatalf("untinted currentColor pixel = %+v, want black", pixel)
	}

	tint := woxui.Color{R: 244, G: 247, B: 250, A: 255}
	tinted, err := decodeWoxImageWithTint(source, &tint, 10)
	if err != nil {
		t.Fatalf("decode tinted currentColor SVG: %v", err)
	}
	pixel = tinted.RGBAAt(5, 5)
	if pixel.R != tint.R || pixel.G != tint.G || pixel.B != tint.B || pixel.A != tint.A {
		t.Fatalf("tinted currentColor pixel = %+v, want %+v", pixel, tint)
	}
}

func TestIsLoadingIconMatchesSharedLoadingSVG(t *testing.T) {
	if !isLoadingIcon(fromCoreImage(icons.Get(icons.StatusLoading))) {
		t.Fatal("shared LoadingIcon should be recognized as a loading placeholder")
	}
	if isLoadingIcon(fromCoreImage(icons.Get(icons.ActionSearch))) {
		t.Fatal("a regular result icon should not be treated as loading")
	}
}

func TestImageCacheReplaceUpdatesByteCounter(t *testing.T) {
	app := &App{images: map[string]*woxui.Image{}, imageLastUsed: map[string]uint64{}}
	app.insertImageLocked("photo", &woxui.Image{Width: 10, Height: 10})
	if got := app.imageCacheByteSizeLocked(); got != 400 {
		t.Fatalf("initial cache bytes = %d, want 400", got)
	}
	app.insertImageLocked("photo", &woxui.Image{Width: 20, Height: 20})
	if got := app.imageCacheByteSizeLocked(); got != 1600 {
		t.Fatalf("replaced cache bytes = %d, want 1600", got)
	}
}

func mostOpaqueImagePixel(image *woxui.Image) (color.RGBA, bool) {
	var best color.RGBA
	found := false
	for y := 0; y < image.Height; y++ {
		for x := 0; x < image.Width; x++ {
			pixel := image.RGBAAt(x, y)
			if !found || pixel.A > best.A {
				best = pixel
				found = true
			}
		}
	}
	return best, found && best.A >= 200
}

func TestActionCopyIconFollowsRowTextTint(t *testing.T) {
	source := fromCoreImage(icons.Get(icons.ActionCopy))
	tint := woxui.Color{R: 255, G: 255, B: 255, A: 255}
	decoded, err := decodeWoxImageWithTint(source, &tint, 48)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := mostOpaqueImagePixel(decoded)
	if !ok {
		t.Fatal("tinted action.copy has no visible stroke")
	}
	if got.R != tint.R || got.G != tint.G || got.B != tint.B || got.A < 240 {
		t.Fatalf("tinted action.copy pixel = %+v, want selected-text white", got)
	}
}

func TestActionCopyIconFollowsAppearance(t *testing.T) {
	source := fromCoreImage(icons.Get(icons.ActionCopy))
	if !strings.Contains(source.ImageData, "var(--wox-theme-icon-color)") {
		t.Fatal("action.copy must use var(--wox-theme-icon-color)")
	}
	for _, dark := range []bool{false, true} {
		decoded, err := decodeWoxImageWithTintDimensions(source, nil, 48, 48, dark)
		if err != nil {
			t.Fatal(err)
		}
		want := uint8(0)
		if dark {
			want = 255
		}
		got, ok := mostOpaqueImagePixel(decoded)
		if !ok {
			t.Fatalf("dark=%v: action.copy has no visible stroke", dark)
		}
		if got.R != want || got.G != want || got.B != want || got.A < 240 {
			t.Fatalf("dark=%v: action.copy pixel = %+v, want theme icon color", dark, got)
		}
	}
}

// TestSVGWoxThemeIconColorFollowsAppearance verifies opt-in paints without recoloring the brand fill.
func TestSVGWoxThemeIconColorFollowsAppearance(t *testing.T) {
	source := woxImage{ImageType: "svg", ImageData: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 10"><path fill="var(--wox-theme-icon-color)" d="M0 0h10v10H0z"/><path fill="#4D6BFE" d="M10 0h10v10H10z"/></svg>`}
	for _, dark := range []bool{false, true} {
		decoded, err := decodeWoxImageWithTintDimensions(source, nil, 20, 10, dark)
		if err != nil {
			t.Fatal(err)
		}
		want := uint8(0)
		if dark {
			want = 255
		}
		if got := decoded.RGBAAt(5, 5); got.R != want || got.G != want || got.B != want || got.A != 255 {
			t.Fatalf("dark=%v: var(--wox-theme-icon-color) = %+v", dark, got)
		}
		if got := decoded.RGBAAt(15, 5); got.R != 0x4d || got.G != 0x6b || got.B != 0xfe || got.A != 255 {
			t.Fatalf("dark=%v: brand color = %+v", dark, got)
		}
	}
}

func TestImageCacheSeparatesAppearance(t *testing.T) {
	source := woxImage{ImageType: "svg", ImageData: `<svg fill="var(--wox-theme-icon-color)"/>`}
	key := imageKey(source) + "-svg-18"
	light, dark := &woxui.Image{}, &woxui.Image{}
	app := &App{images: map[string]*woxui.Image{key: light, key + "-dark": dark}, imageLastUsed: map[string]uint64{}}
	for _, isDark := range []bool{false, true, false} {
		app.palette.background = woxui.Color{R: 255, G: 255, B: 255, A: 255}
		want := light
		if isDark {
			app.palette.background = woxui.Color{A: 255}
			want = dark
		}
		if got := app.imageForSize(source, 18); got != want {
			t.Fatalf("dark=%v: reused the wrong appearance", isDark)
		}
	}
}
