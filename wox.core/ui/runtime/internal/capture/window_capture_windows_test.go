//go:build windows

package capture

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	"github.com/lxn/win"
)

var (
	cornerCreateRoundRectRgn = syscall.NewLazyDLL("gdi32.dll").NewProc("CreateRoundRectRgn")
	cornerSetWindowRgn       = syscall.NewLazyDLL("user32.dll").NewProc("SetWindowRgn")
	dwmSetWindowAttribute    = syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute")
)

// applyCaptureTestBackdrop constructs an Acrylic fixture without importing Wox's window lifecycle.
func applyCaptureTestBackdrop(hwnd win.HWND) {
	margins := [4]int32{-1, -1, -1, -1}
	_, _, _ = syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmExtendFrameIntoClientArea").Call(uintptr(hwnd), uintptr(unsafe.Pointer(&margins)))
	corner, backdrop := int32(2), int32(3)
	_, _, _ = dwmSetWindowAttribute.Call(uintptr(hwnd), dwmwaWindowCorner, uintptr(unsafe.Pointer(&corner)), unsafe.Sizeof(corner))
	_, _, _ = dwmSetWindowAttribute.Call(uintptr(hwnd), 38, uintptr(unsafe.Pointer(&backdrop)), unsafe.Sizeof(backdrop))
	win.SendMessage(hwnd, win.WM_NCACTIVATE, 1, 0)
}

// TestWindowsCapturedCornerPixels checks shadow removal, premultiplied edges and untouched square windows.
func TestWindowsCapturedCornerPixels(t *testing.T) {
	for _, test := range []struct {
		scale float64
		edge  image.Point
	}{{1, image.Pt(2, 2)}, {1.25, image.Pt(3, 2)}, {1.5, image.Pt(3, 3)}, {2, image.Pt(4, 4)}, {2.5, image.Pt(5, 6)}} {
		t.Run(fmt.Sprint(test.scale), func(t *testing.T) {
			pixels := image.NewRGBA(image.Rect(-10, -20, 90, 60))
			content := color.RGBA{R: 80, G: 40, B: 20, A: 128}
			draw.Draw(pixels, pixels.Bounds(), image.NewUniform(content), image.Point{}, draw.Src)
			radius := 8 * test.scale
			// Native shadow is black with nonzero alpha even outside the system's rounded shape.
			for _, point := range [...]image.Point{pixels.Rect.Min, {X: pixels.Rect.Max.X - 1, Y: pixels.Rect.Min.Y}, {X: pixels.Rect.Min.X, Y: pixels.Rect.Max.Y - 1}, pixels.Rect.Max.Sub(image.Pt(1, 1))} {
				pixels.SetRGBA(point.X, point.Y, color.RGBA{A: 64})
			}
			edge := pixels.Rect.Min.Add(test.edge)
			translucent := color.RGBA{R: 4, G: 2, B: 1, A: 16}
			pixels.SetRGBA(edge.X, edge.Y, translucent)
			clipWindowsCapturedCornerPixels(pixels, radius)
			for _, point := range [...]image.Point{pixels.Rect.Min, {X: pixels.Rect.Max.X - 1, Y: pixels.Rect.Min.Y}, {X: pixels.Rect.Min.X, Y: pixels.Rect.Max.Y - 1}, pixels.Rect.Max.Sub(image.Pt(1, 1))} {
				if pixels.RGBAAt(point.X, point.Y) != (color.RGBA{}) {
					t.Fatal("rounded corner retained compositor shadow")
				}
			}
			if pixels.RGBAAt(40, 20) != content || pixels.RGBAAt(pixels.Rect.Min.X+int(radius)+1, pixels.Rect.Min.Y) != content {
				t.Fatal("corner clipping modified native content or a straight border")
			}
			if got := pixels.RGBAAt(edge.X, edge.Y); got != translucent {
				t.Fatalf("native edge alpha was applied twice: got=%v want=%v", got, translucent)
			}
		})
	}
	for _, corner := range []color.RGBA{{A: 255}, {R: 10, A: 64}} {
		pixels := image.NewRGBA(image.Rect(0, 0, 40, 30))
		draw.Draw(pixels, pixels.Bounds(), image.NewUniform(corner), image.Point{}, draw.Src)
		clipWindowsCapturedCornerPixels(pixels, 8)
		if pixels.RGBAAt(0, 0) != corner || pixels.RGBAAt(39, 29) != corner {
			t.Fatal("opaque or authored colored window corners were changed")
		}
	}
	pixels := image.NewRGBA(image.Rect(0, 0, 40, 30))
	draw.Draw(pixels, pixels.Bounds(), image.NewUniform(color.RGBA{R: 80, A: 128}), image.Point{}, draw.Src)
	for _, point := range [...]image.Point{{}, {X: 39}, {Y: 29}, {X: 39, Y: 29}} {
		pixels.SetRGBA(point.X, point.Y, color.RGBA{})
	}
	before := append([]byte(nil), pixels.Pix...)
	clipWindowsCapturedCornerPixels(pixels, 8)
	if !bytes.Equal(pixels.Pix, before) {
		t.Fatal("already transparent native corners were clipped a second time")
	}
}

