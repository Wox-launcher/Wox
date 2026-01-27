//go:build linux

package overlay

// ShowExplorerHint - Not supported on Linux
func ShowExplorerHint(windowFrame Rect, message string, iconData []byte, onClick OverlayClickCallback) {
	// Not supported on Linux
}

// HideExplorerHint - Not supported on Linux
func HideExplorerHint() {
	// Not supported on Linux
}

// StartAppActivationListener - Not supported on Linux
func StartAppActivationListener(onFinderActivated func(windowFrame Rect)) {
	// Not supported on Linux
}

// StopAppActivationListener - Not supported on Linux
func StopAppActivationListener() {
	// Not supported on Linux
}
