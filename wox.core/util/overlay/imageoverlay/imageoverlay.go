package imageoverlay

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"  // Register GIF header decoding for image overlays.
	_ "image/jpeg" // Register JPEG header decoding for image overlays.
	"image/png"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wox/common"
	"wox/i18n"
	"wox/util"
	"wox/util/imagecache"
	"wox/util/mouse"
	"wox/util/overlay"
	"wox/util/overlay/textoverlay"
	"wox/util/screen"
)

const imageOverlayPrefix = "wox_image_overlay_"
const defaultImageOverlayCornerRadius = 16

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

type imageRenderer struct {
	handle uintptr
	width  float64
	height float64
}

func newImageOverlaySource(img image.Image) overlayImage {
	return overlayImage{kind: overlayImageKindImage, image: img}
}

func newFileOverlaySource(filePath string) overlayImage {
	return overlayImage{kind: overlayImageKindFile, filePath: filePath}
}

// Options describes a native image overlay request shared by preview and pinning
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
}

// Show prepares the image source and displays it as a native overlay. The refactor keeps
// URL loading feedback, cache reuse, local file-backed icons, base64/SVG fallback decode, and common
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

	window := overlay.WindowOptions{
		ID:          opts.ID,
		Transparent: true,
		Movable:     opts.Movable,
		// Feature change: image overlays are user-managed reference surfaces. Making only this
		// shared image path resizable keeps notification overlays fixed while preview and pinned
		// images can be adjusted without adding another public API parameter.
		Resizable:     true,
		CornerRadius:  opts.CornerRadius,
		AspectRatio:   width / height,
		CloseOnEscape: opts.CloseOnEscape,
		Topmost:       opts.Topmost,
		// Bug fix: pinned screenshots already carry desktop-absolute coordinates from the
		// screenshot workspace. Mark that contract explicitly so Windows does not treat the offset
		// as a notification-style displacement from the primary work area and clamp it back there.
		AbsolutePosition: opts.AbsolutePosition,
		Anchor:           opts.Anchor,
		OffsetX:          opts.OffsetX,
		OffsetY:          opts.OffsetY,
		Width:            width,
		Height:           height,
	}
	if renderer, ok := newImageRenderer(opts.ID, overlayImage, width, height, opts.CornerRadius, opts.Closable); ok {
		attachment := renderer.nativeAttachment()
		attachment.OnRelease = renderer.destroy
		window.NativeAttachment = attachment
	}
	overlay.RegisterClickCallback(opts.ID, nil)
	overlay.ShowWindow(window)
	return nil
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
		FontSize: 13,
		IconSize: 20,
	})
	util.GetLogger().Info(ctx, fmt.Sprintf("image overlay loading shown: id=%s, cost=%s", opts.ID, time.Since(start)))
}

func showImageOverlayErrorOverlay(ctx context.Context, opts Options) {
	// Bug fix: URL overlay failures replace the loading window with a localized error instead of
	// leaving stale native UI while the caller receives the concrete error for route/API handling.
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
		FontSize:         13,
	})
}

// prepareImageOverlay returns an overlay icon plus intrinsic dimensions without showing a window.
// Raster files and cached URL images intentionally stay file-backed so large
// images avoid Go-side full decode and PNG bridge encoding.
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
			// Optimization: remote preview images are immutable enough for URL-keyed cache reuse.
			// Reusing the downloaded file keeps repeated overlay opens on the same file-backed
			// native path as local screenshots instead of repeating decode and bridge encoding.
			util.GetLogger().Info(ctx, fmt.Sprintf("image overlay url cache hit: url=%s, path=%s, fileBytes=%d, size=%dx%d, headerCost=%s", imageURL, cachePath, cachedInfo.Size(), width, height, time.Since(headerStart)))
			imagecache.Touch(ctx, cachePath, cachedInfo)
			return newFileOverlaySource(cachePath), float64(width), float64(height), nil
		}
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to read cached image overlay header, refreshing cache: url=%s path=%s err=%s", imageURL, cachePath, headerErr.Error()))
	}

	// Remote raster images are downloaded to cache and then handed to the native layer as files.
	// The old image-preview path decoded full-size photos in Go and then PNG-encoded them again for
	// CGO; file-backed transport keeps large markdown images and screenshot overlays on one fast path.
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

// readFileImageSize reads only the encoded image header. It exists here so callers do not fall back
// to full image decoding when they only need dimensions for a file-backed overlay.
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
	// Refactor compatibility: keep the original preview cache prefix so the shared helper reuses
	// images already downloaded by the previous preview-only implementation instead of forcing one
	// extra remote fetch after the code moved into the shared image overlay package.
	return filepath.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("preview_overlay_url_%s%s", util.Md5([]byte(imageURL)), ext))
}

func writeURLImageOverlayCache(cachePath string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return err
	}
	// Bug fix: multiple clicks on the same markdown image can prepare the same URL concurrently.
	// A unique temp file keeps those writers independent while the final rename still publishes a
	// complete cache file atomically for native file-backed overlay loading.
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

func imageToPNG(img image.Image) ([]byte, error) {
	if img == nil {
		return nil, nil
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
