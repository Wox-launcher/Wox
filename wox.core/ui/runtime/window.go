package woxui

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

const (
	defaultWindowWidth  = 760
	defaultWindowHeight = 480
	defaultWindowTitle  = "Wox Go UI"
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

// FrameInfo describes both the logical layout space and its backing surface.
type FrameInfo struct {
	Size      Size
	PixelSize PixelSize
	Scale     float32
}

// FocusEvent reports whether this window's focus domain owns keyboard input.
// Moving focus between child or owned native surfaces in the same domain does not emit a blur.
type FocusEvent struct {
	Epoch  FocusEpoch
	Active bool
}

// WindowRole controls whether a window behaves like a transient utility surface or a normal application window.
type WindowRole uint8

const (
	WindowRoleUtility WindowRole = iota
	WindowRoleApplication
)

// WindowOptions configures a launcher window using platform-neutral units and behavior.
// Size is the preferred initial logical client size; FrameInfo reports the actual drawable size.
type WindowOptions struct {
	Title            string
	Size             Size
	Role             WindowRole
	HideOnBlur       bool
	OnFrame          func(displayList *DisplayList, frame FrameInfo)
	OnFocus          func(event FocusEvent)
	OnKey            func(event KeyEvent) bool
	OnTextInput      func(event TextInputEvent)
	OnPointer        func(event PointerEvent)
	OnCloseRequested func()
	OnClosed         func()
}

// Window wraps the native implementation selected for the current platform.
type Window struct {
	native *platformWindow
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

	native, err := openPlatformWindow(options)
	if err != nil {
		return nil, err
	}
	return &Window{native: native}, nil
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

// StartDragging hands the active primary-pointer gesture to the native window manager.
func (w *Window) StartDragging() error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.startDragging()
}

// SetHideOnBlur changes whether the current window hides after leaving its focus domain.
func (w *Window) SetHideOnBlur(enabled bool) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.setHideOnBlur(enabled)
}

// SetFontFamily changes the window-wide UI font while preserving platform fallback when family is empty or unavailable.
func (w *Window) SetFontFamily(family string) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.setFontFamily(strings.TrimSpace(family))
}

// Invalidate requests another frame without starting a continuous render loop.
func (w *Window) Invalidate() error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.invalidate()
}

// SetTextInputState enables or disables IME delivery and positions native candidate UI.
func (w *Window) SetTextInputState(state TextInputState) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.setTextInputState(state)
}

// MeasureText measures one line using the same system font as DrawText.
// It must be called from Run's start callback or a UI callback.
func (w *Window) MeasureText(text string, style TextStyle) (TextMetrics, error) {
	if w == nil || w.native == nil {
		return TextMetrics{}, errors.New("window is not initialized")
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
	return w.native.measureText(text, style)
}

// PickFile opens the platform file picker owned by this window.
// An empty path with no error means the user cancelled the dialog.
func (w *Window) PickFile(options FileDialogOptions) (string, error) {
	if w == nil || w.native == nil {
		return "", errors.New("window is not initialized")
	}
	return w.native.pickFile(options)
}

// OpenExternalURL asks the desktop to open an HTTP or HTTPS URL in the user's default browser.
func (w *Window) OpenExternalURL(rawURL string) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("unsupported external URL %q", rawURL)
	}
	return w.native.openExternalURL(parsed.String())
}

// Close releases the native window. Run returns after the final window closes.
func (w *Window) Close() error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	clearAccessibility(w.native)
	return w.native.close()
}
