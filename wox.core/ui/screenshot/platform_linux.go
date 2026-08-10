//go:build linux

package screenshot

/*
#include <stdlib.h>
#include "../runtime/native_linux.h"
int32_t wox_screenshot_cursor_position(float *x, float *y);
*/
import "C"

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"time"
	"unsafe"
)

func captureScreenshotPlatform(options ScreenshotOptions) (ScreenshotResult, error) {
	if options.ExportFilePath == "" {
		return ScreenshotResult{}, errors.New("screenshot export file path is empty")
	}
	// Give the compositor one frame to remove the launcher before copying desktop pixels.
	time.Sleep(80 * time.Millisecond)
	var cursorX, cursorY C.float
	hasCapturedCursor := C.wox_screenshot_cursor_position(&cursorX, &cursorY) == 0
	source, bounds, err := captureLinuxDesktop()
	if err != nil {
		return ScreenshotResult{}, err
	}
	platform := screenshotEditorPlatform{
		setWindowBounds: func(window *Window) error {
			return window.SetBounds(bounds)
		},
		logicalSelection: func(selection Rect, _ Size) Rect {
			return Rect{
				X:      bounds.X + selection.X,
				Y:      bounds.Y + selection.Y,
				Width:  selection.Width,
				Height: selection.Height,
			}
		},
		captureDesktop: func() (screenshotDesktopCapture, error) {
			captured, _, captureErr := captureLinuxDesktop()
			return screenshotDesktopCapture{source: captured}, captureErr
		},
	}
	if hasCapturedCursor {
		platform.cursorPixel = screenshotEditorCursorPixelFromDesktop(Point{X: float32(cursorX), Y: float32(cursorY)}, bounds, source)
	}
	return runScreenshotEditor(options, source, platform)
}

// captureLinuxDesktop captures the X11 root window and its logical workspace bounds.
func captureLinuxDesktop() (image.Image, Rect, error) {
	file, err := os.CreateTemp("", "wox-screenshot-*.png")
	if err != nil {
		return nil, Rect{}, fmt.Errorf("create screenshot capture file: %w", err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		return nil, Rect{}, fmt.Errorf("close screenshot capture file: %w", err)
	}
	defer os.Remove(path)

	nativePath := C.CString(path)
	defer C.free(unsafe.Pointer(nativePath))
	var x, y, width, height C.float
	switch result := C.wox_linux_capture_desktop_png(nativePath, &x, &y, &width, &height); result {
	case 0:
	case -2:
		return nil, Rect{}, fmt.Errorf("Wayland desktop capture requires a portal-owned flow: %w", ErrPlatformUnsupported)
	default:
		return nil, Rect{}, errors.New("failed to capture the Linux desktop")
	}

	captured, err := os.Open(path)
	if err != nil {
		return nil, Rect{}, fmt.Errorf("open captured desktop image: %w", err)
	}
	defer captured.Close()
	source, err := png.Decode(captured)
	if err != nil {
		return nil, Rect{}, fmt.Errorf("decode captured desktop image: %w", err)
	}
	return source, Rect{X: float32(x), Y: float32(y), Width: float32(width), Height: float32(height)}, nil
}
