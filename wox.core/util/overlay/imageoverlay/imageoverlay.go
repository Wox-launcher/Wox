package imageoverlay

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"  // Register GIF header decoding for image overlays.
	_ "image/jpeg" // Register JPEG header decoding for image overlays.
	_ "image/png"  // Register PNG header decoding for image overlays.
	"math"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"wox/common"
	"wox/i18n"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
	"wox/util/imagecache"
	"wox/util/mouse"
	"wox/util/overlay"
	"wox/util/overlay/textoverlay"
	"wox/util/screen"
)

const imageOverlayPrefix = "wox_image_overlay_"
const defaultImageOverlayCornerRadius = 16
const imageOverlayTitleBarHeight = 40
const imageOverlayMinWidth = 180
const imageOverlayMinHeight = 120
const imageOverlayWheelSensitivity = 0.0025

type overlayImageKind string

const (
	overlayImageKindImage overlayImageKind = "image"
	overlayImageKindFile  overlayImageKind = "file"
)

type overlayImage struct {
	kind     overlayImageKind
	image    image.Image
	filePath string
}

func newImageOverlaySource(img image.Image) overlayImage {
	return overlayImage{kind: overlayImageKindImage, image: img}
}

func newFileOverlaySource(filePath string) overlayImage {
	return overlayImage{kind: overlayImageKindFile, filePath: filePath}
}

// Options describes an image overlay request shared by preview and pinning
// features. Width and height are optional; when either side is missing the helper reads image
// metadata so callers do not duplicate file-header parsing.
type Options struct {
	ID               string
	Image            common.WoxImage
	Width            float64
	Height           float64
	OffsetX          float64
	OffsetY          float64
	Anchor           int
	FitToScreen      bool
	Topmost          bool
	Movable          bool
	AbsolutePosition bool
	CornerRadius     float64
	CloseOnEscape    bool
	Closable         bool
	Title            string
}

// Show prepares the image source and displays it as a runtime overlay. It keeps
// URL loading feedback, cache reuse, local files, base64/SVG decode, and common
// sizing in one place for image preview, screenshot pinning, and future overlay image consumers.
func Show(ctx context.Context, opts Options) error {
	opts = normalizeImageOverlayOptions(opts)
	showLoading := opts.Image.ImageType == common.WoxImageTypeUrl
	if showLoading {
		showImageOverlayLoadingOverlay(ctx, opts)
	}

	overlayImage, sourceWidth, sourceHeight, err := prepareImageOverlay(ctx, opts.Image)
	if err != nil {
		if showLoading {
			showImageOverlayErrorOverlay(ctx, opts)
		}
		return err
	}

	width := opts.Width
	height := opts.Height
	if width < 1 {
		width = sourceWidth
	}
	if height < 1 {
		height = sourceHeight
	}
	if opts.FitToScreen {
		width, height = fitImageOverlaySize(width, height)
	}
	title := opts.Title
	if title == "" {
		title = imageOverlayTitle(opts.Image)
	}
	decoded, err := decodeOverlayImage(overlayImage)
	if err != nil {
		return err
	}
	runtimeImage, err := woxui.NewImage(decoded)
	if err != nil {
		return fmt.Errorf("failed to create runtime image overlay: %w", err)
	}
	logo, err := common.WoxIcon.ToImageWithoutRemoteFetch()
	if err != nil {
		return fmt.Errorf("failed to decode Wox logo: %w", err)
	}
	runtimeLogo, err := woxui.NewImage(logo)
	if err != nil {
		return fmt.Errorf("failed to create runtime Wox logo: %w", err)
	}

	window := overlay.WindowOptions{
		ID:            opts.ID,
		Movable:       opts.Movable,
		Resizable:     true,
		CornerRadius:  opts.CornerRadius,
		AspectRatio:   width / (height + imageOverlayTitleBarHeight),
		CloseOnEscape: opts.CloseOnEscape,
		TakeFocus:     opts.CloseOnEscape,
		Topmost:       opts.Topmost,
		// Bug fix: pinned screenshots already carry desktop-absolute coordinates from the
		// screenshot workspace. Mark that contract explicitly so Windows does not treat the offset
		// as a notification-style displacement from the primary work area and clamp it back there.
		AbsolutePosition: opts.AbsolutePosition,
		Anchor:           opts.Anchor,
		OffsetX:          opts.OffsetX,
		OffsetY:          opts.OffsetY,
		Width:            width,
		Height:           height + imageOverlayTitleBarHeight,
	}
	overlay.ShowWindow(window, overlay.View{Kind: "image", Build: func(_ *woxui.Window, frame woxui.FrameInfo) woxwidget.Widget {
		foreground := woxui.Color{R: 245, G: 245, B: 245, A: 255}
		bodyHeight := max(float32(1), frame.Size.Height-imageOverlayTitleBarHeight)
		children := []woxwidget.StackChild{
			{Child: woxwidget.Container{
				Width: frame.Size.Width, Height: frame.Size.Height,
				Color: woxui.Color{R: 24, G: 24, B: 26, A: 255}, BorderColor: woxui.Color{R: 255, G: 255, B: 255, A: 30}, BorderWidth: 1,
			}},
			{Top: imageOverlayTitleBarHeight, Child: woxwidget.Image{
				Source: runtimeImage, Width: frame.Size.Width, Height: bodyHeight, Fit: woxwidget.ImageFitContain,
			}},
			{Left: 12, Top: 10, Child: woxwidget.Image{Source: runtimeLogo, Width: 20, Height: 20, Fit: woxwidget.ImageFitContain}},
			{Left: 40, Top: 9, Child: woxwidget.TextBlock{
				Value: title, Width: max(float32(0), frame.Size.Width-88), Height: 24, MaxLines: 1,
				Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: foreground,
			}},
			{Top: imageOverlayTitleBarHeight - 1, Child: woxwidget.Container{Width: frame.Size.Width, Height: 1, Color: woxui.Color{R: 255, G: 255, B: 255, A: 76}}},
		}
		if opts.Closable || opts.CloseOnEscape {
			children = append(children, woxwidget.StackChild{Right: 0, Top: 0, AnchorRight: true, Child: overlay.TitleBarCloseButton(runtime.GOOS == "windows", "image-overlay-close", foreground, func() { overlay.RequestClose(opts.ID) })})
		}
		return woxwidget.Stack{Width: frame.Size.Width, Height: frame.Size.Height, Children: children}
	}, OnPointer: func(event woxui.PointerEvent) {
		if event.Kind != woxui.PointerScroll || event.Position.Y < imageOverlayTitleBarHeight || event.Scroll.Y == 0 {
			return
		}
		factor := float32(math.Exp(float64(event.Scroll.Y) * imageOverlayWheelSensitivity))
		overlay.ScaleWindow(opts.ID, min(max(factor, 0.8), 1.25), imageOverlayMinWidth, imageOverlayMinHeight)
	}})
	return nil
}

