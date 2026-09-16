package window

import (
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	"github.com/lxn/win"
)

// TestFullscreenWindow checks native coordinates on every attached display and
// changes caller DPI awareness to catch accidental coordinate virtualization.
func TestFullscreenWindow(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	setDPI := moduser32.NewProc("SetThreadDpiAwarenessContext")
	getDPI := moduser32.NewProc("GetThreadDpiAwarenessContext")
	previous, _, _ := setDPI.Call(^uintptr(3)) // PER_MONITOR_AWARE_V2
	defer setDPI.Call(previous)
	var monitors []win.RECT
	callback := syscall.NewCallback(func(monitor, dc, rect, param uintptr) uintptr {
		var info win.MONITORINFO
		info.CbSize = uint32(unsafe.Sizeof(info))
		if win.GetMonitorInfo(win.HMONITOR(monitor), &info) {
			monitors = append(monitors, info.RcMonitor)
		}
		return 1
	})
	moduser32.NewProc("EnumDisplayMonitors").Call(0, 0, callback, 0)
	if len(monitors) == 0 {
		t.Fatal("no monitors available")
	}
	class, _ := syscall.UTF16PtrFromString("STATIC")
	// A transparent, non-activating window exercises native geometry without
	// covering the user's desktop or moving focus away from the current app.
	hwnd, _, _ := moduser32.NewProc("CreateWindowExW").Call(0x08080080, uintptr(unsafe.Pointer(class)), 0,
		0x80000000, 0, 0, 100, 100, 0, 0, 0, 0)
	if hwnd == 0 {
		t.Fatal("could not create test window")
	}
	defer moduser32.NewProc("DestroyWindow").Call(hwnd)
	moduser32.NewProc("SetLayeredWindowAttributes").Call(hwnd, 0, 0, 2)
	for _, bounds := range monitors {
		moduser32.NewProc("SetWindowPos").Call(hwnd, 0, uintptr(bounds.Left), uintptr(bounds.Top),
			uintptr(bounds.Right-bounds.Left), uintptr(bounds.Bottom-bounds.Top), 0x54)
		for _, dpi := range []uintptr{^uintptr(0), ^uintptr(1), ^uintptr(2), ^uintptr(3)} {
			setDPI.Call(dpi)
			before, _, _ := getDPI.Call()
			if !isWindowFullscreen(hwnd) {
				t.Errorf("fullscreen not detected: monitor=%+v callerDPI=%d", bounds, dpi)
			}
			after, _, _ := getDPI.Call()
			if before != after {
				t.Error("fullscreen check changed caller DPI awareness")
			}
		}
		setDPI.Call(^uintptr(3))
		moduser32.NewProc("SetWindowPos").Call(hwnd, 0, uintptr(bounds.Left), uintptr(bounds.Top),
			uintptr(bounds.Right-bounds.Left-1), uintptr(bounds.Bottom-bounds.Top), 0x14)
		if isWindowFullscreen(hwnd) {
			t.Error("smaller window mistaken for fullscreen")
		}
	}
	// A decorated, maximized window must remain eligible for Wox activation.
	moduser32.NewProc("SetWindowLongW").Call(hwnd, ^uintptr(15), 0x10CF0000)
	moduser32.NewProc("ShowWindow").Call(hwnd, 3)
	if isWindowFullscreen(hwnd) {
		t.Error("maximized window mistaken for fullscreen")
	}
	if isWindowFullscreen(0) || isWindowFullscreen(uintptr(win.GetDesktopWindow())) {
		t.Error("no window or desktop mistaken for fullscreen")
	}
}
