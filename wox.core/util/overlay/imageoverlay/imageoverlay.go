package imageoverlay

import (
	"bytes"
	"context"
	"encoding/hex"
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
	"strconv"
	"strings"
	"sync"
	"time"

	"wox/common"
	"wox/i18n"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
	"wox/util/imagecache"
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

// ThemeColors carries the Wox theme colors used for image overlay chrome.
type ThemeColors struct {
	// Background is AppBackgroundColor, painted under the title bar so overlay
	// chrome matches the launcher and WebView instead of raw window vibrancy.
	Background woxui.Color
	Foreground woxui.Color
	Toolbar    woxui.Color
	Border     woxui.Color
	Separator  woxui.Color
	// Dark reports whether the theme is a dark palette, so the overlay window
	// can request the matching system appearance.
	Dark bool
}

var (
	imageOverlayThemeMu       sync.Mutex
	imageOverlayThemeProvider func() common.Theme
)

// SetThemeProvider registers the theme source used to color image overlay
// chrome, letting preview and pinned screenshot overlays follow the active
// Wox theme instead of a fixed dark palette.
func SetThemeProvider(provider func() common.Theme) {
	imageOverlayThemeMu.Lock()
	imageOverlayThemeProvider = provider
	imageOverlayThemeMu.Unlock()
	overlay.SetThemeProvider(provider)
}

func imageOverlayThemeColors() ThemeColors {
	imageOverlayThemeMu.Lock()
	provider := imageOverlayThemeProvider
	imageOverlayThemeMu.Unlock()
	fallback := ThemeColors{
		Background: woxui.Color{R: 24, G: 24, B: 26, A: 255},
		Foreground: woxui.Color{R: 245, G: 245, B: 245, A: 255},
		Toolbar:    woxui.Color{R: 245, G: 245, B: 245, A: 255},
		Border:     woxui.Color{R: 255, G: 255, B: 255, A: 30},
		Separator:  woxui.Color{R: 255, G: 255, B: 255, A: 76},
	}
	if provider == nil {
		return fallback
	}
	theme := provider()
	background := overlay.ThemeBackground(fallback.Background)
	return ThemeColors{
		Background: background,
		Foreground: parseImageOverlayCSSColor(theme.ActionItemFontColor, fallback.Foreground),
		Toolbar:    parseImageOverlayCSSColor(theme.ToolbarFontColor, fallback.Toolbar),
		Border:     parseImageOverlayCSSColor(theme.PreviewSplitLineColor, fallback.Border),
		Separator:  parseImageOverlayCSSColor(theme.PreviewSplitLineColor, fallback.Separator),
		Dark:       overlay.ColorIsDark(background),
	}
}

func imageOverlayColorIsDark(color woxui.Color) bool {
	linear := func(value uint8) float64 {
		channel := float64(value) / 255
		if channel <= 0.03928 {
			return channel / 12.92
		}
		return math.Pow((channel+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(color.R)+0.7152*linear(color.G)+0.0722*linear(color.B) < 0.5
}

func parseImageOverlayCSSColor(value string, fallback woxui.Color) woxui.Color {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "#") {
		raw := strings.TrimPrefix(value, "#")
		if len(raw) == 6 || len(raw) == 8 {
			decoded, err := hex.DecodeString(raw)
			if err == nil {
				color := woxui.Color{R: decoded[0], G: decoded[1], B: decoded[2], A: 255}
				if len(decoded) == 4 {
					color.A = decoded[3]
				}
				return color
			}
		}
	}
	if (strings.HasPrefix(value, "rgb(") || strings.HasPrefix(value, "rgba(")) && strings.HasSuffix(value, ")") {
		start := strings.IndexByte(value, '(')
		parts := strings.Split(value[start+1:len(value)-1], ",")
		if len(parts) == 3 || len(parts) == 4 {
			channels := make([]float64, len(parts))
			valid := true
			for index, part := range parts {
				channel, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
				if err != nil {
					valid = false
					break
				}
				channels[index] = channel
			}
			if valid {
				alpha := float64(255)
				if len(channels) == 4 {
					alpha = channels[3]
					if alpha <= 1 {
						alpha = math.Floor(alpha * 255)
					}
				}
				return woxui.Color{
					R: imageOverlayColorByte(channels[0]),
					G: imageOverlayColorByte(channels[1]),
					B: imageOverlayColorByte(channels[2]),
					A: imageOverlayColorByte(alpha),
				}
			}
		}
	}
	return fallback
}

func imageOverlayColorByte(value float64) uint8 {
	return uint8(math.Round(max(float64(0), min(float64(255), value))))
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
	var runtimeLogo *woxui.Image
	if runtime.GOOS != "darwin" {
		logo, err := common.WoxIcon.ToImageWithoutRemoteFetch()
		if err != nil {
			return fmt.Errorf("failed to decode Wox logo: %w", err)
		}
		runtimeLogo, err = woxui.NewImage(logo)
		if err != nil {
			return fmt.Errorf("failed to create runtime Wox logo: %w", err)
		}
	}

	window := overlay.WindowOptions{
		ID:                     opts.ID,
		Movable:                opts.Movable,
		Resizable:              true,
		LightAppearance:        !imageOverlayThemeColors().Dark,
		FollowsThemeAppearance: true,
		CornerRadius:           opts.CornerRadius,
		AspectRatio:            width / (height + imageOverlayTitleBarHeight),
		CloseOnEscape:          opts.CloseOnEscape,
		TakeFocus:              opts.CloseOnEscape,
		Topmost:                opts.Topmost,
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
		return woxwidget.Stateful{
			Key: woxwidget.Key("image-overlay-title-" + opts.ID), Type: (*imageOverlayTitleBarState)(nil),
			Widget: imageOverlayTitleBarProps{
				ID: opts.ID, Width: frame.Size.Width, Height: frame.Size.Height, Title: title,
				Platform: runtime.GOOS, Colors: imageOverlayThemeColors(), Image: runtimeImage, Logo: runtimeLogo,
				Closable: opts.Closable || opts.CloseOnEscape, Active: frame.WindowFocused,
				OnClose: func() { overlay.RequestClose(opts.ID) },
			},
			CreateState: func() woxwidget.State { return &imageOverlayTitleBarState{} },
		}
	}, OnPointer: func(event woxui.PointerEvent) {
		if event.Kind != woxui.PointerScroll || event.Position.Y < imageOverlayTitleBarHeight || event.Scroll.Y == 0 {
			return
		}
		factor := float32(math.Exp(float64(event.Scroll.Y) * imageOverlayWheelSensitivity))
		overlay.ScaleWindow(opts.ID, min(max(factor, 0.8), 1.25), imageOverlayMinWidth, imageOverlayMinHeight)
	}})
	return nil
}

// imageOverlayTitleBarProps carries the per-frame image overlay chrome state.
type imageOverlayTitleBarProps struct {
	ID       string
	Width    float32
	Height   float32
	Title    string
	Platform string
	Colors   ThemeColors
	Image    *woxui.Image
	Logo     *woxui.Image
	Closable bool
	Active   bool
	OnClose  func()
}

type imageOverlayTitleBarState struct {
	hovered string
	pressed string
}

func (s *imageOverlayTitleBarState) InitState(_ woxwidget.StateContext, _ any)          {}
func (s *imageOverlayTitleBarState) DidUpdateWidget(_ woxwidget.StateContext, _, _ any) {}
func (s *imageOverlayTitleBarState) Dispose()                                           {}

// Build keeps native close-control hover painting inside the retained state.
func (s *imageOverlayTitleBarState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(imageOverlayTitleBarProps)
	onHover := func(control string, inside bool) {
		context.SetState(func() {
			if inside {
				s.hovered = control
			} else if s.hovered == control {
				s.hovered = ""
			}
		})
	}
	onPress := func(control string, pressed bool) {
		context.SetState(func() {
			if pressed {
				s.pressed = control
			} else if s.pressed == control {
				s.pressed = ""
			}
		})
	}
	return buildImageOverlayChrome(props, s.hovered, s.pressed, onHover, onPress)
}

// buildImageOverlayChrome composes the image body and the shared platform title
// bar chrome so preview and pinned screenshot windows match the WebView chrome.
func buildImageOverlayChrome(props imageOverlayTitleBarProps, hovered, pressed string, onHover, onPress func(string, bool)) woxwidget.Widget {
	bodyHeight := max(float32(1), props.Height-imageOverlayTitleBarHeight)
	theme := woxcomponent.Theme{Background: props.Colors.Background, ToolbarText: props.Colors.Toolbar}
	backgroundTop := float32(0)
	backgroundHeight := props.Height
	if props.Platform == "windows" {
		// Leave the title bar unpainted so the native Accent Acrylic remains visible.
		backgroundTop = imageOverlayTitleBarHeight
		backgroundHeight = bodyHeight
	}
	children := []woxwidget.StackChild{
		// Fill only: the platform window owns the outer shape. A widget
		// radius/stroke stacks a second corner on top of that clip.
		{Top: backgroundTop, Child: woxwidget.Container{
			Width: props.Width, Height: backgroundHeight, Color: props.Colors.Background,
		}},
		{Top: imageOverlayTitleBarHeight, Child: woxwidget.Image{
			Source: props.Image, Width: props.Width, Height: bodyHeight, Fit: woxwidget.ImageFitContain,
		}},
		{Top: imageOverlayTitleBarHeight - 1, Child: woxwidget.Container{Width: props.Width, Height: 1, Color: props.Colors.Separator}},
	}
	titleStyle := woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}
	if props.Platform == "darwin" {
		// macOS title bars are text-only; the app icon is a Windows/Linux chrome cue.
		children = append(children, woxwidget.StackChild{Top: 8, Child: woxwidget.Align{
			Width: props.Width, Height: 24, Horizontal: 0.5, Vertical: 0.5,
			Child: woxwidget.TextBlock{Value: props.Title, MaxLines: 1, ShrinkWrap: true, Style: titleStyle, Color: props.Colors.Foreground},
		}})
	} else {
		children = append(children,
			woxwidget.StackChild{Left: 12, Top: 10, Child: woxwidget.Image{Source: props.Logo, Width: 20, Height: 20, Fit: woxwidget.ImageFitContain}},
			woxwidget.StackChild{Left: 40, Top: 8, Child: woxwidget.Align{
				Width: max(float32(0), props.Width-96), Height: 24, Horizontal: 0, Vertical: 0.5,
				Child: woxwidget.TextBlock{
					Value: props.Title, Width: max(float32(0), props.Width-96), MaxLines: 1,
					Style: titleStyle, Color: props.Colors.Foreground,
				},
			}},
		)
	}
	if props.Closable {
		switch props.Platform {
		case "darwin":
			children = append(children, woxwidget.StackChild{Left: 13, Child: woxcomponent.MacTrafficLight("image-overlay-close", woxui.Color{R: 255, G: 92, B: 95, A: 255}, "×", woxui.Color{R: 128, G: 47, B: 49, A: 255}, hovered == "mac-controls", pressed == "image-overlay-close", props.Active, theme, props.OnClose, onHover, onPress)})
		case "windows":
			children = append(children, woxwidget.StackChild{AnchorRight: true, Child: woxcomponent.WindowsTitleBarButton("image-overlay-close", "close", hovered == "close", theme, props.OnClose, onHover)})
		default:
			children = append(children, woxwidget.StackChild{AnchorRight: true, Child: woxcomponent.LinuxTitleBarCloseButton("image-overlay-close", hovered == "close", theme, props.OnClose, onHover)})
		}
	}
	return woxwidget.Stack{Width: props.Width, Height: props.Height, Children: children}
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
		if x, y, ok := imageOverlayDefaultPosition(screen.GetMouseScreen()); ok {
			// Preview overlays open from a click, so the default is the center of the
			// display under the pointer rather than the cursor or the primary screen.
			opts.AbsolutePosition = true
			opts.Anchor = overlay.AnchorCenter
			opts.OffsetX = x
			opts.OffsetY = y
		}
	}
	return opts
}

