package ui

// TextMeasurer measures text dimensions for layout calculations.
// This is a callback interface — the Go layout engine calls it,
// the native backend implements it (DirectWrite / CoreText / Xft).
type TextMeasurer interface {
	// MeasureText returns the width and height of text in DIP
	// at the given font size and family.
	MeasureText(text string, fontSize float32, fontFamily string) (width, height float32)
}

// Renderer is the native backend interface that executes draw commands.
// Each platform implements this via CGO:
//   - Windows: Direct2D + DirectWrite
//   - macOS: CoreGraphics + CoreText
//   - Linux: X11 + Xft + XRender
type Renderer interface {
	// Render executes the command list to the window surface.
	// Called from the Go side after layout is complete.
	Render(commands *CommandList) error

	// MeasureText delegates to the platform's text shaping engine.
	TextMeasurer() TextMeasurer
}

// WindowLifecycle is managed by the native backend.
// The Go side calls these to control the launcher window.
type WindowLifecycle interface {
	// Show makes the window visible and focused.
	Show() error

	// Hide hides the window (does not destroy it).
	Hide() error

	// SetPosition moves the window to absolute screen coordinates.
	SetPosition(x, y int) error

	// SetSize resizes the window.
	SetSize(w, h int) error

	// Close destroys the window and exits the message loop.
	Close() error

	// IsVisible returns whether the window is currently shown.
	IsVisible() bool
}

// NativeRenderer merges Renderer, WindowLifecycle, and the extra methods the
// gpuUIImpl needs from the platform backend. Each platform (Windows/macOS)
// implements this via CGO and asserts conformance at compile time.
// gpuUIImpl holds a NativeRenderer so it does not need a platform-specific
// concrete type and can share the same code path across OSes.
type NativeRenderer interface {
	Renderer
	WindowLifecycle

	// GetSize returns the current logical (DIP) window dimensions.
	GetSize() (int, int)

	// SetDarkMode switches the native system backdrop tone so the vibrancy /
	// Mica material matches the active theme.
	SetDarkMode(dark bool)

	// ReleaseMemory drops native caches that are only useful while the
	// launcher is visible. Called when the window is hidden.
	ReleaseMemory()

	// RequestRepaint triggers a native repaint of the window surface.
	RequestRepaint()

	// RunMessageLoop enters the native message loop. On Windows this blocks
	// until Close/WM_QUIT; on macOS it just stores onRender and returns so
	// the Cocoa event loop ([NSApp run]) continues driving rendering.
	RunMessageLoop(onRender func() *CommandList)
}