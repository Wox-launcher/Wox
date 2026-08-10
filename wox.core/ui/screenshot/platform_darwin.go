//go:build darwin

package screenshot

/*
#cgo CFLAGS: -fblocks -Wno-deprecated-declarations
#cgo LDFLAGS: -framework Cocoa -framework QuartzCore -framework CoreText -framework CoreGraphics -framework CoreVideo -framework IOSurface -framework WebKit
#include <stdlib.h>
#include "../runtime/native_darwin.h"
int32_t wox_screenshot_cursor_position(float *x, float *y);
*/
import "C"

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"sync"
	"time"
	"unsafe"
)

func captureScreenshotPlatform(options ScreenshotOptions) (ScreenshotResult, error) {
	if options.ExportFilePath == "" {
		return ScreenshotResult{}, errors.New("screenshot export file path is empty")
	}
	// Give WindowServer one frame to remove the launcher before the native selector caches display pixels.
	time.Sleep(80 * time.Millisecond)
	var cursorX, cursorY C.float
	hasCapturedCursor := C.wox_screenshot_cursor_position(&cursorX, &cursorY) == 0
	source, sessionHandle, displayID, bounds, selection, cancelled, err := selectDarwinScreenshotRegion()
	if err != nil {
		return ScreenshotResult{}, err
	}
	if cancelled {
		return ScreenshotResult{Cancelled: true}, nil
	}
	var dismissOnce sync.Once
	dismissSelection := func() {
		dismissOnce.Do(func() {
			C.wox_darwin_dismiss_screenshot_selection(C.uintptr_t(sessionHandle))
		})
	}
	defer dismissSelection()
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
		captureDesktop: func() (image.Image, error) {
			captured, captureErr := captureDarwinDisplay(displayID)
			return captured, captureErr
		},
		frameSize:        Size{Width: bounds.Width, Height: bounds.Height},
		initialSelection: &selection,
		afterShow:        dismissSelection,
	}
	if hasCapturedCursor {
		platform.cursorPixel = screenshotEditorCursorPixelFromDesktop(Point{X: float32(cursorX), Y: float32(cursorY)}, bounds, source)
	}
	return runScreenshotEditor(options, source, platform)
}

// selectDarwinScreenshotRegion keeps one cached native image and overlay window per display until the user finishes selecting.
func selectDarwinScreenshotRegion() (image.Image, uintptr, uint32, Rect, Rect, bool, error) {
	file, err := os.CreateTemp("", "wox-screenshot-*.png")
	if err != nil {
		return nil, 0, 0, Rect{}, Rect{}, false, fmt.Errorf("create screenshot capture file: %w", err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		return nil, 0, 0, Rect{}, Rect{}, false, fmt.Errorf("close screenshot capture file: %w", err)
	}
	defer os.Remove(path)

	nativePath := C.CString(path)
	defer C.free(unsafe.Pointer(nativePath))
	var sessionHandle C.uintptr_t
	var displayID C.uint32_t
	var displayX, displayY, displayWidth, displayHeight C.float
	var selectionX, selectionY, selectionWidth, selectionHeight C.float
	switch result := C.wox_darwin_select_screenshot_region(
		nativePath,
		&sessionHandle,
		&displayID,
		&displayX,
		&displayY,
		&displayWidth,
		&displayHeight,
		&selectionX,
		&selectionY,
		&selectionWidth,
		&selectionHeight,
	); result {
	case 0:
	case 1:
		return nil, 0, 0, Rect{}, Rect{}, true, nil
	case -2:
		return nil, 0, 0, Rect{}, Rect{}, false, errors.New("screen recording permission is required to capture screenshots")
	default:
		return nil, 0, 0, Rect{}, Rect{}, false, errors.New("failed to start the macOS screenshot selector")
	}

	source, err := decodeDarwinScreenshot(path)
	if err != nil {
		C.wox_darwin_dismiss_screenshot_selection(sessionHandle)
		return nil, 0, 0, Rect{}, Rect{}, false, err
	}
	return source,
		uintptr(sessionHandle),
		uint32(displayID),
		Rect{X: float32(displayX), Y: float32(displayY), Width: float32(displayWidth), Height: float32(displayHeight)},
		Rect{X: float32(selectionX), Y: float32(selectionY), Width: float32(selectionWidth), Height: float32(selectionHeight)},
		false,
		nil
}

// captureDarwinDisplay captures only the display that owns the active annotation editor.
func captureDarwinDisplay(displayID uint32) (image.Image, error) {
	file, err := os.CreateTemp("", "wox-screenshot-display-*.png")
	if err != nil {
		return nil, fmt.Errorf("create screenshot capture file: %w", err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close screenshot capture file: %w", err)
	}
	defer os.Remove(path)

	nativePath := C.CString(path)
	defer C.free(unsafe.Pointer(nativePath))
	if C.wox_darwin_capture_display_png(C.uint32_t(displayID), nativePath) != 0 {
		return nil, errors.New("failed to capture the selected macOS display")
	}
	return decodeDarwinScreenshot(path)
}

func decodeDarwinScreenshot(path string) (image.Image, error) {
	captured, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open captured desktop image: %w", err)
	}
	defer captured.Close()
	source, err := png.Decode(captured)
	if err != nil {
		return nil, fmt.Errorf("decode captured desktop image: %w", err)
	}
	return source, nil
}