// imageOverlayDefaultPosition returns the logical center of the pointer's
// display, including negative desktop origins on a secondary monitor.
func imageOverlayDefaultPosition(mouseScreen screen.Size) (float64, float64, bool) {
	if mouseScreen.Width <= 0 || mouseScreen.Height <= 0 {
		return 0, 0, false
	}
	return float64(mouseScreen.X) + float64(mouseScreen.Width)/2, float64(mouseScreen.Y) + float64(mouseScreen.Height)/2, true
}

func showImageOverlayLoadingOverlay(ctx context.Context, opts Options) {
	// Feature change: URL image overlays acknowledge the click before network download and cache
	// preparation. Local files, screenshots, base64, and inline SVG stay direct because they do not
	// need a separate waiting state.
	start := time.Now()
	textoverlay.Show(textoverlay.Options{
		Window: overlay.WindowOptions{
			ID:               opts.ID,
			AbsolutePosition: opts.AbsolutePosition,
			Anchor:           opts.Anchor,
			OffsetX:          opts.OffsetX,
			OffsetY:          opts.OffsetY,
			Width:            200,
			Movable:          true,
			CloseOnEscape:    true,
			Topmost:          true,
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
			ID:               opts.ID,
			AbsolutePosition: opts.AbsolutePosition,
			Anchor:           opts.Anchor,
			OffsetX:          opts.OffsetX,
			OffsetY:          opts.OffsetY,
			Width:            220,
			CloseOnEscape:    true,
			Topmost:          true,
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

// fitImageOverlaySize caps preview-style overlays to the pointer's display while preserving aspect
// ratio. Pinning callers can skip this by passing explicit logical selection dimensions.
func fitImageOverlaySize(sourceWidth, sourceHeight float64) (float64, float64) {
	target := screen.GetMouseScreen()
	if target.Width <= 0 || target.Height <= 0 {
		target = screen.GetActiveScreen()
	}
	return fitImageOverlaySizeToScreen(sourceWidth, sourceHeight, target)
}

// fitImageOverlaySizeToScreen scales the overlay to at most 86% of the target
// display while keeping the source aspect ratio.
func fitImageOverlaySizeToScreen(sourceWidth, sourceHeight float64, target screen.Size) (float64, float64) {
	if sourceWidth < 1 || sourceHeight < 1 {
		return 1, 1
	}

	maxWidth := float64(target.Width) * 0.86
	maxHeight := float64(target.Height) * 0.86
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
