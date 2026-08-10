//go:build windows

package woxui

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
)

const windowsCaptureBlt = uint32(0x40000000)

var monitorFromRect = syscall.NewLazyDLL("user32.dll").NewProc("MonitorFromRect")
var dwmFlush = syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmFlush")

// FlushWindowsDesktopComposition waits for pending DWM updates without an arbitrary sleep.
func FlushWindowsDesktopComposition() {
	if dwmFlush.Find() == nil {
		_, _, _ = dwmFlush.Call()
	}
}

// CaptureWindowsVirtualDesktop returns the virtual desktop in physical pixel coordinates.
func CaptureWindowsVirtualDesktop() (*image.RGBA, image.Rectangle, error) {
	x := win.GetSystemMetrics(win.SM_XVIRTUALSCREEN)
	y := win.GetSystemMetrics(win.SM_YVIRTUALSCREEN)
	width := win.GetSystemMetrics(win.SM_CXVIRTUALSCREEN)
	height := win.GetSystemMetrics(win.SM_CYVIRTUALSCREEN)
	if width <= 0 || height <= 0 || width > 16384 || height > 16384 {
		return nil, image.Rectangle{}, fmt.Errorf("invalid virtual desktop size: %dx%d", width, height)
	}
	screenDC := win.GetDC(0)
	if screenDC == 0 {
		return nil, image.Rectangle{}, errors.New("failed to open the Windows desktop device context")
	}
	defer win.ReleaseDC(0, screenDC)
	memoryDC := win.CreateCompatibleDC(screenDC)
	if memoryDC == 0 {
		return nil, image.Rectangle{}, errors.New("failed to create the screenshot memory device context")
	}
	defer win.DeleteDC(memoryDC)
	bitmap := win.CreateCompatibleBitmap(screenDC, width, height)
	if bitmap == 0 {
		return nil, image.Rectangle{}, errors.New("failed to create the screenshot bitmap")
	}
	defer win.DeleteObject(win.HGDIOBJ(bitmap))
	previous := win.SelectObject(memoryDC, win.HGDIOBJ(bitmap))
	if previous == 0 {
		return nil, image.Rectangle{}, errors.New("failed to select the screenshot bitmap")
	}
	if !win.BitBlt(memoryDC, 0, 0, width, height, screenDC, x, y, win.SRCCOPY|windowsCaptureBlt) {
		win.SelectObject(memoryDC, previous)
		return nil, image.Rectangle{}, errors.New("failed to copy Windows desktop pixels")
	}
	win.SelectObject(memoryDC, previous)

	bitmapInfo := win.BITMAPINFO{BmiHeader: win.BITMAPINFOHEADER{
		BiSize:        uint32(unsafe.Sizeof(win.BITMAPINFOHEADER{})),
		BiWidth:       width,
		BiHeight:      -height,
		BiPlanes:      1,
		BiBitCount:    32,
		BiCompression: win.BI_RGB,
	}}
	rgba := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	if win.GetDIBits(memoryDC, bitmap, 0, uint32(height), &rgba.Pix[0], &bitmapInfo, win.DIB_RGB_COLORS) == 0 {
		return nil, image.Rectangle{}, errors.New("failed to read Windows screenshot pixels")
	}
	// GetDIBits writes BGRA. Swap red and blue in place so capture keeps one desktop buffer.
	for offset := 0; offset < len(rgba.Pix); offset += 4 {
		rgba.Pix[offset], rgba.Pix[offset+2] = rgba.Pix[offset+2], rgba.Pix[offset]
		rgba.Pix[offset+3] = 255
	}
	return rgba, image.Rect(int(x), int(y), int(x+width), int(y+height)), nil
}

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

func writeWindowsCapturePNG(path string, source image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(file, source); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
