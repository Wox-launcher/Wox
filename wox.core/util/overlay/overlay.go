package overlay

// Rect represents the position and size of a window or UI element.
type Rect struct {
	X      int
	Y      int
	Width  int
	Height int
}

// OverlayClickCallback is the callback function type for overlay clicks.
type OverlayClickCallback func()

// ShowExplorerHint displays an explorer hint overlay window.
// windowFrame specifies the position and size of the parent window.
// message is the hint text to display.
// iconData is the icon image data.
// onClick is the callback invoked when the overlay is clicked.
func ShowExplorerHint(windowFrame Rect, message string, iconData []byte, onClick OverlayClickCallback) {
	// Platform-specific implementation in overlay_*.go
}

// HideExplorerHint hides the currently displayed explorer hint overlay.
func HideExplorerHint() {
	// Platform-specific implementation in overlay_*.go
}

// StartAppActivationListener starts listening for app activation events.
// onFinderActivated is called with the window frame when the app is activated.
func StartAppActivationListener(onFinderActivated func(windowFrame Rect)) {
	// Platform-specific implementation in overlay_*.go
}

// StopAppActivationListener stops listening for app activation events.
func StopAppActivationListener() {
	// Platform-specific implementation in overlay_*.go
}
