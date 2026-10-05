//go:build windows

package woxui

import (
	"image"
	"os"
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	"github.com/lxn/win"
)

// TestWindowsWindowCaptureRetainsRoundedAlpha reads real compositor pixels from a disposable native window.
func TestWindowsWindowCaptureRetainsRoundedAlpha(t *testing.T) {
	if os.Getenv("WOX_WINDOW_CAPTURE_TEST") != "1" {
		t.Skip("requires an interactive Windows desktop")
	}
	t.Run("custom region", func(t *testing.T) { testWindowsWindowAlpha(t, true) })
	t.Run("system corners", func(t *testing.T) { testWindowsWindowAlpha(t, false) })
}

// testWindowsWindowAlpha exercises both authored window regions and DWM's system rounding.
func testWindowsWindowAlpha(t *testing.T, customRegion bool) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	class, _ := syscall.UTF16PtrFromString("STATIC")
	title, _ := syscall.UTF16PtrFromString("Wox window alpha test")
	style := uint32(win.WS_POPUP)
	if !customRegion {
		style = win.WS_OVERLAPPEDWINDOW
	}
	hwnd := win.CreateWindowEx(win.WS_EX_TOOLWINDOW, class, title, style, 100, 100, 240, 160, 0, 0, 0, nil)
	if hwnd == 0 {
		t.Fatal("create native test window")
	}
	defer win.DestroyWindow(hwnd)
	if customRegion {
		region, _, _ := cornerCreateRoundRectRgn.Call(0, 0, 241, 161, 64, 64)
		if result, _, _ := cornerSetWindowRgn.Call(uintptr(hwnd), region, 1); result == 0 {
			t.Fatal("set native round region")
		}
	} else {
		preference := uint32(2)
		if status, _, _ := dwmSetWindowAttribute.Call(uintptr(hwnd), 33, uintptr(unsafe.Pointer(&preference)), unsafe.Sizeof(preference)); int32(status) < 0 {
			t.Skip("system rounding requires Windows 11")
		}
	}
	win.ShowWindow(hwnd, win.SW_SHOWNOACTIVATE)
	win.UpdateWindow(hwnd)
	dc := win.GetDC(hwnd)
	brush, _, _ := syscall.NewLazyDLL("gdi32.dll").NewProc("CreateSolidBrush").Call(uintptr(win.RGB(220, 40, 60)))
	paintBounds := win.RECT{Right: 240, Bottom: 160}
	syscall.NewLazyDLL("user32.dll").NewProc("FillRect").Call(uintptr(dc), uintptr(unsafe.Pointer(&paintBounds)), brush)
	win.DeleteObject(win.HGDIOBJ(brush))
	win.ReleaseDC(hwnd, dc)
	FlushWindowsDesktopComposition()
	var bounds win.RECT
	win.GetWindowRect(hwnd, &bounds)
	var dwmBounds win.RECT
	getAttribute := syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmGetWindowAttribute")
	if status, _, _ := getAttribute.Call(uintptr(hwnd), 9, uintptr(unsafe.Pointer(&dwmBounds)), unsafe.Sizeof(dwmBounds)); int32(status) >= 0 {
		bounds = dwmBounds
	}
	pixels, err := CaptureWindowsWindow(uintptr(hwnd), image.Rect(int(bounds.Left), int(bounds.Top), int(bounds.Right), int(bounds.Bottom)))
	if err != nil {
		t.Fatal(err)
	}
	// DWM may retain a faint antialiased frame edge; authored regions have exactly zero coverage outside the shape.
	maxCornerAlpha := uint8(16)
	if customRegion {
		maxCornerAlpha = 0
	}
	if c := pixels.RGBAAt(0, 0); c.A > maxCornerAlpha {
		t.Fatalf("rounded corner contains desktop pixels: %+v", c)
	}
	if c := pixels.RGBAAt(pixels.Bounds().Dx()/2, pixels.Bounds().Dy()/2); c.A != 255 || c.R < 200 || c.G > 70 || c.B > 90 {
		t.Fatalf("window content or channel order changed: %+v", c)
	}
}