// TestWindowsWindowCaptureRetainsRoundedAlpha reads real compositor pixels from a disposable native window.
func TestWindowsWindowCaptureRetainsRoundedAlpha(t *testing.T) {
	if os.Getenv("WOX_WINDOW_CAPTURE_TEST") != "1" {
		t.Skip("requires an interactive Windows desktop")
	}
	for _, test := range []struct {
		name         string
		customRegion bool
		backdrop     bool
		preference   uint32
	}{
		{name: "custom region", customRegion: true},
		{name: "default system corners"},
		{name: "requested system corners", preference: 2},
		{name: "small system corners", preference: 3},
		{name: "system corners with backdrop", preference: 2, backdrop: true},
		{name: "disabled system corners", preference: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			testWindowsWindowAlpha(t, test.customRegion, test.backdrop, test.preference)
		})
	}
}

// testWindowsWindowAlpha exercises both authored window regions and DWM's system rounding.
func testWindowsWindowAlpha(t *testing.T, customRegion, backdrop bool, preference uint32) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	class, _ := syscall.UTF16PtrFromString("STATIC")
	title, _ := syscall.UTF16PtrFromString("Wox window alpha test")
	style := uint32(win.WS_POPUP)
	if !customRegion && !backdrop {
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
		if status, _, _ := dwmSetWindowAttribute.Call(uintptr(hwnd), dwmwaWindowCorner, uintptr(unsafe.Pointer(&preference)), unsafe.Sizeof(preference)); int32(status) < 0 {
			t.Skip("system rounding requires Windows 11")
		}
	}
	if backdrop {
		applyCaptureTestBackdrop(hwnd)
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
	if customRegion || preference == 1 {
		// Non-client borders can be translucent even when system rounding is disabled.
		// Disabled rounding and authored regions must preserve their pixels without a system mask.
		shadow := image.NewRGBA(image.Rect(0, 0, 40, 30))
		draw.Draw(shadow, shadow.Bounds(), image.NewUniform(color.RGBA{A: 64}), image.Point{}, draw.Src)
		before := append([]byte(nil), shadow.Pix...)
		clipWindowsCapturedWindowCorners(hwnd, shadow)
		if !bytes.Equal(before, shadow.Pix) {
			t.Fatal("disabled system rounding or authored region changed captured pixels")
		}
	}
	for _, point := range [...]image.Point{pixels.Rect.Min, {X: pixels.Rect.Max.X - 1}, {Y: pixels.Rect.Max.Y - 1}, pixels.Rect.Max.Sub(image.Pt(1, 1))} {
		if preference == 1 {
			if c := pixels.RGBAAt(point.X, point.Y); c.A == 0 {
				t.Fatalf("square corner became transparent at %v: %+v", point, c)
			}
			continue
		}
		if c := pixels.RGBAAt(point.X, point.Y); c.A != 0 {
			t.Fatalf("rounded corner contains compositor shadow at %v: %+v", point, c)
		}
	}
	c := pixels.RGBAAt(pixels.Bounds().Dx()/2, pixels.Bounds().Dy()/2)
	// Acrylic tint varies with composition timing; the solid fixture checks exact channel order.
	if c.A == 0 || c.R < 200 || (!backdrop && (c.A != 255 || c.G > 70 || c.B > 90)) {
		t.Fatalf("window content or channel order changed: %+v", c)
	}
}
