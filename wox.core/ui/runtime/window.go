package woxui

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"wox/util/screen"
)

const (
	defaultWindowWidth  = 760
	defaultWindowHeight = 480
	defaultWindowTitle  = "Wox Go UI"
)

var errWindowClosed = errors.New("woxui: window is closed")

// windowLifecycle tracks the portable Close/OnClosed state machine for Window.
type windowLifecycle uint32

const (
	windowLifecycleOpen windowLifecycle = iota
	windowLifecycleClosing
	windowLifecycleClosed
)

// FocusEpoch identifies one show/focus lifetime of a window.
type FocusEpoch uint64

// Color stores a straight-alpha sRGB color.
type Color struct {
	R uint8
	G uint8
	B uint8
	A uint8
}

// Size describes an area in logical pixels.
type Size struct {
	Width  float32
	Height float32
}

// PixelSize describes a drawable surface in physical pixels.
type PixelSize struct {
	Width  int
	Height int
}

// Rect describes a drawing region in logical pixels, with a top-left origin.
type Rect struct {
	X      float32
	Y      float32
	Width  float32
	Height float32
}

// Point describes a position or delta in logical pixels.
type Point struct {
	X float32
	Y float32
}

// FileDialogOptions configures a single-selection native file dialog.
type FileDialogOptions struct {
	Directory bool
}

// SaveFileOptions configures a platform-native save dialog with overwrite confirmation.
type SaveFileOptions struct {
	Title           string
	DefaultFileName string
	Extension       string
}

// FrameInfo describes both the logical layout space and its backing surface.
type FrameInfo struct {
	Size      Size
	PixelSize PixelSize
	Scale     float32
	// Damage is the logical region requiring redraw; the zero value means the complete frame.
	Damage Rect
	// WindowFocused is populated by the widget Host from the latest native focus event.
	WindowFocused bool
}

// FocusEvent reports whether this window's focus domain owns keyboard input.
// Moving focus between child or owned native surfaces in the same domain does not emit a blur.
type FocusEvent struct {
	Epoch  FocusEpoch
	Active bool
}

// WindowRole selects the native chrome and coordinate space for a window.
type WindowRole uint8

const (
	WindowRoleUtility WindowRole = iota
	WindowRoleApplication
	WindowRoleScreenshot
)

// FileDragStatus reports how a native file drag ended.
type FileDragStatus uint8

const (
	FileDragStatusSuccess FileDragStatus = iota
	FileDragStatusCancel
	FileDragStatusCancelInSource
	FileDragStatusPending
)

// WindowOptions configures a launcher window using platform-neutral units and behavior.
// Size is the preferred initial logical client size; FrameInfo reports the actual drawable size.
type WindowOptions struct {
	Title string
	Size  Size
	Role  WindowRole
	// Resizable enables native edge resizing for frameless windows.
	Resizable bool
	// AspectRatio constrains native resizing to width/height when greater than zero.
	AspectRatio float32
	// Nonactivating keeps tooltips and recording chrome visible without stealing
	// focus. It is a focus policy only; native window materials still apply.
	Nonactivating bool
	// TransientOverlay selects the platform material used by shared overlay windows.
	TransientOverlay bool
	// Topmost raises a utility window above the launcher so preview overlays
	// cannot open behind Wox when both share the floating window band.
	Topmost                    bool
	HideOnBlur                 bool
	OnFrame                    func(displayList *DisplayList, frame FrameInfo)
	OnFocus                    func(event FocusEvent)
	OnKey                      func(event KeyEvent) bool
	OnTextInput                func(event TextInputEvent)
	OnPointer                  func(event PointerEvent)
	OnFileDrop                 func(paths []string)
	OnFileDragEnded            func(status FileDragStatus)
	OnWebViewHideRequested     func()
	OnWebViewTooltip           func(event WebViewTooltipEvent)
	OnWebViewNavigationChanged func(state WebViewNavigationState)
	// OnStickyWindowChanged receives the Windows hook notification used by overlays
	// that follow a foreign native window. Other platforms ignore it.
	OnStickyWindowChanged func(target uintptr)
	OnCloseRequested      func()
	OnClosed              func()
	frameMetrics          *frameMetricsRecorder
}

// Window wraps the native implementation selected for the current platform.
type Window struct {
	native     *platformWindow
	metrics    *frameMetricsRecorder
	fontFamily atomic.Value // string; last successfully applied SetFontFamily value
	lifecycle  atomic.Uint32
	// measureTextFn overrides the native measurer for tests; production leaves it nil.
	measureTextFn func(text string, style TextStyle) (TextMetrics, error)
	// closeFn overrides native close for tests so failure/rollback can be exercised without a platform window.
	closeFn func() error
	// userOnClosed is the caller-supplied OnClosed, invoked after the window reaches closed.
	userOnClosed func()
}