// imageOverlayTitle derives a compact title from file and URL sources.
func imageOverlayTitle(source common.WoxImage) string {
	if source.ImageType == common.WoxImageTypeAbsolutePath {
		if title := filepath.Base(source.ImageData); title != "." && title != string(filepath.Separator) {
			return title
		}
	}
	if source.ImageType == common.WoxImageTypeUrl {
		if parsed, err := url.Parse(source.ImageData); err == nil {
			if title := filepath.Base(parsed.Path); title != "." && title != string(filepath.Separator) && title != "/" {
				return title
			}
			if parsed.Host != "" {
				return parsed.Host
			}
		}
	}
	return "Image"
}

// decodeOverlayImage turns prepared memory or file sources into runtime pixels.
func decodeOverlayImage(source overlayImage) (image.Image, error) {
	if source.kind == overlayImageKindImage && source.image != nil {
		return source.image, nil
	}
	if source.kind != overlayImageKindFile || source.filePath == "" {
		return nil, fmt.Errorf("image overlay source is empty")
	}
	file, err := os.Open(source.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image overlay file: %w", err)
	}
	defer file.Close()
	decoded, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image overlay file: %w", err)
	}
	return decoded, nil
}

func normalizeImageOverlayOptions(opts Options) Options {
	if opts.ID == "" {
		opts.ID = imageOverlayPrefix + opts.Image.Hash()
	}
	if opts.CornerRadius <= 0 {
		// Feature change: image overlay corner radius is now configurable, while the default is
		// intentionally larger than the first 8pt pass so the standalone preview surface reads as
		// rounded after scaling on high-DPI desktop screens.
		opts.CornerRadius = defaultImageOverlayCornerRadius
	}
	if !opts.AbsolutePosition && opts.OffsetX == 0 && opts.OffsetY == 0 {
		if pos, ok := mouse.CurrentPosition(); ok {
			// Image previews are user-triggered reference surfaces, so the natural default is the
			// current cursor location rather than the primary screen's notification position.
			opts.AbsolutePosition = true
			opts.Anchor = overlay.AnchorCenter
			opts.OffsetX = pos.X
			opts.OffsetY = pos.Y
		}
	}
	return opts
}

