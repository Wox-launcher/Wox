//go:build darwin

package screenshot

/*
#cgo CFLAGS: -fblocks -Wno-deprecated-declarations
#cgo LDFLAGS: -framework Cocoa -framework QuartzCore -framework CoreText -framework CoreGraphics -framework CoreVideo -framework IOSurface -framework WebKit
#include <stdlib.h>
#include "../runtime/native_darwin.h"
int32_t wox_screenshot_cursor_position(float *x, float *y);
int32_t wox_screenshot_set_cursor_position(float x, float y);
int32_t wox_screenshot_cursor_png(const char *path, float *hotspot_x, float *hotspot_y);
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"sync"
	"time"
	"unsafe"

	"wox/util"
	"wox/util/clipboard"
)

func captureScreenshotPlatform(options ScreenshotOptions) (ScreenshotResult, error) {
	// Give WindowServer one frame to remove the launcher before the native selector caches display pixels.
	time.Sleep(80 * time.Millisecond)
	var cursorX, cursorY C.float
	hasCapturedCursor := C.wox_screenshot_cursor_position(&cursorX, &cursorY) == 0
	capturedCursor := captureDarwinCursor()
	source, sessionHandle, displayID, bounds, selection, copiedColor, cancelled, err := selectDarwinScreenshotRegion()
	if err != nil {
		return ScreenshotResult{}, err
	}
	if copiedColor != "" {
		if err := clipboard.WriteText(copiedColor); err != nil {
			util.GetLogger().Warn(context.Background(), fmt.Sprintf("failed to copy screenshot color: %s", err.Error()))
		}
		return ScreenshotResult{CopiedColor: copiedColor}, nil
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
		captureDesktop: func() (screenshotDesktopCapture, error) {
			captured, captureErr := captureDarwinDisplay(displayID)
			return screenshotDesktopCapture{source: captured}, captureErr
		},
		desktopPixelOrigin: screenshotEditorDesktopPixelOrigin(bounds, source),
		setPointerPosition: func(point Point) error {
			if C.wox_screenshot_set_cursor_position(C.float(bounds.X+point.X), C.float(bounds.Y+point.Y)) != 0 {
				return errors.New("failed to set macOS screenshot cursor position")
			}
			return nil
		},
		cursorPosition: func() *Point {
			var x, y C.float
			if C.wox_screenshot_cursor_position(&x, &y) != 0 {
				return nil
			}
			return &Point{X: float32(x) - bounds.X, Y: float32(y) - bounds.Y}
		},
		setRecordingBounds: func(window *Window, selection Rect, _ Size, margin float32) error {
			return window.SetBounds(Rect{
				X: bounds.X + selection.X - margin, Y: bounds.Y + selection.Y - margin,
				Width: selection.Width + margin*2, Height: selection.Height + margin*2,
			})
		},
		showScrollBorder: func(selection Rect, _ Size) (func(), error) {
			handle := C.wox_darwin_show_screenshot_border(
				C.float(bounds.X+selection.X),
				C.float(bounds.Y+selection.Y),
				C.float(selection.Width),
				C.float(selection.Height),
				C.float(2),
			)
			if handle == 0 {
				return nil, errors.New("failed to show macOS screenshot border")
			}
			var closeOnce sync.Once
			return func() {
				closeOnce.Do(func() {
					C.wox_darwin_dismiss_screenshot_border(C.uintptr_t(handle))
				})
			}, nil
		},
		frameSize:        Size{Width: bounds.Width, Height: bounds.Height},
		initialSelection: &selection,
		afterShow:        dismissSelection,
	}
	if hasCapturedCursor {
		platform.cursorPixel = screenshotEditorCursorPixelFromDesktop(Point{X: float32(cursorX), Y: float32(cursorY)}, bounds, source)
		platform.capturedCursor = capturedCursor
	}
	return runScreenshotEditor(options, source, platform)
}

// captureDarwinCursor snapshots the current AppKit cursor at its native representation scale.
func captureDarwinCursor() *screenshotEditorCapturedCursor {
	file, err := os.CreateTemp("", "wox-screenshot-cursor-*.png")
	if err != nil {
		return nil
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return nil
	}
	defer os.Remove(path)
	nativePath := C.CString(path)
	defer C.free(unsafe.Pointer(nativePath))
	var hotspotX, hotspotY C.float
	if C.wox_screenshot_cursor_png(nativePath, &hotspotX, &hotspotY) != 0 {
		return nil
	}
	cursorFile, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer cursorFile.Close()
	decoded, err := png.Decode(cursorFile)
	if err != nil {
		return nil
	}
	raster := image.NewRGBA(image.Rect(0, 0, decoded.Bounds().Dx(), decoded.Bounds().Dy()))
	draw.Draw(raster, raster.Bounds(), decoded, decoded.Bounds().Min, draw.Src)
	preview, err := NewImage(raster)
	if err != nil {
		return nil
	}
	return &screenshotEditorCapturedCursor{
		raster: raster, preview: preview, hotspot: Point{X: float32(hotspotX), Y: float32(hotspotY)},
	}
}

// selectDarwinScreenshotRegion keeps one cached native image and overlay window per display until
// the user finishes selecting or copies a color from the pre-selection inspector.
func selectDarwinScreenshotRegion() (image.Image, uintptr, uint32, Rect, Rect, string, bool, error) {
	file, err := os.CreateTemp("", "wox-screenshot-*.png")
	if err != nil {
		return nil, 0, 0, Rect{}, Rect{}, "", false, fmt.Errorf("create screenshot capture file: %w", err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		return nil, 0, 0, Rect{}, Rect{}, "", false, fmt.Errorf("close screenshot capture file: %w", err)
	}
	defer os.Remove(path)

	nativePath := C.CString(path)
	defer C.free(unsafe.Pointer(nativePath))
	var sessionHandle C.uintptr_t
	var displayID C.uint32_t
	var displayX, displayY, displayWidth, displayHeight C.float
	var selectionX, selectionY, selectionWidth, selectionHeight C.float
	var copiedColor *C.char
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
		&copiedColor,
	); result {
	case 0:
	case 1:
		return nil, 0, 0, Rect{}, Rect{}, "", true, nil
	case 2:
		value := C.GoString(copiedColor)
		if copiedColor != nil {
			C.free(unsafe.Pointer(copiedColor))
		}
		return nil, 0, 0, Rect{}, Rect{}, value, false, nil
	case -2:
		return nil, 0, 0, Rect{}, Rect{}, "", false, errors.New("screen recording permission is required to capture screenshots")
	default:
		return nil, 0, 0, Rect{}, Rect{}, "", false, errors.New("failed to start the macOS screenshot selector")
	}

	source, err := decodeDarwinScreenshot(path)
	if err != nil {
		C.wox_darwin_dismiss_screenshot_selection(sessionHandle)
		return nil, 0, 0, Rect{}, Rect{}, "", false, err
	}
	return source,
		uintptr(sessionHandle),
		uint32(displayID),
		Rect{X: float32(displayX), Y: float32(displayY), Width: float32(displayWidth), Height: float32(displayHeight)},
		Rect{X: float32(selectionX), Y: float32(selectionY), Width: float32(selectionWidth), Height: float32(selectionHeight)},
		"",
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

// darwinScreenshotPixelAtPoint exposes the native selector's capture mapping for tests.
func darwinScreenshotPixelAtPoint(imageWidth, imageHeight int, frame Size, point Point) (int, int, bool) {
	var pixelX, pixelY C.int32_t
	if C.wox_darwin_test_screenshot_pixel_at_point(
		C.int32_t(imageWidth),
		C.int32_t(imageHeight),
		C.float(frame.Width),
		C.float(frame.Height),
		C.float(point.X),
		C.float(point.Y),
		&pixelX,
		&pixelY,
	) != 0 {
		return 0, 0, false
	}
	return int(pixelX), int(pixelY), true
}

// darwinScreenshotInspectorRect exposes the native selector's inspector placement for tests.
func darwinScreenshotInspectorRect(frame Size, pointer Point, panel Size, uiScale float32) Rect {
	var x, y, width, height C.float
	if C.wox_darwin_test_screenshot_inspector_rect(
		C.float(frame.Width),
		C.float(frame.Height),
		C.float(pointer.X),
		C.float(pointer.Y),
		C.float(panel.Width),
		C.float(panel.Height),
		C.float(uiScale),
		&x,
		&y,
		&width,
		&height,
	) != 0 {
		return Rect{}
	}
	return Rect{X: float32(x), Y: float32(y), Width: float32(width), Height: float32(height)}
}

// darwinScreenshotColorShortcut reports whether a macOS key code copies RGB or HEX like Windows.
func darwinScreenshotColorShortcut(keyCode uint16) (asHex bool, ok bool) {
	var hex C.int32_t
	if C.wox_darwin_test_screenshot_color_shortcut(C.uint16_t(keyCode), &hex) != 0 {
		return false, false
	}
	return hex != 0, true
}