// Open creates a hidden window. It must be called from Run's start callback or a UI callback.
func Open(options WindowOptions) (*Window, error) {
	if options.Title == "" {
		options.Title = defaultWindowTitle
	}
	if options.Size.Width <= 0 {
		options.Size.Width = defaultWindowWidth
	}
	if options.Size.Height <= 0 {
		options.Size.Height = defaultWindowHeight
	}

	window := &Window{userOnClosed: options.OnClosed, metrics: newFrameMetricsRecorder()}
	window.lifecycle.Store(uint32(windowLifecycleOpen))
	options.frameMetrics = window.metrics
	// Bind closed state to the real native teardown callback, including external destroys.
	options.OnClosed = window.handleNativeClosed
	native, err := openPlatformWindow(options)
	if err != nil {
		return nil, err
	}
	window.native = native
	return window, nil
}

// FrameMetrics returns a detached snapshot of timings collected since the last reset.
func (w *Window) FrameMetrics() FrameMetricsSnapshot {
	if w == nil {
		return FrameMetricsSnapshot{}
	}
	return w.metrics.current()
}

// ResetFrameMetrics starts a fresh measurement interval without disturbing in-flight rendering.
func (w *Window) ResetFrameMetrics() {
	if w == nil {
		return
	}
	w.metrics.reset()
}

// RecordFramePhase attaches a portable Host phase to the native frame that owns the display list.
func (w *Window) RecordFramePhase(frameID uint64, phase FrameMetricPhase, duration time.Duration) {
	if w == nil {
		return
	}
	w.metrics.recordPhase(frameID, phase, duration)
}

// RecordFrameCounts stores retained tree, display-list sizes, and pre-diagnostic logical damage for one frame.
func (w *Window) RecordFrameCounts(frameID uint64, nodes, commands, accessibilityNodes int, logicalDamage Rect) {
	if w == nil {
		return
	}
	w.metrics.recordCounts(frameID, nodes, commands, accessibilityNodes, logicalDamage)
}

// RecordFrameWork stores portable Host visit and reuse counts for one frame.
func (w *Window) RecordFrameWork(frameID uint64, work FrameWorkMetrics) {
	if w == nil {
		return
	}
	w.metrics.recordWork(frameID, work)
}

// RecordFrameRendererResources stores native encode-time resource counters for one frame.
func (w *Window) RecordFrameRendererResources(frameID uint64, resources FrameRendererResourceMetrics) {
	if w == nil {
		return
	}
	w.metrics.recordRendererResources(frameID, resources)
}

// Show begins a new focus lifetime and requests platform activation.
// A later FocusEvent with Active set confirms that the platform granted the request.
func (w *Window) Show() (FocusEpoch, error) {
	if w == nil || w.native == nil {
		return 0, errors.New("window is not initialized")
	}
	return w.native.show()
}

// Hide ends the current focus lifetime.
func (w *Window) Hide() error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.hide()
}

// SetBounds moves and resizes the window in logical virtual-desktop coordinates.
func (w *Window) SetBounds(bounds Rect) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return errors.New("window bounds must have a positive size")
	}
	return w.native.setBounds(bounds)
}

// Bounds returns the current window rectangle in logical virtual-desktop coordinates.
func (w *Window) Bounds() (Rect, error) {
	if w == nil || w.native == nil {
		return Rect{}, errors.New("window is not initialized")
	}
	return w.native.bounds()
}

// CapturePNG writes the current native window pixels for visual automation.
func (w *Window) CapturePNG(path string) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	path = strings.TrimSpace(path)
	if path == "" || !filepath.IsAbs(path) {
		return errors.New("window capture path must be absolute")
	}
	return w.native.capturePNG(path)
}

// Center resizes the window and centers it in the current display work area.
// Native backends clamp oversized requests so management windows remain reachable.
func (w *Window) Center(size Size) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	if size.Width <= 0 || size.Height <= 0 {
		return errors.New("window size must be positive")
	}
	return w.native.center(size)
}

// CenterOnMouseScreen resizes and centers the window in the work area under the pointer.
func (w *Window) CenterOnMouseScreen(size Size) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	if size.Width <= 0 || size.Height <= 0 {
		return errors.New("window size must be positive")
	}

	mouseScreen := screen.GetMouseScreen()
	if mouseScreen.Width <= 0 || mouseScreen.Height <= 0 {
		return w.Center(size)
	}

	width := min(size.Width, float32(mouseScreen.Width))
	height := min(size.Height, float32(mouseScreen.Height))
	return w.SetBounds(Rect{
		X:      float32(mouseScreen.X) + (float32(mouseScreen.Width)-width)/2,
		Y:      float32(mouseScreen.Y) + (float32(mouseScreen.Height)-height)/2,
		Width:  width,
		Height: height,
	})
}

