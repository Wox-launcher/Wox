package launcher

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"wox/common"
	"wox/resource"
	woxui "wox/ui/runtime"
	"wox/util"
	woxsvg "wox/util/svg"
)

type woxImage struct {
	ImageType string `json:"ImageType"`
	ImageData string `json:"ImageData"`
}

var appIconImageSource = woxImage{ImageType: "appicon", ImageData: "embedded"}

// UnmarshalJSON accepts both the structured image DTO and legacy type:data strings.
func (w *woxImage) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		if value == "" {
			*w = woxImage{}
			return nil
		}
		imageType, imageData, ok := strings.Cut(value, ":")
		if !ok {
			return fmt.Errorf("invalid Wox image string")
		}
		w.ImageType = imageType
		w.ImageData = imageData
		return nil
	}
	type imageAlias woxImage
	var decoded imageAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*w = woxImage(decoded)
	return nil
}

type lazyImagePayload struct {
	Token       string   `json:"token"`
	CacheKey    string   `json:"cacheKey"`
	Placeholder woxImage `json:"placeholder"`
}

type svgTextElement struct {
	X          float32 `xml:"x,attr"`
	FontSize   float32 `xml:"font-size,attr"`
	TextAnchor string  `xml:"text-anchor,attr"`
	Fill       string  `xml:"fill,attr"`
	Value      string  `xml:",chardata"`
}

type svgTextDocument struct {
	Width   string           `xml:"width,attr"`
	Height  string           `xml:"height,attr"`
	ViewBox string           `xml:"viewBox,attr"`
	Texts   []svgTextElement `xml:"text"`
}

type svgCenteredText struct {
	Value string
	Color woxui.Color
	Size  float32
}

const (
	launcherImageCacheLimit    = 512
	launcherImageCacheMaxBytes = 32 << 20
	hiddenImageCacheKeepCount  = 64
	hiddenImageCacheMaxBytes   = 8 << 20
)

// imageCacheBytes accounts for one decoded RGBA buffer in the dual-budget LRU.
func imageCacheBytes(image *woxui.Image) int {
	if image == nil {
		return 0
	}
	return image.Width * image.Height * 4
}

func (a *App) imageFor(source woxImage) *woxui.Image {
	return a.imageForTint(source, nil, 256)
}

// imageForSize resolves display images at a caller-selected resolution while sharing the image cache.
func (a *App) imageForSize(source woxImage, size int) *woxui.Image {
	return a.imageForTint(source, nil, size)
}

// imageForDimensions preserves non-square SVG geometry at the requested physical resolution.
func (a *App) imageForDimensions(source woxImage, width, height int) *woxui.Image {
	return a.imageForTintDimensions(source, nil, width, height)
}

// physicalImageSize keeps rasterized vector assets sharp at the window's current backing scale.
func physicalImageSize(logicalSize int, scale float32) int {
	if scale <= 0 {
		scale = 1
	}
	return max(1, int(math.Ceil(float64(float32(logicalSize)*scale))))
}

// previewImageRequestSize returns a bounded raster size for images displayed in a preview surface.
func previewImageRequestSize(width, height float32) int {
	maxDimension := max(width, height)
	return int(min(float32(2048), max(float32(512), float32(math.Ceil(float64(maxDimension*2))))))
}

