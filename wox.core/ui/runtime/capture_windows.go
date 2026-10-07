//go:build windows

package woxui

import (
	image "image"
	capture "wox/ui/runtime/internal/capture"
)

// WindowsDesktopCaptureTimings separates the native capture stages for diagnostics.
type WindowsDesktopCaptureTimings = capture.WindowsDesktopCaptureTimings

// WindowsDesktopCapture owns an immutable top-down BGRX desktop image and its backing DIB.
type WindowsDesktopCapture = capture.WindowsDesktopCapture

// FlushWindowsDesktopComposition waits for pending DWM updates without an arbitrary sleep.
func FlushWindowsDesktopComposition() { capture.FlushWindowsDesktopComposition() }

// CaptureWindowsVirtualDesktop captures directly into one mapped top-down BGRX DIB.
func CaptureWindowsVirtualDesktop() (*WindowsDesktopCapture, error) {
	return capture.CaptureWindowsVirtualDesktop()
}

// WindowsRectCapturer reuses one DIB so recording can copy a fixed rectangle every frame.
type WindowsRectCapturer = capture.WindowsRectCapturer

// NewWindowsRectCapturer prepares a reusable capture surface for one physical rectangle.
// DXGI desktop duplication is preferred because GDI BitBlt hides the cursor inside the captured region.
func NewWindowsRectCapturer(bounds image.Rectangle) (*WindowsRectCapturer, error) {
	return capture.NewWindowsRectCapturer(bounds)
}

// CaptureWindowsRect copies one physical pixel rectangle into an owned BGR0 image.
func CaptureWindowsRect(bounds image.Rectangle) (*image.RGBA, error) {
	return capture.CaptureWindowsRect(bounds)
}

// CaptureWindowsWindow reads compositor-owned pixels with alpha and excludes surrounding windows and shadows.
// Bounds are the HWND's physical DWM frame, independently of monitor DPI or desktop origin.
func CaptureWindowsWindow(hwnd uintptr, bounds image.Rectangle) (*image.RGBA, error) {
	return capture.CaptureWindowsWindow(hwnd, bounds)
}