// StartDragging hands the active primary-pointer gesture to the native window manager.
func (w *Window) StartDragging() error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.startDragging()
}

// StartFileDrag exports existing files through the platform's native drag session.
func (w *Window) StartFileDrag(paths []string) (FileDragStatus, error) {
	if w == nil || w.native == nil {
		return FileDragStatusCancel, errors.New("window is not initialized")
	}
	return w.native.startFileDrag(paths)
}

// Minimize sends the window to the platform taskbar or dock.
func (w *Window) Minimize() error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.minimize()
}

// SetHideOnBlur changes whether the current window hides after leaving its focus domain.
func (w *Window) SetHideOnBlur(enabled bool) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.setHideOnBlur(enabled)
}

// FocusReadyForBlur reports whether a newly shown window can accept an external focus transition.
func (w *Window) FocusReadyForBlur() bool {
	return w != nil && w.native != nil && w.native.focusReadyForBlur()
}

// SetAppearance updates native window materials to match the active light or dark theme.
func (w *Window) SetAppearance(isDark bool) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.setAppearance(isDark)
}

// SetFontFamily changes the window-wide UI font while preserving platform fallback when family is empty or unavailable.
func (w *Window) SetFontFamily(family string) error {
	if w == nil || (w.native == nil && w.measureTextFn == nil) {
		return errors.New("window is not initialized")
	}
	if !w.isOpen() {
		return errWindowClosed
	}
	family = strings.TrimSpace(family)
	if w.native != nil {
		if err := w.native.setFontFamily(family); err != nil {
			return err
		}
	}
	w.fontFamily.Store(family)
	return nil
}

// Invalidate requests another frame without starting a continuous render loop.
func (w *Window) Invalidate() error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.invalidate()
}

// InvalidateRect requests another frame for one logical client region.
func (w *Window) InvalidateRect(rect Rect) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	if rect.Width <= 0 || rect.Height <= 0 {
		return w.native.invalidate()
	}
	return w.native.invalidateRect(rect)
}

// DisplayListDamageCullingEnabled reports whether the native renderer can preserve every pixel outside Damage.
func (w *Window) DisplayListDamageCullingEnabled() bool {
	return w != nil && w.native != nil && w.native.displayListDamageCullingEnabled()
}

// RequestAnimationFrame coalesces one vsync-backed invalidation for the next display refresh.
func (w *Window) RequestAnimationFrame() error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.requestAnimationFrame()
}

// StopAnimationFrames releases the platform vsync callback while no animation is active.
func (w *Window) StopAnimationFrames() error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.stopAnimationFrames()
}

// DispatchPointer sends portable input directly to an independently managed raw window.
func (w *Window) DispatchPointer(event PointerEvent) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return Call(func() {
		if w.native.options.OnPointer != nil {
			w.native.options.OnPointer(event)
		}
	})
}

// DispatchKey sends portable keyboard input directly to an independently managed raw window.
func (w *Window) DispatchKey(event KeyEvent) (bool, error) {
	if w == nil || w.native == nil {
		return false, errors.New("window is not initialized")
	}
	handled := false
	err := Call(func() {
		if w.native.options.OnKey != nil {
			handled = w.native.options.OnKey(event)
		}
	})
	return handled, err
}

// SetTextInputState enables or disables IME delivery and positions native candidate UI.
func (w *Window) SetTextInputState(state TextInputState) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.setTextInputState(state)
}

// SetPointerCursor updates the native cursor for the current pointer target.
func (w *Window) SetPointerCursor(cursor PointerCursor) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.setPointerCursor(cursor)
}

// MeasureText measures one line using the same system font as DrawText.
// It must be called from Run's start callback or a UI callback.
// Results are cached by (text, style, font family) so layout hot paths avoid repeated CGO.
func (w *Window) MeasureText(text string, style TextStyle) (TextMetrics, error) {
	if w == nil || (w.native == nil && w.measureTextFn == nil) {
		return TextMetrics{}, errors.New("window is not initialized")
	}
	// Only an open window may answer from cache or native; closing matches platform measure failures.
	if !w.isOpen() {
		return TextMetrics{}, errWindowClosed
	}
	if text == "" {
		return TextMetrics{}, nil
	}
	if style.Size <= 0 {
		return TextMetrics{}, errors.New("text size must be positive")
	}
	if style.Weight != FontWeightRegular && style.Weight != FontWeightSemibold {
		style.Weight = FontWeightRegular
	}
	if style.Family != FontFamilyUI && style.Family != FontFamilyMonospace {
		style.Family = FontFamilyUI
	}
	key := textMetricsCacheKey{text: text, size: style.Size, weight: style.Weight, family: w.currentFontFamily(), kind: style.Family, italic: style.Italic}
	if metrics, ok := globalTextMetricsCache.get(key); ok {
		return metrics, nil
	}
	metrics, err := w.measureText(text, style)
	if err != nil {
		return metrics, err
	}
	globalTextMetricsCache.put(key, metrics)
	return metrics, nil
}

