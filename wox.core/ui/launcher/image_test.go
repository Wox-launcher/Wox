package launcher

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"os"
	"path/filepath"
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
	}, nil, 192, 36, false, nil)
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

func TestCenteredSVGTextExtractsAIQuotaProgressLabels(t *testing.T) {
	tests := []struct {
		name  string
		svg   string
		label string
	}{
		{
			name:  "session",
			label: "5H 100%",
			svg: `<svg xmlns="http://www.w3.org/2000/svg" width="96" height="18" viewBox="0 0 96 18">
  <rect x="0" y="0" width="96" height="18" rx="9" fill="#687084"/>
  <rect x="1" y="1" width="94" height="16" rx="8" fill="#ffffff"/>
  <path d="M 9 1 H 87 A 8 8 0 0 1 87 17 H 9 A 8 8 0 0 1 9 1 Z" fill="#9bc27d"/>
  <text x="48" y="12.4" text-anchor="middle" font-family="Arial, sans-serif" font-size="9.5" fill="#1f2937">5H 100%</text>
</svg>`,
		},
		{
			name:  "week",
			label: "Week 2%",
			svg: `<svg xmlns="http://www.w3.org/2000/svg" width="96" height="18" viewBox="0 0 96 18">
  <rect x="0" y="0" width="96" height="18" rx="9" fill="#687084"/>
  <rect x="1" y="1" width="94" height="16" rx="8" fill="#ffffff"/>
  <path d="M 3 3.71 L 3 14.29 A 8 8 0 0 1 3 3.71 Z" fill="#d95c5c"/>
  <text x="48" y="12.4" text-anchor="middle" font-family="Arial, sans-serif" font-size="9.5" fill="#1f2937">Week 2%</text>
</svg>`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			text, ok := centeredSVGText(woxImage{ImageType: "svg", ImageData: test.svg}, 96, 18)
			if !ok {
				t.Fatal("expected centered SVG badge text")
			}
			if text.Value != test.label || text.Size != 9.5 || text.Color != (woxui.Color{R: 31, G: 41, B: 55, A: 255}) {
				t.Fatalf("centered SVG text = %+v, want %q", text, test.label)
			}
		})
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

func TestImageForViewportPinsUntilRelease(t *testing.T) {
	source := woxImage{ImageType: "svg", ImageData: `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"><rect width="16" height="16" fill="#ffffff"/></svg>`}
	app := &App{
		palette:          uiPalette{background: woxui.Color{R: 255, G: 255, B: 255, A: 255}},
		images:           map[string]*woxui.Image{},
		imageRequested:   map[string]string{},
		imageVariants:    map[string]string{},
		imageVariantKeys: map[string]string{},
		imageLastUsed:    map[string]uint64{},
		imageErrors:      map[string]string{},
	}
	key, _, _ := imageAppearanceCacheKey(source, nil, 32, 32, false, nil)
	app.images[key] = &woxui.Image{Width: 32, Height: 32}
	if app.imageForViewport(source, 32) == nil {
		t.Fatal("expected the cached viewport image")
	}
	if _, ok := app.imageViewport[key]; !ok {
		t.Fatalf("viewport pin missing for %s", key)
	}
	app.releaseViewportImage(source, 32)
	if _, ok := app.imageViewport[key]; ok {
		t.Fatal("release left the viewport pin in place")
	}
}

func TestImageCacheKeepsViewportImagesWhenBudgetIsExceeded(t *testing.T) {
	app := &App{images: map[string]*woxui.Image{}, imageLastUsed: map[string]uint64{}, imageViewport: map[string]struct{}{"shown": {}}}
	app.imageLastUsed["shown"] = 1
	app.insertImageLocked("shown", &woxui.Image{Width: 3000, Height: 3000})
	app.imageViewport["also"] = struct{}{}
	app.imageLastUsed["also"] = 2
	app.insertImageLocked("also", &woxui.Image{Width: 3000, Height: 3000})
	if _, ok := app.images["shown"]; !ok {
		t.Fatal("viewport image was discarded to store another viewport image")
	}
	if _, ok := app.images["also"]; !ok {
		t.Fatal("incoming viewport image was discarded")
	}
	app.imageLastUsed["cold"] = 0
	app.insertImageLocked("cold", &woxui.Image{Width: 8, Height: 8})
	if _, ok := app.images["cold"]; !ok {
		t.Fatal("unpinned image was not stored")
	}
	if _, shown := app.images["shown"]; !shown {
		t.Fatal("storing an unpinned image discarded a viewport image")
	}
	if _, also := app.images["also"]; !also {
		t.Fatal("storing an unpinned image discarded the other viewport image")
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

func TestHiddenLauncherKeepsImagesUsedBySettings(t *testing.T) {
	app := &App{settingsOpen: true, images: map[string]*woxui.Image{}, imageLastUsed: map[string]uint64{}}
	for index := 0; index < hiddenImageCacheKeepCount+1; index++ {
		key := fmt.Sprintf("settings-icon-%d", index)
		app.imageLastUsed[key] = uint64(index + 1)
		app.insertImageLocked(key, &woxui.Image{Width: 64, Height: 64})
	}
	app.trimIdleImageCache()
	if got := len(app.images); got != hiddenImageCacheKeepCount+1 {
		t.Fatalf("settings cache has %d images, want %d", got, hiddenImageCacheKeepCount+1)
	}
	app.settingsOpen = false
	app.trimIdleImageCache()
	if got := len(app.images); got != hiddenImageCacheKeepCount {
		t.Fatalf("closed settings cache has %d images, want %d", got, hiddenImageCacheKeepCount)
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
	image, err := decodeWoxImageWithTintDimensions(source, nil, 10, 10, true, nil)
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
		decoded, err := decodeWoxImageWithTintDimensions(source, nil, 48, 48, dark, nil)
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
		decoded, err := decodeWoxImageWithTintDimensions(source, nil, 20, 10, dark, nil)
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

func TestDecodeThemeImageUsesRoundedSettingsSwatch(t *testing.T) {
	decoded, err := decodeWoxImageWithTint(woxImage{
		ImageType: "theme",
		ImageData: `{"AppBackgroundColor":"#112233","QueryBoxBackgroundColor":"#445566","ResultItemActiveBackgroundColor":"#778899"}`,
	}, nil, 128)
	if err != nil {
		t.Fatalf("decode theme image: %v", err)
	}
	if corner := decoded.RGBAAt(0, 0); corner.A != 0 {
		t.Fatalf("corner = %+v, want a transparent rounded catalog swatch", corner)
	}
	center := decoded.RGBAAt(64, 40)
	if center.A == 0 || center.R < 0x10 {
		t.Fatalf("swatch body = %+v, want the theme background or query bar", center)
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
	// Image themes can have a transparent outer frame and an opaque light content panel.
	app.palette.background = woxui.Color{}
	app.palette.AppContentBackground = woxui.Color{R: 245, G: 241, B: 233, A: 255}
	if got := app.imageForSize(source, 18); got != light {
		t.Fatal("light content inside a transparent frame reused the dark icon")
	}
}

func TestPaletteAppearanceUsesVisibleContent(t *testing.T) {
	white := woxui.Color{R: 255, G: 255, B: 255, A: 255}
	black := woxui.Color{A: 255}
	for _, tc := range []struct {
		name                      string
		background, content, text woxui.Color
		dark                      bool
	}{
		{"light content", black, white, black, false},
		{"dark content", white, black, white, true},
		{"transparent content", white, woxui.Color{}, black, false},
		{"translucent light content", black, woxui.Color{R: 255, G: 255, B: 255, A: 240}, black, false},
		{"translucent dark content", white, woxui.Color{A: 240}, white, true},
		{"transparent frame", woxui.Color{}, white, black, false},
		{"unpainted light surface", woxui.Color{}, woxui.Color{}, black, false},
		{"unpainted dark surface", woxui.Color{}, woxui.Color{}, white, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			palette := uiPalette{background: tc.background, AppContentBackground: tc.content, resultTitle: tc.text}
			if got := palette.isDark(); got != tc.dark {
				t.Fatalf("dark = %v, want %v", got, tc.dark)
			}
		})
	}
}

// TestResultIconColors verifies selection variants preserve authored brand paints.
func TestResultIconColors(t *testing.T) {
	source := woxImage{ImageType: "svg", ImageData: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 10"><path fill="var(--wox-theme-icon-color)" d="M0 0h10v10H0z"/><path fill="#4D6BFE" d="M10 0h10v10H10z"/></svg>`}
	palette := defaultPalette()
	palette.resultTitle = woxui.Color{R: 40, G: 60, B: 80, A: 255}
	palette.selectedTitle = woxui.Color{R: 255, G: 255, B: 255, A: 255}
	app := &App{palette: palette, images: map[string]*woxui.Image{}, imageRequested: map[string]string{}, imageLastUsed: map[string]uint64{}, imageErrors: map[string]string{}}
	for _, selected := range []bool{false, true, false} {
		want := palette.resultTitle
		if selected {
			want = palette.selectedTitle
		}
		key := imageKey(source) + fmt.Sprintf("-svg-20-icon-%02x%02x%02x%02x", want.R, want.G, want.B, want.A)
		if palette.isDark() {
			key = imageKey(source) + fmt.Sprintf("-svg-20-dark-icon-%02x%02x%02x%02x", want.R, want.G, want.B, want.A)
		}
		decoded, err := decodeWoxImageWithTintDimensions(source, nil, 20, 10, palette.isDark(), &want)
		if err != nil {
			t.Fatal(err)
		}
		app.images[key] = decoded
		got := app.imageForResult(source, 20, palette, selected)
		if got != decoded {
			t.Fatal("result icon did not use its row-color cache entry")
		}
		if pixel := got.RGBAAt(5, 5); pixel != (color.RGBA{R: want.R, G: want.G, B: want.B, A: want.A}) {
			t.Fatalf("selected=%v: icon = %+v, want %+v", selected, pixel, want)
		}
		if pixel := got.RGBAAt(15, 5); pixel != (color.RGBA{R: 0x4d, G: 0x6b, B: 0xfe, A: 255}) {
			t.Fatalf("brand color changed: %+v", pixel)
		}
	}
}

// TestResultIconSelectionReusesRemoteBitmap verifies that moving the active result
// does not reload a remote icon through the light placeholder.
func TestDecodeSVGAnimationIsPlayback(t *testing.T) {
	const source = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10" fill="#fff"><animate attributeName="opacity" from="0" to="1" dur="1s" repeatCount="indefinite" fill="freeze"/></rect></svg>`
	decoded, err := decodeSVGImage(source, 10, 10, nil, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !decoded.IsAnimated() || decoded.FrameCount() < 2 {
		t.Fatalf("frames = %d, want an animated SVG", decoded.FrameCount())
	}
	if decoded.Frame(0).IsAnimated() {
		t.Fatal("a playback frame must not start another animation")
	}
	if decoded.Frame(0).RGBAAt(5, 5).A > 20 {
		t.Fatalf("first frame alpha = %d, want the transparent pose", decoded.Frame(0).RGBAAt(5, 5).A)
	}
}

func TestResultIconSelectionReusesRemoteBitmap(t *testing.T) {
	payload, err := json.Marshal(lazyImagePayload{
		Token: "token", CacheKey: "icon-cache",
		Placeholder: woxImage{ImageType: "svg", ImageData: `<svg xmlns="http://www.w3.org/2000/svg"/>`},
	})
	if err != nil {
		t.Fatal(err)
	}
	source := woxImage{ImageType: "lazyloadimage", ImageData: string(payload)}
	palette := defaultPalette()
	palette.resultTitle = woxui.Color{R: 40, G: 60, B: 80, A: 255}
	palette.selectedTitle = woxui.Color{R: 255, G: 255, B: 255, A: 255}
	app := &App{
		palette:        palette,
		images:         map[string]*woxui.Image{},
		imageRequested: map[string]string{},
		imageLastUsed:  map[string]uint64{},
		imageErrors:    map[string]string{},
	}
	decoded := &woxui.Image{Width: 20, Height: 20}
	key, _, _ := imageAppearanceCacheKey(source, nil, 20, 20, palette.isDark(), nil)
	app.images[key] = decoded

	if got := app.imageForResult(source, 20, palette, false); got != decoded {
		t.Fatal("idle remote icon missed its cached bitmap")
	}
	if got := app.imageForResult(source, 20, palette, true); got != decoded {
		t.Fatal("selecting a remote icon started a new decode")
	}
	if len(app.imageRequested) != 0 {
		t.Fatalf("selection requested another image load: %+v", app.imageRequested)
	}

	plain := woxImage{ImageType: "svg", ImageData: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><path fill="#111111" d="M0 0h10v10H0z"/></svg>`}
	plainKey, _, _ := imageAppearanceCacheKey(plain, nil, 20, 20, palette.isDark(), nil)
	plainImage := &woxui.Image{Width: 20, Height: 20}
	app.images[plainKey] = plainImage
	if app.imageForResult(plain, 20, palette, true) != plainImage {
		t.Fatal("fixed-color SVG was keyed by the selected row color")
	}

	path := filepath.Join(t.TempDir(), "icon.svg")
	if err := os.WriteFile(path, []byte(`<svg xmlns="http://www.w3.org/2000/svg"><path fill="#111" d="M0 0h1v1H0z"/></svg>`), 0644); err != nil {
		t.Fatal(err)
	}
	fileIcon := woxImage{ImageType: "absolute", ImageData: path}
	fileKey, _, _ := imageAppearanceCacheKey(fileIcon, nil, 20, 20, palette.isDark(), nil)
	fileImage := &woxui.Image{Width: 20, Height: 20}
	app.images[fileKey] = fileImage
	if app.imageForResult(fileIcon, 20, palette, true) != fileImage {
		t.Fatal("on-disk SVG without the theme variable was redecoded for selection")
	}
}

// TestResultIconKeepsPreviousBitmapWhileReplacementDecodes verifies that an
// UpdateResult icon swap does not blank the row before the new bitmap exists.
func TestResultIconKeepsPreviousBitmapWhileReplacementDecodes(t *testing.T) {
	palette := defaultPalette()
	app := &App{
		palette:        palette,
		images:         map[string]*woxui.Image{},
		imageRequested: map[string]string{},
		imageLastUsed:  map[string]uint64{},
		imageErrors:    map[string]string{},
	}
	current := woxImage{ImageType: "svg", ImageData: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10" fill="#111111"/></svg>`}
	next := woxImage{ImageType: "svg", ImageData: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10" fill="#222222"/></svg>`}
	currentImage := &woxui.Image{Width: 20, Height: 20}
	nextImage := &woxui.Image{Width: 20, Height: 20}
	currentKey, _, _ := imageAppearanceCacheKey(current, nil, 20, 20, palette.isDark(), nil)
	nextKey, _, _ := imageAppearanceCacheKey(next, nil, 20, 20, palette.isDark(), nil)
	app.images[currentKey] = currentImage

	if got := app.imageForResultRow("status", current, 20, palette, false); got != currentImage {
		t.Fatal("decoded icon was not shown")
	}
	// The replacement is already in flight and has no bitmap yet.
	app.imageRequested[nextKey] = next.ImageData
	if got := app.imageForResultRow("status", next, 20, palette, false); got != currentImage {
		t.Fatal("in-flight replacement cleared the visible icon")
	}
	if got := app.imageForResultRow("other", next, 20, palette, false); got != nil {
		t.Fatal("another result reused the retained icon")
	}
	if got := app.imageForResultRow("", next, 20, palette, false); got != nil {
		t.Fatal("an empty result id reused a retained icon")
	}
	app.imageErrors[nextKey] = "decode failed"
	if got := app.imageForResultRow("status", next, 20, palette, false); got != currentImage {
		t.Fatal("decode failure cleared the visible icon")
	}

	app.images[nextKey] = nextImage
	if got := app.imageForResultRow("status", next, 20, palette, false); got != nextImage {
		t.Fatal("decoded replacement was not shown")
	}
	if got := app.imageForResultRow("status", woxImage{}, 20, palette, false); got != nil {
		t.Fatal("cleared icon kept the previous bitmap")
	}
	delete(app.images, nextKey)
	if got := app.imageForResultRow("status", next, 20, palette, false); got != nil {
		t.Fatal("cleared icon was restored while its replacement was still decoding")
	}

	app.retainedResultIcons = map[string]*woxui.Image{"stay": currentImage, "gone": nextImage}
	app.pruneRetainedResultIcons([]queryResult{{ID: "stay"}, {ID: "group", IsGroup: true}}, 1)
	if app.retainedResultIcons["stay"] != currentImage {
		t.Fatal("prune dropped a result that is still listed")
	}
	if _, ok := app.retainedResultIcons["gone"]; ok {
		t.Fatal("prune kept a result that left the list")
	}
	app.retainedResultIcons["later"] = nextImage
	app.pruneRetainedResultIcons([]queryResult{{ID: "stay"}}, 1)
	if app.retainedResultIcons["later"] != nextImage {
		t.Fatal("prune scanned the same result revision again")
	}
}
