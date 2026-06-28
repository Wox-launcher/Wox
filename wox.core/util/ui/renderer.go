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

// NativeRenderer is the platform-agnostic interface implemented by each
// native backend. gpuUIImpl holds a NativeRenderer so it does not need a
// platform-specific concrete type and can share the same code path across OSes.
//
// To add a new platform (e.g. Linux):
//  1. Create ui_<platform>.go (build tag: <platform> && cgo) with a renderer
//     struct embedding baseRenderer.
//  2. Create ui_<platform>.c with the platform's implementation of the
//     uiWindow* and uiMeasureText functions declared in ui_native.h.
//  3. Implement NewNativeRenderer to return the platform renderer.
//  4. Implement StartEventLoop (GTK main loop, X11 event pipe, etc.).
//  5. No changes needed to gpuUIImpl, LayoutEngine, Widget, Theme, or any
//     other shared Go-side logic — the baseRenderer and ui_native.h
//     abstraction handle all the shared plumbing.
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

	// StartEventLoop begins the platform's native event loop and registers
	// an onRender callback for frame production.
	//
	// Blocking semantics differ by platform — callers must NOT depend on
	// whether this method returns:
	//   - Windows: blocks until Close() is called (Win32 GetMessage loop)
	//   - macOS:   returns immediately (Cocoa [NSApp run] already running)
	//   - Linux:   TBD (GTK main loop or GLib frame clock)
	//
	// The onRender callback is invoked when the platform requests a frame.
	// It returns nil when the window is hidden or nothing changed; the
	// platform decides whether to present.
	StartEventLoop(onRender func() *CommandList)
}