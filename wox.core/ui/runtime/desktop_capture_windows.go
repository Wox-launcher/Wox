//go:build windows

package woxui

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/win"
)

const windowsCaptureBlt = uint32(0x40000000)

var monitorFromRect = syscall.NewLazyDLL("user32.dll").NewProc("MonitorFromRect")
var dwmFlush = syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmFlush")

// WindowsDesktopCaptureTimings separates the native capture stages for diagnostics.
type WindowsDesktopCaptureTimings struct {
	Setup   time.Duration
	BitBlt  time.Duration
	Convert time.Duration
	Total   time.Duration
}

// WindowsDesktopCapture owns a top-down DIB and its mapped RGBA view.
type WindowsDesktopCapture struct {
	Image   *image.RGBA
	Bounds  image.Rectangle
	Timings WindowsDesktopCaptureTimings

	bitmap    win.HBITMAP
	closeOnce sync.Once
	closeErr  error
}

// Close releases the DIB after every consumer has stopped reading Image.
func (capture *WindowsDesktopCapture) Close() error {
	if capture == nil {
		return nil
	}
	capture.closeOnce.Do(func() {
		if capture.bitmap != 0 && !win.DeleteObject(win.HGDIOBJ(capture.bitmap)) {
			capture.closeErr = errors.New("failed to release the Windows screenshot DIB")
		}
		capture.bitmap = 0
		capture.Image = nil
	})
	return capture.closeErr
}

// FlushWindowsDesktopComposition waits for pending DWM updates without an arbitrary sleep.
func FlushWindowsDesktopComposition() {
	if dwmFlush.Find() == nil {
		_, _, _ = dwmFlush.Call()
	}
}

// CaptureWindowsVirtualDesktop captures directly into one mapped top-down DIB.
func CaptureWindowsVirtualDesktop() (*WindowsDesktopCapture, error) {
	startedAt := time.Now()
	x := win.GetSystemMetrics(win.SM_XVIRTUALSCREEN)
	y := win.GetSystemMetrics(win.SM_YVIRTUALSCREEN)
	width := win.GetSystemMetrics(win.SM_CXVIRTUALSCREEN)
	height := win.GetSystemMetrics(win.SM_CYVIRTUALSCREEN)
	if width <= 0 || height <= 0 || width > 16384 || height > 16384 {
		return nil, fmt.Errorf("invalid virtual desktop size: %dx%d", width, height)
	}

	screenDC := win.GetDC(0)
	if screenDC == 0 {
		return nil, errors.New("failed to open the Windows desktop device context")
	}
	memoryDC := win.CreateCompatibleDC(screenDC)
	if memoryDC == 0 {
		win.ReleaseDC(0, screenDC)
		return nil, errors.New("failed to create the screenshot memory device context")
	}
	bitmapInfo := win.BITMAPINFOHEADER{
		BiSize:        uint32(unsafe.Sizeof(win.BITMAPINFOHEADER{})),
		BiWidth:       width,
		BiHeight:      -height,
		BiPlanes:      1,
		BiBitCount:    32,
		BiCompression: win.BI_RGB,
	}
	var bits unsafe.Pointer
	bitmap := win.CreateDIBSection(screenDC, &bitmapInfo, win.DIB_RGB_COLORS, &bits, 0, 0)
	if bitmap == 0 || bits == nil {
		win.DeleteDC(memoryDC)
		win.ReleaseDC(0, screenDC)
		return nil, errors.New("failed to create the Windows screenshot DIB")
	}
	previous := win.SelectObject(memoryDC, win.HGDIOBJ(bitmap))
	if previous == 0 {
		win.DeleteObject(win.HGDIOBJ(bitmap))
		win.DeleteDC(memoryDC)
		win.ReleaseDC(0, screenDC)
		return nil, errors.New("failed to select the Windows screenshot DIB")
	}
	setupDuration := time.Since(startedAt)

	bitBltStartedAt := time.Now()
	copied := win.BitBlt(memoryDC, 0, 0, width, height, screenDC, x, y, win.SRCCOPY|windowsCaptureBlt)
	bitBltDuration := time.Since(bitBltStartedAt)
	win.SelectObject(memoryDC, previous)
	win.DeleteDC(memoryDC)
	win.ReleaseDC(0, screenDC)
	if !copied {
		win.DeleteObject(win.HGDIOBJ(bitmap))
		return nil, errors.New("failed to copy Windows desktop pixels")
	}

	pixelBytes := int(width) * int(height) * 4
	pixels := unsafe.Slice((*byte)(bits), pixelBytes)
	convertStartedAt := time.Now()
	windowsConvertDIBToRGBA(pixels)
	convertDuration := time.Since(convertStartedAt)
	capture := &WindowsDesktopCapture{
		Image:  &image.RGBA{Pix: pixels, Stride: int(width) * 4, Rect: image.Rect(0, 0, int(width), int(height))},
		Bounds: image.Rect(int(x), int(y), int(x+width), int(y+height)),
		bitmap: bitmap,
	}
	capture.Timings = WindowsDesktopCaptureTimings{
		Setup: setupDuration, BitBlt: bitBltDuration, Convert: convertDuration, Total: time.Since(startedAt),
	}
	return capture, nil
}

// windowsConvertDIBToRGBA converts GDI's BGRX bytes in place without another desktop buffer.
func windowsConvertDIBToRGBA(pixels []byte) {
	for offset := 0; offset+3 < len(pixels); offset += 4 {
		pixels[offset], pixels[offset+2] = pixels[offset+2], pixels[offset]
		pixels[offset+3] = 255
	}
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
