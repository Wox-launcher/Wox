//go:build windows

package capture

/*
#cgo LDFLAGS: -lruntimeobject
#include "window_capture_windows.h"
*/
import "C"

import (
	"fmt"
	"image"
	"math"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
)

var captureDwmGetWindowAttribute = syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmGetWindowAttribute")
var captureGetWindowRgn = syscall.NewLazyDLL("user32.dll").NewProc("GetWindowRgn")

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
	clipWindowsCapturedWindowCorners(win.HWND(hwnd), pixels)
	return pixels, nil
}

// clipWindowsCapturedWindowCorners removes compositor shadow outside default or requested system rounding.
// The preference alone is insufficient: maximized/snapped windows can retain it while displaying square corners.
func clipWindowsCapturedWindowCorners(hwnd win.HWND, pixels *image.RGBA) {
	if win.GetWindowLong(hwnd, win.GWL_STYLE)&win.WS_MAXIMIZE != 0 || captureDwmGetWindowAttribute.Find() != nil {
		return
	}
	var preference uint32
	status, _, _ := captureDwmGetWindowAttribute.Call(uintptr(hwnd), dwmwaWindowCorner, uintptr(unsafe.Pointer(&preference)), unsafe.Sizeof(preference))
	if int32(status) < 0 || (preference != 0 && preference != 2 && preference != 3) {
		return
	}
	// Default rounding is also used by external apps such as Qt windows. An authored region,
	// however, has its own shape and must not be replaced with DWM's default radius.
	region := win.CreateRectRgn(0, 0, 0, 0)
	if region == 0 {
		return
	}
	regionKind, _, _ := captureGetWindowRgn.Call(uintptr(hwnd), uintptr(region))
	win.DeleteObject(win.HGDIOBJ(region))
	if regionKind != 0 {
		return
	}
	// DWM system radii are logical units, while WGC captures physical pixels at the HWND's DPI.
	radius := float64(8)
	if preference == 3 {
		radius = 4
	}
	dpi := win.GetDpiForWindow(hwnd)
	if dpi == 0 {
		return
	}
	clipWindowsCapturedCornerPixels(pixels, radius*float64(dpi)/96)
}

// clipWindowsCapturedCornerPixels intersects native alpha with the rounded shape without scaling it twice.
// Black translucent corner pixels identify shadow residue; opaque or colored corners must keep their native shape.
func clipWindowsCapturedCornerPixels(pixels *image.RGBA, radius float64) {
	if pixels == nil || pixels.Bounds().Empty() || radius <= 0 {
		return
	}
	bounds := pixels.Bounds()
	hasShadow := false
	for _, point := range [...]image.Point{bounds.Min, {X: bounds.Max.X - 1, Y: bounds.Min.Y}, {X: bounds.Min.X, Y: bounds.Max.Y - 1}, bounds.Max.Sub(image.Pt(1, 1))} {
		corner := pixels.RGBAAt(point.X, point.Y)
		if corner.A == 255 || corner.R != 0 || corner.G != 0 || corner.B != 0 {
			return
		}
		hasShadow = hasShadow || corner.A > 0
	}
	if !hasShadow {
		return
	}
	radius = min(radius, float64(min(bounds.Dx(), bounds.Dy()))/2)
	for y := 0; y < int(math.Ceil(radius)); y++ {
		for x := 0; x < int(math.Ceil(radius)); x++ {
			coverage := min(float64(1), max(float64(0), radius+0.5-math.Hypot(float64(x)+0.5-radius, float64(y)+0.5-radius)))
			alpha := uint8(coverage*255 + 0.5)
			for _, point := range [...]image.Point{{X: x, Y: y}, {X: bounds.Dx() - 1 - x, Y: y}, {X: x, Y: bounds.Dy() - 1 - y}, {X: bounds.Dx() - 1 - x, Y: bounds.Dy() - 1 - y}} {
				offset := pixels.PixOffset(bounds.Min.X+point.X, bounds.Min.Y+point.Y)
				oldAlpha := pixels.Pix[offset+3]
				if alpha >= oldAlpha {
					continue
				}
				for channel := 0; channel < 3; channel++ {
					pixels.Pix[offset+channel] = uint8(uint16(pixels.Pix[offset+channel]) * uint16(alpha) / uint16(oldAlpha))
				}
				pixels.Pix[offset+3] = alpha
			}
		}
	}
}
