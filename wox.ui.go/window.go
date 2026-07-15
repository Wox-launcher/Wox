package woxui

import "errors"

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

// WindowOptions configures a launcher window using platform-neutral units and behavior.
// Size is the preferred initial logical client size; FrameInfo reports the actual drawable size.
type WindowOptions struct {
	Title      string
	Size       Size
	HideOnBlur bool
	OnFrame    func(displayList *DisplayList, frame FrameInfo)
	OnFocus    func(event FocusEvent)
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

// Invalidate requests another frame without starting a continuous render loop.
func (w *Window) Invalidate() error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.invalidate()
}

// Close releases the native window. Run returns after the final window closes.
func (w *Window) Close() error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.close()
}
