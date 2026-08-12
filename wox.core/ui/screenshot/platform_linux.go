//go:build linux

package screenshot

/*
#include <stdlib.h>
#include "../runtime/native_linux.h"
int32_t wox_screenshot_cursor_position(float *x, float *y);
int32_t wox_screenshot_set_cursor_position(int32_t x, int32_t y);
*/
import "C"

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"time"
	"unsafe"
)

var errLinuxPortalCaptureRequired = errors.New("Wayland requires Portal desktop capture")

func captureScreenshotPlatform(options ScreenshotOptions) (ScreenshotResult, error) {
	if options.ExportFilePath == "" {
		return ScreenshotResult{}, errors.New("screenshot export file path is empty")
	}
	// Give the compositor one frame to remove the launcher before copying desktop pixels.
	time.Sleep(80 * time.Millisecond)
	var cursorX, cursorY C.float
	hasCapturedCursor := C.wox_screenshot_cursor_position(&cursorX, &cursorY) == 0
	source, bounds, err := captureLinuxX11Desktop()
	var portalCapture *linuxPortalDesktopCapture
	if errors.Is(err, errLinuxPortalCaptureRequired) {
		portalCapture, err = newLinuxPortalDesktopCapture()
		if err == nil {
			defer portalCapture.close()
			bounds = portalCapture.bounds
			source, err = portalCapture.capture()
		}
	}
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
			if portalCapture != nil {
				captured, captureErr := portalCapture.capture()
				return screenshotDesktopCapture{source: captured}, captureErr
			}
			captured, _, captureErr := captureLinuxX11Desktop()
			return screenshotDesktopCapture{source: captured}, captureErr
		},
		desktopPixelOrigin: screenshotEditorDesktopPixelOrigin(bounds, source),
		setPointerPosition: func(point Point) error {
			x := int32(math.Round(float64(bounds.X + point.X)))
			y := int32(math.Round(float64(bounds.Y + point.Y)))
			if C.wox_screenshot_set_cursor_position(C.int32_t(x), C.int32_t(y)) != 0 {
				return errors.New("failed to set Linux screenshot cursor position")
			}
			return nil
		},
	}
	if hasCapturedCursor {
		platform.cursorPixel = screenshotEditorCursorPixelFromDesktop(Point{X: float32(cursorX), Y: float32(cursorY)}, bounds, source)
	}
	return runScreenshotEditor(options, source, platform)
}

// captureLinuxX11Desktop captures the root window or reports that Wayland needs the Portal path.
func captureLinuxX11Desktop() (image.Image, Rect, error) {
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
		return nil, Rect{}, errLinuxPortalCaptureRequired
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