// centeredSVGText extracts a single centered label so the native text renderer can cover SVG text unsupported by the rasterizer.
func centeredSVGText(source woxImage, targetWidth, targetHeight float32) (svgCenteredText, bool) {
	if source.ImageType != "svg" || source.ImageData == "" || targetWidth <= 0 || targetHeight <= 0 {
		return svgCenteredText{}, false
	}
	var document svgTextDocument
	if err := xml.Unmarshal([]byte(source.ImageData), &document); err != nil || len(document.Texts) != 1 {
		return svgCenteredText{}, false
	}
	text := document.Texts[0]
	value := strings.TrimSpace(text.Value)
	if value == "" || !strings.EqualFold(strings.TrimSpace(text.TextAnchor), "middle") || text.FontSize <= 0 {
		return svgCenteredText{}, false
	}
	viewWidth, viewHeight, ok := svgDocumentSize(document)
	if !ok || math.Abs(float64(text.X-viewWidth/2)) > 0.01 {
		return svgCenteredText{}, false
	}
	scale := min(targetWidth/viewWidth, targetHeight/viewHeight)
	color, ok := decodeThemeColor(text.Fill)
	if !ok {
		color = woxui.Color{A: 255}
	}
	return svgCenteredText{Value: value, Color: color, Size: text.FontSize * scale}, true
}

// svgDocumentSize resolves the coordinate space used to scale an extracted text label.
func svgDocumentSize(document svgTextDocument) (float32, float32, bool) {
	parts := strings.Fields(document.ViewBox)
	if len(parts) == 4 {
		width, widthErr := strconv.ParseFloat(parts[2], 32)
		height, heightErr := strconv.ParseFloat(parts[3], 32)
		if widthErr == nil && heightErr == nil && width > 0 && height > 0 {
			return float32(width), float32(height), true
		}
	}
	width, widthErr := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(document.Width), "px"), 32)
	height, heightErr := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(document.Height), "px"), 32)
	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 {
		return 0, 0, false
	}
	return float32(width), float32(height), true
}

// imageForTint applies a source-in tint to SVG images and sets the resolution for core-resolved assets.
func (a *App) imageForTint(source woxImage, tint *woxui.Color, svgSize int) *woxui.Image {
	return a.imageForTintDimensions(source, tint, svgSize, svgSize)
}

// imageForTintDimensions keeps cache and decode dimensions aligned for rectangular SVGs.
func (a *App) imageForTintDimensions(source woxImage, tint *woxui.Color, svgWidth, svgHeight int) *woxui.Image {
	if source.ImageType == "" || source.ImageData == "" {
		return nil
	}
	svgWidth = max(1, svgWidth)
	svgHeight = max(1, svgHeight)
	variantKey := imageVariantKey(source, tint)
	key := imageKey(source)
	if svgWidth == svgHeight {
		key += fmt.Sprintf("-svg-%d", svgWidth)
	} else {
		key += fmt.Sprintf("-svg-%dx%d", svgWidth, svgHeight)
	}
	if tint != nil {
		key += fmt.Sprintf("-tint-%02x%02x%02x%02x", tint.R, tint.G, tint.B, tint.A)
	}
	a.imageMu.Lock()
	if a.imageVariants == nil {
		a.imageVariants = map[string]string{}
	}
	if a.imageVariantKeys == nil {
		a.imageVariantKeys = map[string]string{}
	}
	a.imageUseSequence++
	a.imageLastUsed[key] = a.imageUseSequence
	a.imageVariantKeys[key] = variantKey
	cachedImage := a.images[key]
	image := cachedImage
	if cachedImage == nil {
		image = a.images[a.imageVariants[variantKey]]
	}
	requestedSource, requested := a.imageRequested[key]
	if cachedImage != nil || requested && requestedSource == source.ImageData {
		a.imageMu.Unlock()
		return image
	}
	a.imageRequested[key] = source.ImageData
	delete(a.imageErrors, key)
	a.imageMu.Unlock()
	util.Go(a.lifecycleCtx, "load launcher image", func() {
		a.loadImage(key, source, tint, svgWidth, svgHeight)
	})
	return nil
}

