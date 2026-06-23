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