func showImageOverlayLoadingOverlay(ctx context.Context, opts Options) {
	// Feature change: URL image overlays acknowledge the click before network download and cache
	// preparation. Local files, screenshots, base64, and inline SVG stay direct because they do not
	// need a separate waiting state.
	start := time.Now()
	textoverlay.Show(textoverlay.Options{
		Window: overlay.WindowOptions{
			ID:            opts.ID,
			Anchor:        overlay.AnchorCenter,
			Width:         200,
			Movable:       true,
			CloseOnEscape: true,
			Topmost:       true,
		},
		Message:  i18n.GetI18nManager().TranslateWox(ctx, "ui_preview_image_loading"),
		Loading:  true,
		IconSize: 20,
	})
	util.GetLogger().Info(ctx, fmt.Sprintf("image overlay loading shown: id=%s, cost=%s", opts.ID, time.Since(start)))
}

func showImageOverlayErrorOverlay(ctx context.Context, opts Options) {
	// URL overlay failures replace the loading window with a localized error while the caller
	// receives the concrete error for route/API handling.
	textoverlay.Show(textoverlay.Options{
		Window: overlay.WindowOptions{
			ID:            opts.ID,
			Anchor:        overlay.AnchorCenter,
			Width:         220,
			CloseOnEscape: true,
			Topmost:       true,
		},
		Closable:         true,
		AutoCloseSeconds: 6,
		Message:          i18n.GetI18nManager().TranslateWox(ctx, "ui_preview_image_load_failed"),
	})
}

// prepareImageOverlay returns an image source plus intrinsic dimensions without showing a window.
func prepareImageOverlay(ctx context.Context, woxImage common.WoxImage) (overlayImage, float64, float64, error) {
	if woxImage.ImageType == common.WoxImageTypeUrl {
		return prepareURLImageOverlay(ctx, woxImage.ImageData)
	}

	if woxImage.ImageType == common.WoxImageTypeAbsolutePath && !strings.EqualFold(filepath.Ext(woxImage.ImageData), ".svg") {
		return prepareFileImageOverlay(ctx, woxImage.ImageData)
	}

	if woxImage.ImageType != common.WoxImageTypeAbsolutePath && woxImage.ImageType != common.WoxImageTypeBase64 && woxImage.ImageType != common.WoxImageTypeSvg {
		return overlayImage{}, 0, 0, fmt.Errorf("image overlay does not support image type: %s", woxImage.ImageType)
	}

	decodeStart := time.Now()
	img, err := woxImage.ToImage()
	if err != nil {
		return overlayImage{}, 0, 0, fmt.Errorf("failed to decode image overlay source: %w", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return overlayImage{}, 0, 0, fmt.Errorf("image overlay source has invalid size")
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("image overlay decoded: type=%s, dataLength=%d, size=%dx%d, decodeCost=%s", woxImage.ImageType, len(woxImage.ImageData), bounds.Dx(), bounds.Dy(), time.Since(decodeStart)))
	return newImageOverlaySource(img), float64(bounds.Dx()), float64(bounds.Dy()), nil
}

func prepareFileImageOverlay(ctx context.Context, filePath string) (overlayImage, float64, float64, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return overlayImage{}, 0, 0, fmt.Errorf("failed to read image overlay file info: %w", err)
	}
	if info.IsDir() {
		return overlayImage{}, 0, 0, fmt.Errorf("image overlay path is a directory")
	}
	if info.Size() == 0 {
		return overlayImage{}, 0, 0, fmt.Errorf("image overlay file is empty")
	}

	headerStart := time.Now()
	width, height, err := readFileImageSize(filePath)
	if err != nil {
		return overlayImage{}, 0, 0, fmt.Errorf("failed to read image overlay file size: %w", err)
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("image overlay file prepared: path=%s, fileBytes=%d, size=%dx%d, headerCost=%s", filePath, info.Size(), width, height, time.Since(headerStart)))
	return newFileOverlaySource(filePath), float64(width), float64(height), nil
}

