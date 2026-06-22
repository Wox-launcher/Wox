//go:build windows

package mouse

import (
	"syscall"
	"unsafe"
)

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	procGetCursorPos    = user32.NewProc("GetCursorPos")
	procGetDpiForSystem = user32.NewProc("GetDpiForSystem")
)

type windowsPoint struct {
	X int32
	Y int32
}

// CurrentPosition returns the pointer position in the DIP-like coordinates used
// by the overlay Go API. The native overlay scales absolute offsets back to
// pixels, so returning raw GetCursorPos pixels would over-shoot on scaled
// displays.
func CurrentPosition() (Point, bool) {
	var point windowsPoint
	result, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&point)))
	if result == 0 {
		return Point{}, false
	}

	scale := float64(getSystemDPI()) / 96.0
	if scale <= 0 {
		scale = 1
	}

	return Point{
		X: float64(point.X) / scale,
		Y: float64(point.Y) / scale,
	}, true
}

func getSystemDPI() uint32 {
	result, _, _ := procGetDpiForSystem.Call()
	if result == 0 {
		return 96
	}
	return uint32(result)
}
