//go:build linux

package screenshot

func init() {
	registerLinuxWaylandCaptureBackend(linuxWaylandCaptureBackend{
		name:     "portal-fallback",
		priority: 0,
		matches:  func() bool { return true },
		open:     newLinuxFallbackDesktopCapture,
	})
}

// newLinuxFallbackDesktopCapture uses the safest Portal behavior when the compositor is unknown.
func newLinuxFallbackDesktopCapture() (linuxDesktopCapture, error) {
	return newLinuxPortalDesktopCapture(linuxPortalCaptureConfig{
		backend:        "portal-fallback",
		multiple:       false,
		cursorMode:     1,
		disableRestore: true,
	})
}
