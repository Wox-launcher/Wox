//go:build windows

package window

import (
	"github.com/lxn/win"
	"syscall"
	"unsafe"
)

var monitorFromRect = syscall.NewLazyDLL("user32.dll").NewProc("MonitorFromRect")

// WindowsLogicalRectFromPhysical converts a pixel rectangle using the DPI of its dominant monitor.
func WindowsLogicalRectFromPhysical(bounds Rect) Rect {
	return windowsLogicalRectAtScale(bounds, WindowsPhysicalRectScale(bounds))
}

// WindowsPhysicalRectScale returns the effective DPI scale of a physical rectangle's dominant monitor.
func WindowsPhysicalRectScale(bounds Rect) float32 {
	nativeBounds := win.RECT{
		Left:   int32(bounds.X),
		Top:    int32(bounds.Y),
		Right:  int32(bounds.X + bounds.Width),
		Bottom: int32(bounds.Y + bounds.Height),
	}
	monitorHandle, _, _ := monitorFromRect.Call(uintptr(unsafe.Pointer(&nativeBounds)), win.MONITOR_DEFAULTTONEAREST)
	return monitorScale(win.HMONITOR(monitorHandle))
}

func windowsLogicalRectAtScale(bounds Rect, scale float32) Rect {
	if scale <= 0 {
		scale = 1
	}
	return Rect{X: bounds.X / scale, Y: bounds.Y / scale, Width: bounds.Width / scale, Height: bounds.Height / scale}
}
