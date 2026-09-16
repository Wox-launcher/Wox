//go:build wox_ui_smoke && windows

package hotkey

import (
	"runtime"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/lxn/win"
)

var fullscreenTarget win.HWND

func requireFullscreenHotkeyRuntime(t *testing.T) { t.Helper() }

func fullscreenHotkeyTargetFocused() bool {
	return win.GetForegroundWindow() == fullscreenTarget
}

// newFullscreenHotkeyTarget owns a native window in the test process, separate
// from Wox. Its message loop keeps focus/resize messages responsive during RPCs.
func newFullscreenHotkeyTarget(t *testing.T) func(bool) {
	t.Helper()
	ready := make(chan win.HWND, 1)
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(done)
		user32 := syscall.NewLazyDLL("user32.dll")
		setDPI := user32.NewProc("SetThreadDpiAwarenessContext")
		previous, _, _ := setDPI.Call(^uintptr(3))
		defer setDPI.Call(previous)
		isWindow := user32.NewProc("IsWindow")
		class, _ := syscall.UTF16PtrFromString("STATIC")
		title, _ := syscall.UTF16PtrFromString("Wox fullscreen hotkey smoke")
		hwnd := win.CreateWindowEx(0, class, title, win.WS_OVERLAPPEDWINDOW, 100, 100, 640, 480, 0, 0, 0, nil)
		ready <- hwnd
		if hwnd == 0 {
			return
		}
		var message win.MSG
		for win.GetMessage(&message, 0, 0, 0) > 0 {
			win.TranslateMessage(&message)
			win.DispatchMessage(&message)
			if exists, _, _ := isWindow.Call(uintptr(hwnd)); exists == 0 {
				return
			}
		}
	}()
	hwnd := <-ready
	if hwnd == 0 {
		t.Fatal("create native fullscreen target")
	}
	fullscreenTarget = hwnd
	t.Cleanup(func() {
		win.PostMessage(hwnd, win.WM_CLOSE, 0, 0)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("native fullscreen target did not close")
		}
		fullscreenTarget = 0
	})
	return func(fullscreen bool) {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		setDPI := syscall.NewLazyDLL("user32.dll").NewProc("SetThreadDpiAwarenessContext")
		previous, _, _ := setDPI.Call(^uintptr(3))
		defer setDPI.Call(previous)
		// Both client placement and monitor bounds use physical desktop pixels.
		monitor := win.MONITORINFO{}
		monitor.CbSize = uint32(unsafe.Sizeof(monitor))
		if !win.GetMonitorInfo(win.MonitorFromWindow(hwnd, win.MONITOR_DEFAULTTONEAREST), &monitor) {
			t.Fatal("read target monitor")
		}
		rect := monitor.RcMonitor
		style := uint32(win.WS_POPUP | win.WS_VISIBLE)
		if !fullscreen {
			style = win.WS_OVERLAPPEDWINDOW | win.WS_VISIBLE
			rect.Right = rect.Left + 640
			rect.Bottom = rect.Top + 480
		}
		win.SetWindowLong(hwnd, win.GWL_STYLE, int32(style))
		if !win.SetWindowPos(hwnd, win.HWND_TOP, rect.Left, rect.Top, rect.Right-rect.Left, rect.Bottom-rect.Top, win.SWP_FRAMECHANGED|win.SWP_SHOWWINDOW) {
			t.Fatal("resize fullscreen target")
		}
		win.SetForegroundWindow(hwnd)
	}
}
