//go:build windows

package woxui

import (
	"errors"
	"fmt"
	"image"
	"time"
	"unsafe"

	"github.com/lxn/win"
)

const captureBlt = uint32(0x40000000)

func captureScreenshotPlatform(options ScreenshotOptions) (ScreenshotResult, error) {
	if options.ExportFilePath == "" {
		return ScreenshotResult{}, errors.New("screenshot export file path is empty")
	}
	// Give DWM one frame to remove the launcher before copying the desktop pixels.
	time.Sleep(80 * time.Millisecond)
	source, virtualBounds, err := captureWindowsVirtualDesktop()
	if err != nil {
		return ScreenshotResult{}, err
	}
	return runScreenshotEditor(options, source, screenshotEditorPlatform{
		setWindowBounds: func(window *Window) error {
			return window.native.setPhysicalBounds(Rect{
				X:      float32(virtualBounds.Min.X),
				Y:      float32(virtualBounds.Min.Y),
				Width:  float32(virtualBounds.Dx()),
				Height: float32(virtualBounds.Dy()),
			})
		},
		logicalSelection: func(selection Rect, frameSize Size) Rect {
			scaleX := float32(source.Bounds().Dx()) / frameSize.Width
			scaleY := float32(source.Bounds().Dy()) / frameSize.Height
			return Rect{
				X:      float32(virtualBounds.Min.X)/scaleX + selection.X,
				Y:      float32(virtualBounds.Min.Y)/scaleY + selection.Y,
				Width:  selection.Width,
				Height: selection.Height,
			}
		},
		captureDesktop: func() (image.Image, error) {
			captured, _, captureErr := captureWindowsVirtualDesktop()
			return captured, captureErr
		},
	})
}

func captureWindowsVirtualDesktop() (*image.RGBA, image.Rectangle, error) {
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
	if !win.BitBlt(memoryDC, 0, 0, width, height, screenDC, x, y, win.SRCCOPY|captureBlt) {
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
	bgra := make([]byte, int(width)*int(height)*4)
	if win.GetDIBits(memoryDC, bitmap, 0, uint32(height), &bgra[0], &bitmapInfo, win.DIB_RGB_COLORS) == 0 {
		return nil, image.Rectangle{}, errors.New("failed to read Windows screenshot pixels")
	}
	rgba := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	for offset := 0; offset < len(bgra); offset += 4 {
		rgba.Pix[offset] = bgra[offset+2]
		rgba.Pix[offset+1] = bgra[offset+1]
		rgba.Pix[offset+2] = bgra[offset]
		rgba.Pix[offset+3] = 255
	}
	return rgba, image.Rect(int(x), int(y), int(x+width), int(y+height)), nil
}
