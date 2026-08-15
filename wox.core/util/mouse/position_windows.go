//go:build windows

package mouse

import (
	"syscall"
	"unsafe"
)

const (
	monitorDefaultToNearest = 2
	mdtEffectiveDPI         = 0
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	shcore               = syscall.NewLazyDLL("shcore.dll")
	procGetCursorPos     = user32.NewProc("GetCursorPos")
	procMonitorFromPoint = user32.NewProc("MonitorFromPoint")
	procGetDpiForMonitor = shcore.NewProc("GetDpiForMonitor")
	procGetDpiForSystem  = user32.NewProc("GetDpiForSystem")
)

type windowsPoint struct {
	X int32
	Y int32
}

// CurrentPosition returns the pointer position in the DIP-like coordinates used
// by overlay absolute positioning. Scale from the monitor under the cursor so
// mixed-DPI desktops match window.Bounds rather than GetDpiForSystem.
func CurrentPosition() (Point, bool) {
	var point windowsPoint
	result, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&point)))
	if result == 0 {
		return Point{}, false
	}
	return logicalDesktopPoint(point.X, point.Y, dpiScaleAtPhysicalPoint(point.X, point.Y)), true
}

func dpiScaleAtPhysicalPoint(x, y int32) float64 {
	packed := uintptr(uint32(x)) | uintptr(uint32(y))<<32
	monitor, _, _ := procMonitorFromPoint.Call(packed, monitorDefaultToNearest)
	if monitor != 0 && procGetDpiForMonitor.Find() == nil {
		var dpiX uint32
		var dpiY uint32
		status, _, _ := procGetDpiForMonitor.Call(monitor, mdtEffectiveDPI, uintptr(unsafe.Pointer(&dpiX)), uintptr(unsafe.Pointer(&dpiY)))
		if status == 0 && dpiX > 0 {
			return float64(dpiX) / 96
		}
	}
	return float64(systemDPI()) / 96
}

func systemDPI() uint32 {
	result, _, _ := procGetDpiForSystem.Call()
	if result == 0 {
		return 96
	}
	return uint32(result)
}