// measureText calls the injectable test backend or the native platform measurer.
func (w *Window) measureText(text string, style TextStyle) (TextMetrics, error) {
	if w.measureTextFn != nil {
		return w.measureTextFn(text, style)
	}
	return w.native.measureText(text, style)
}

// currentFontFamily returns the last font family applied via SetFontFamily.
func (w *Window) currentFontFamily() string {
	if w == nil {
		return ""
	}
	if value := w.fontFamily.Load(); value != nil {
		return value.(string)
	}
	return ""
}

// PickFile opens the platform file picker owned by this window.
// An empty path with no error means the user cancelled the dialog.
func (w *Window) PickFile(options FileDialogOptions) (string, error) {
	if w == nil || w.native == nil {
		return "", errors.New("window is not initialized")
	}
	return w.native.pickFile(options)
}

// SaveFile opens the platform save picker. An empty path means the user cancelled.
func (w *Window) SaveFile(options SaveFileOptions) (string, error) {
	if w == nil || w.native == nil {
		return "", errors.New("window is not initialized")
	}
	options.Title = strings.TrimSpace(options.Title)
	options.DefaultFileName = filepath.Base(strings.TrimSpace(options.DefaultFileName))
	options.Extension = strings.TrimPrefix(strings.TrimSpace(options.Extension), ".")
	return w.native.saveFile(options)
}

// SetPointerPassthrough controls whether pointer input reaches windows below this surface.
func (w *Window) SetPointerPassthrough(enabled bool) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.setPointerPassthrough(enabled)
}

// OpenExternalURL asks the desktop to open a web URL or email draft in the user's default application.
func (w *Window) OpenExternalURL(rawURL string) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	parsed, err := parseExternalURL(rawURL)
	if err != nil {
		return fmt.Errorf("unsupported external URL %q", rawURL)
	}
	return w.native.openExternalURL(parsed.String())
}

// parseExternalURL limits native URL dispatch to the schemes used by Wox-owned actions.
func parseExternalURL(rawURL string) (*url.URL, error) {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return nil, err
	}
	if (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" {
		return parsed, nil
	}
	if parsed.Scheme == "mailto" && parsed.Opaque != "" {
		return parsed, nil
	}
	return nil, errors.New("unsupported external URL")
}

func splitFileDropPayload(payload string) []string {
	parts := strings.Split(payload, "\n")
	paths := make([]string, 0, len(parts))
	for _, path := range parts {
		if path = strings.TrimSpace(path); path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}

// Close releases the native window. Run returns after the final window closes.
func (w *Window) Close() error {
	if w == nil || (w.native == nil && w.measureTextFn == nil && w.closeFn == nil) {
		return errors.New("window is not initialized")
	}
	if !w.beginClose() {
		return nil
	}
	if w.native != nil {
		clearAccessibility(w.native)
	}

	var err error
	switch {
	case w.closeFn != nil:
		err = w.closeFn()
	case w.native != nil:
		err = w.native.close()
	default:
		// Test windows without a native backend treat Close as a successful teardown.
		w.handleNativeClosed()
		return nil
	}
	if err != nil {
		// Native close failed and the window remains usable.
		w.abortClose()
		return err
	}
	// Successful native close invokes OnClosed; finishClosed is idempotent if it already ran.
	w.finishClosed()
	return nil
}

// beginClose transitions open → closing. Returns false when already closing or closed.
func (w *Window) beginClose() bool {
	return w.lifecycle.CompareAndSwap(uint32(windowLifecycleOpen), uint32(windowLifecycleClosing))
}

// abortClose rolls closing → open after a failed native close.
func (w *Window) abortClose() {
	w.lifecycle.CompareAndSwap(uint32(windowLifecycleClosing), uint32(windowLifecycleOpen))
}

// finishClosed transitions open/closing → closed once the native window is gone.
func (w *Window) finishClosed() {
	for {
		current := w.lifecycle.Load()
		if windowLifecycle(current) == windowLifecycleClosed {
			return
		}
		if w.lifecycle.CompareAndSwap(current, uint32(windowLifecycleClosed)) {
			return
		}
	}
}

// handleNativeClosed is installed as WindowOptions.OnClosed so external native destroys
// mark the wrapper closed even when Close() was never called.
func (w *Window) handleNativeClosed() {
	w.finishClosed()
	if w.userOnClosed != nil {
		w.userOnClosed()
	}
}

func (w *Window) isClosed() bool {
	return windowLifecycle(w.lifecycle.Load()) == windowLifecycleClosed
}

func (w *Window) isOpen() bool {
	return windowLifecycle(w.lifecycle.Load()) == windowLifecycleOpen
}