func (a *App) loadImage(key string, source woxImage, tint *woxui.Color, svgWidth, svgHeight int) {
	if source.ImageType == "lazyloadimage" {
		var payload lazyImagePayload
		if err := json.Unmarshal([]byte(source.ImageData), &payload); err != nil {
			log.Printf("decode lazy result image payload: %v", err)
			a.storeImageError(key, err)
			return
		}
		if placeholder, err := decodeWoxImageWithTintDimensions(payload.Placeholder, tint, svgWidth, svgHeight); err == nil {
			a.storeImage(key, placeholder)
		}
		if payload.Token == "" {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		loaded, err := a.services.LoadLazyResultImage(ctx, a.sessionID, payload.Token)
		cancel()
		if err != nil {
			log.Printf("load lazy result image: %v", err)
			a.storeImageError(key, err)
			return
		}
		resolved := woxImage{ImageType: loaded.ImageType, ImageData: loaded.ImageData}
		image, err := decodeWoxImageWithTintDimensions(resolved, tint, svgWidth, svgHeight)
		if err != nil {
			log.Printf("decode resolved lazy result image: %v", err)
			a.storeImageError(key, err)
			return
		}
		a.storeImage(key, image)
		return
	}
	if source.ImageType == "url" || source.ImageType == "emoji" || source.ImageType == "fileicon" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		loaded, err := a.services.ResolveImage(ctx, a.sessionID, common.WoxImage{ImageType: source.ImageType, ImageData: source.ImageData}, max(svgWidth, svgHeight))
		cancel()
		if err != nil {
			log.Printf("resolve %s result image %q: %v", source.ImageType, source.ImageData, err)
			a.storeImageError(key, err)
			return
		}
		resolved := woxImage{ImageType: loaded.ImageType, ImageData: loaded.ImageData}
		image, err := decodeWoxImageWithTintDimensions(resolved, tint, svgWidth, svgHeight)
		if err != nil {
			log.Printf("decode resolved %s result image: %v", source.ImageType, err)
			a.storeImageError(key, err)
			return
		}
		a.storeImage(key, image)
		return
	}

	image, err := decodeWoxImageWithTintDimensions(source, tint, svgWidth, svgHeight)
	if err != nil {
		log.Printf("decode %s result image: %v", source.ImageType, err)
		a.storeImageError(key, err)
		return
	}
	a.storeImage(key, image)
}

func (a *App) storeImage(key string, image *woxui.Image) {
	if image == nil {
		return
	}
	if err := a.runOnUI("store launcher image", func() {
		a.imageMu.Lock()
		a.insertImageLocked(key, image)
		if variantKey := a.imageVariantKeys[key]; variantKey != "" {
			if a.imageVariants == nil {
				a.imageVariants = map[string]string{}
			}
			a.imageVariants[variantKey] = key
		}
		a.imagesRevision.Add(1)
		delete(a.imageErrors, key)
		a.imageMu.Unlock()
		a.invalidateAllWindows()
	}); err != nil {
		log.Printf("dispatch launcher image: %v", err)
	}
}

func (a *App) storeImageError(key string, err error) {
	if dispatchErr := a.runOnUI("store launcher image error", func() {
		a.imageMu.Lock()
		a.imageErrors[key] = err.Error()
		a.imageMu.Unlock()
		a.invalidateAllWindows()
	}); dispatchErr != nil {
		log.Printf("dispatch launcher image error: %v", dispatchErr)
	}
}

// insertImageLocked stores one decoded image and evicts cold entries to the visible dual budget.
func (a *App) insertImageLocked(key string, image *woxui.Image) {
	if a.images == nil {
		a.images = map[string]*woxui.Image{}
	}
	if existing, exists := a.images[key]; exists {
		a.imageCacheSize -= imageCacheBytes(existing)
		delete(a.images, key)
	}
	incoming := imageCacheBytes(image)
	if incoming > launcherImageCacheMaxBytes {
		a.clearImageCacheLocked(key)
	} else {
		a.evictImagesToBudget(key, launcherImageCacheLimit-1, launcherImageCacheMaxBytes-incoming)
	}
	a.images[key] = image
	a.imageCacheSize += incoming
}