func prepareURLImageOverlay(ctx context.Context, imageURL string) (overlayImage, float64, float64, error) {
	parsedURL, err := url.Parse(imageURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return overlayImage{}, 0, 0, fmt.Errorf("image overlay only supports http/https image urls")
	}

	cachePath := buildURLImageOverlayCachePath(imageURL, parsedURL.Path)
	if cachedInfo, statErr := os.Stat(cachePath); statErr == nil && !cachedInfo.IsDir() && cachedInfo.Size() > 0 {
		headerStart := time.Now()
		width, height, headerErr := readFileImageSize(cachePath)
		if headerErr == nil {
			// Remote preview images are immutable enough for URL-keyed cache reuse.
			util.GetLogger().Info(ctx, fmt.Sprintf("image overlay url cache hit: url=%s, path=%s, fileBytes=%d, size=%dx%d, headerCost=%s", imageURL, cachePath, cachedInfo.Size(), width, height, time.Since(headerStart)))
			imagecache.Touch(ctx, cachePath, cachedInfo)
			return newFileOverlaySource(cachePath), float64(width), float64(height), nil
		}
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to read cached image overlay header, refreshing cache: url=%s path=%s err=%s", imageURL, cachePath, headerErr.Error()))
	}

	// Cache remote raster bytes so repeated previews only pay the runtime decode cost.
	totalStart := time.Now()
	downloadStart := time.Now()
	data, err := util.HttpGet(ctx, imageURL)
	if err != nil {
		return overlayImage{}, 0, 0, fmt.Errorf("failed to download image overlay url: %w", err)
	}
	downloadCost := time.Since(downloadStart)

	headerStart := time.Now()
	if strings.EqualFold(filepath.Ext(parsedURL.Path), ".svg") {
		svgImage := common.NewWoxImageSvg(string(data))
		img, err := svgImage.ToImage()
		if err != nil {
			return overlayImage{}, 0, 0, fmt.Errorf("failed to decode image overlay svg url: %w", err)
		}
		bounds := img.Bounds()
		if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
			return overlayImage{}, 0, 0, fmt.Errorf("image overlay url has invalid size")
		}
		util.GetLogger().Info(ctx, fmt.Sprintf("image overlay url prepared: url=%s, downloadedBytes=%d, size=%dx%d, downloadCost=%s, decodeCost=%s, totalCost=%s", imageURL, len(data), bounds.Dx(), bounds.Dy(), downloadCost, time.Since(headerStart), time.Since(totalStart)))
		return newImageOverlaySource(img), float64(bounds.Dx()), float64(bounds.Dy()), nil
	}

	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return overlayImage{}, 0, 0, fmt.Errorf("failed to decode image overlay url header: %w", err)
	}
	if config.Width <= 0 || config.Height <= 0 {
		return overlayImage{}, 0, 0, fmt.Errorf("image overlay url has invalid size")
	}

	writeStart := time.Now()
	if writeErr := writeURLImageOverlayCache(cachePath, data); writeErr != nil {
		return overlayImage{}, 0, 0, fmt.Errorf("failed to cache image overlay url: %w", writeErr)
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("image overlay url prepared: url=%s, cachePath=%s, downloadedBytes=%d, size=%dx%d, downloadCost=%s, headerCost=%s, writeCost=%s, totalCost=%s", imageURL, cachePath, len(data), config.Width, config.Height, downloadCost, time.Since(headerStart), time.Since(writeStart), time.Since(totalStart)))
	return newFileOverlaySource(cachePath), float64(config.Width), float64(config.Height), nil
}

// fitImageOverlaySize caps preview-style overlays to the active screen while preserving aspect
// ratio. Pinning callers can skip this by passing explicit logical selection dimensions.
func fitImageOverlaySize(sourceWidth, sourceHeight float64) (float64, float64) {
	if sourceWidth < 1 || sourceHeight < 1 {
		return 1, 1
	}

	activeScreen := screen.GetActiveScreen()
	maxWidth := float64(activeScreen.Width) * 0.86
	maxHeight := float64(activeScreen.Height) * 0.86
	if maxWidth < 1 || maxHeight < 1 {
		return sourceWidth, sourceHeight
	}

	scale := 1.0
	if sourceWidth > maxWidth || sourceHeight > maxHeight {
		scale = maxWidth / sourceWidth
		heightScale := maxHeight / sourceHeight
		if heightScale < scale {
			scale = heightScale
		}
	}
	if scale <= 0 {
		scale = 1
	}

	width := sourceWidth * scale
	height := sourceHeight * scale
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	return width, height
}

// readFileImageSize reads only the encoded image header when sizing an overlay.
func readFileImageSize(filePath string) (int, int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, err
	}
	return config.Width, config.Height, nil
}

func buildURLImageOverlayCachePath(imageURL string, urlPath string) string {
	ext := strings.ToLower(filepath.Ext(urlPath))
	if ext == "" || len(ext) > 10 || strings.ContainsAny(ext, `/\`) {
		ext = ".img"
	}
	return filepath.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("image_overlay_url_%s%s", util.Md5([]byte(imageURL)), ext))
}

func writeURLImageOverlayCache(cachePath string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return err
	}
	// A unique temp file keeps concurrent writers independent while rename publishes atomically.
	tmpFile, err := os.CreateTemp(filepath.Dir(cachePath), filepath.Base(cachePath)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	if closeErr := tmpFile.Close(); closeErr != nil {
		_ = os.Remove(tmpPath)
		return closeErr
	}
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, cachePath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}
