//go:build windows

package capture

import (
	"image"
	"testing"
)

func TestCaptureWindowsRectRejectsEmptyBounds(t *testing.T) {
	if _, err := CaptureWindowsRect(image.Rect(0, 0, 0, 0)); err == nil {
		t.Fatal("empty capture rectangle should be rejected")
	}
}

// TestDesktopRegionRejectsOffscreenBounds ensures clipping cannot allocate a DIB for an invisible window.
func TestDesktopRegionRejectsOffscreenBounds(t *testing.T) {
	desktop := windowsVirtualDesktopBounds()
	offscreen := image.Rect(desktop.Max.X+1, desktop.Min.Y, desktop.Max.X+20, desktop.Max.Y)
	if capture, err := CaptureWindowsDesktopRegion(offscreen); err == nil {
		capture.Close()
		t.Fatal("offscreen capture rectangle should be rejected")
	}
}