// trimIdleImageCache evicts cold decoded images while the launcher stays hidden.
// Must run on the UI thread like every other access to the image maps.
func (a *App) trimIdleImageCache() {
	a.imageMu.Lock()
	defer a.imageMu.Unlock()
	a.evictImagesToBudget("", hiddenImageCacheKeepCount, hiddenImageCacheMaxBytes)
	a.imagesRevision.Add(1)
}

func (a *App) imageCacheByteSizeLocked() int {
	if a.imageCacheSize < 0 {
		a.imageCacheSize = 0
	}
	return a.imageCacheSize
}

func (a *App) clearImageCacheLocked(keepKey string) {
	for key := range a.images {
		if key == keepKey {
			continue
		}
		a.removeImageLocked(key)
	}
}

// evictImagesToBudget drops the coldest images until both budgets fit, ordering candidates
// once instead of rescanning the map per eviction. Hiding the launcher can trim hundreds of
// entries in a single call, where the repeated-scan form degraded to O(k*n).
func (a *App) evictImagesToBudget(keepKey string, maxCount, maxBytes int) {
	if len(a.images) <= maxCount && a.imageCacheSize <= maxBytes {
		return
	}
	candidates := make([]string, 0, len(a.images))
	for key := range a.images {
		if key == keepKey {
			continue
		}
		candidates = append(candidates, key)
	}
	sort.Slice(candidates, func(first, second int) bool {
		return a.imageLastUsed[candidates[first]] < a.imageLastUsed[candidates[second]]
	})
	for _, key := range candidates {
		if len(a.images) <= maxCount && a.imageCacheSize <= maxBytes {
			return
		}
		a.removeImageLocked(key)
	}
}

func (a *App) removeImageLocked(key string) {
	if image, ok := a.images[key]; ok {
		a.imageCacheSize -= imageCacheBytes(image)
		if a.imageCacheSize < 0 {
			a.imageCacheSize = 0
		}
	}
	delete(a.images, key)
	if variantKey := a.imageVariantKeys[key]; variantKey != "" && a.imageVariants[variantKey] == key {
		delete(a.imageVariants, variantKey)
	}
	delete(a.imageVariantKeys, key)
	delete(a.imageRequested, key)
	delete(a.imageLastUsed, key)
	delete(a.imageErrors, key)
}

func (a *App) imageErrorFor(source woxImage) string {
	key := imageKey(source)
	a.imageMu.RLock()
	defer a.imageMu.RUnlock()
	return a.imageErrors[key]
}

func decodeWoxImage(source woxImage) (*woxui.Image, error) {
	return decodeWoxImageWithTint(source, nil, 256)
}

func decodeWoxImageWithTint(source woxImage, tint *woxui.Color, svgSize int) (*woxui.Image, error) {
	return decodeWoxImageWithTintDimensions(source, tint, svgSize, svgSize)
}

// decodeWoxImageWithTintDimensions decodes vector sources at their requested width and height.
func decodeWoxImageWithTintDimensions(source woxImage, tint *woxui.Color, svgWidth, svgHeight int) (*woxui.Image, error) {
	switch source.ImageType {
	case "absolute":
		if strings.EqualFold(filepath.Ext(source.ImageData), ".svg") {
			data, err := os.ReadFile(source.ImageData)
			if err != nil {
				return nil, err
			}
			return decodeSVGImage(string(data), svgWidth, svgHeight, tint)
		}
		file, err := os.Open(source.ImageData)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		return woxui.DecodeImage(file)
	case "base64":
		encoded := source.ImageData
		if comma := strings.IndexByte(encoded, ','); comma >= 0 {
			encoded = encoded[comma+1:]
		}
		pixels, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, err
		}
		if strings.Contains(strings.ToLower(source.ImageData), "image/svg+xml") {
			return decodeSVGImage(string(pixels), svgWidth, svgHeight, tint)
		}
		return woxui.DecodeImage(bytes.NewReader(pixels))
	case "svg":
		return decodeSVGImage(source.ImageData, svgWidth, svgHeight, tint)
	case "theme":
		return decodeThemeImage(source.ImageData)
	case "appicon":
		return woxui.DecodeImage(bytes.NewReader(resource.GetAppIconPNG()))
	default:
		return nil, fmt.Errorf("unsupported Wox image type %q", source.ImageType)
	}
}

