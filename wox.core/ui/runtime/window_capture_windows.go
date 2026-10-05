//go:build windows

package woxui

/*
#cgo LDFLAGS: -lruntimeobject
#include "window_capture_windows.h"
*/
import "C"

import (
	"fmt"
	"image"
	"runtime"
	"unsafe"
)

// CaptureWindowsWindow reads compositor-owned pixels with alpha and excludes surrounding windows and shadows.
// Bounds are the HWND's physical DWM frame, independently of monitor DPI or desktop origin.
func CaptureWindowsWindow(hwnd uintptr, bounds image.Rectangle) (*image.RGBA, error) {
	if bounds.Empty() || bounds.Dx() > 16384 || bounds.Dy() > 16384 {
		return nil, fmt.Errorf("invalid window capture bounds: %v", bounds)
	}
	// WinRT initialization and teardown must run on the same native thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pixels := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	status := C.wox_capture_windows_window(C.uintptr_t(hwnd), C.int32_t(bounds.Dx()), C.int32_t(bounds.Dy()), (*C.uint8_t)(unsafe.Pointer(&pixels.Pix[0])))
	if status < 0 {
		return nil, fmt.Errorf("capture Windows window: HRESULT 0x%08x", uint32(status))
	}
	return pixels, nil
}