func decodeSVGImage(data string, width, height int, tint *woxui.Color) (*woxui.Image, error) {
	// Match Flutter/CSS: currentColor defaults to black unless the caller tints the icon.
	var currentColor color.Color
	if tint != nil {
		currentColor = color.NRGBA{R: tint.R, G: tint.G, B: tint.B, A: tint.A}
	}
	rgba, err := woxsvg.RenderWithCurrentColor(data, width, height, currentColor)
	if err != nil {
		return nil, err
	}
	if tint != nil {
		for index := 0; index < len(rgba.Pix); index += 4 {
			alpha := uint8((uint16(rgba.Pix[index+3])*uint16(tint.A) + 127) / 255)
			rgba.Pix[index] = uint8((uint16(tint.R)*uint16(alpha) + 127) / 255)
			rgba.Pix[index+1] = uint8((uint16(tint.G)*uint16(alpha) + 127) / 255)
			rgba.Pix[index+2] = uint8((uint16(tint.B)*uint16(alpha) + 127) / 255)
			rgba.Pix[index+3] = alpha
		}
	}
	return woxui.NewImage(rgba)
}

func decodeThemeImage(data string) (*woxui.Image, error) {
	var theme struct {
		AppBackgroundColor              string
		QueryBoxBackgroundColor         string
		ResultItemActiveBackgroundColor string
		PreviewFontColor                string
	}
	if err := json.Unmarshal([]byte(data), &theme); err != nil {
		return nil, err
	}
	const size = 128
	rgba := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(rgba, rgba.Bounds(), image.NewUniform(themeRasterColor(parseThemeColor(theme.AppBackgroundColor, defaultPalette().background))), image.Point{}, draw.Src)
	draw.Draw(rgba, image.Rect(14, 17, 114, 43), image.NewUniform(themeRasterColor(parseThemeColor(theme.QueryBoxBackgroundColor, defaultPalette().queryBackground))), image.Point{}, draw.Src)
	draw.Draw(rgba, image.Rect(14, 54, 114, 88), image.NewUniform(themeRasterColor(parseThemeColor(theme.ResultItemActiveBackgroundColor, defaultPalette().selectedBackground))), image.Point{}, draw.Src)
	draw.Draw(rgba, image.Rect(23, 102, 105, 108), image.NewUniform(themeRasterColor(parseThemeColor(theme.PreviewFontColor, defaultPalette().previewText))), image.Point{}, draw.Src)
	return woxui.NewImage(rgba)
}

func themeRasterColor(value woxui.Color) color.NRGBA {
	return color.NRGBA{R: value.R, G: value.G, B: value.B, A: value.A}
}

func imageKey(source woxImage) string {
	if source.ImageType == "lazyloadimage" {
		var payload lazyImagePayload
		if json.Unmarshal([]byte(source.ImageData), &payload) == nil && payload.CacheKey != "" {
			// Lazy authorization tokens change for every query and must not invalidate
			// an icon that was already resolved from the same stable source.
			return "lazy-" + payload.CacheKey
		}
	}
	hash := sha256.Sum256([]byte(source.ImageType + "\x00" + source.ImageData))
	return fmt.Sprintf("%x", hash[:])
}

// imageVariantKey identifies an image source and tint independently of its raster size.
func imageVariantKey(source woxImage, tint *woxui.Color) string {
	key := imageKey(source)
	if tint != nil {
		key += fmt.Sprintf("-tint-%02x%02x%02x%02x", tint.R, tint.G, tint.B, tint.A)
	}
	return key
}
