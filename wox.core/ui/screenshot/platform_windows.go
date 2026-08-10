//go:build windows

package screenshot

import (
	"context"
	"errors"
	"fmt"
	"image"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/win"
	woxui "wox/ui/runtime"
	"wox/util"
)

const windowsCursorShowing = uint32(1)

var getCursorInfo = syscall.NewLazyDLL("user32.dll").NewProc("GetCursorInfo")

type windowsCursorInfo struct {
	size     uint32
	flags    uint32
	cursor   win.HCURSOR
	position win.POINT
}

func captureScreenshotPlatform(options ScreenshotOptions) (ScreenshotResult, error) {
	startedAt := time.Now()
	if options.ExportFilePath == "" {
		return ScreenshotResult{}, errors.New("screenshot export file path is empty")
	}
	capturedCursorPosition, capturedCursor := captureWindowsCursor()
	type desktopCapture struct {
		capture      *woxui.WindowsDesktopCapture
		flush        time.Duration
		captureTotal time.Duration
		err          error
	}
	captureDone := make(chan desktopCapture, 1)
	go func() {
		flushStartedAt := time.Now()
		woxui.FlushWindowsDesktopComposition()
		flushDuration := time.Since(flushStartedAt)
		captureStartedAt := time.Now()
		capture, err := woxui.CaptureWindowsVirtualDesktop()
		captureDone <- desktopCapture{capture: capture, flush: flushDuration, captureTotal: flushDuration + time.Since(captureStartedAt), err: err}
	}()

	windowHost := &screenshotEditorWindowHost{}
	prepareStartedAt := time.Now()
	preparedWindow, prepareErr := prepareScreenshotEditorWindowOnUIThread(options.WindowManager, windowHost)
	prepareDuration := time.Since(prepareStartedAt)
	captured := <-captureDone
	if captured.err != nil {
		if preparedWindow != nil {
			_ = preparedWindow.Close()
		}
		return ScreenshotResult{}, captured.err
	}
	defer captured.capture.Close()
	if prepareErr != nil {
		return ScreenshotResult{}, prepareErr
	}
	source, virtualBounds := captured.capture.Image, captured.capture.Bounds
	platform := screenshotEditorPlatform{
		setWindowBounds: func(window *Window) error {
			return window.SetPhysicalBounds(Rect{
				X:      float32(virtualBounds.Min.X),
				Y:      float32(virtualBounds.Min.Y),
				Width:  float32(virtualBounds.Dx()),
				Height: float32(virtualBounds.Dy()),
			})
		},
		logicalSelection: func(selection Rect, _ Size) Rect {
			return woxui.WindowsLogicalRectFromPhysical(Rect{
				X:      float32(virtualBounds.Min.X) + selection.X,
				Y:      float32(virtualBounds.Min.Y) + selection.Y,
				Width:  selection.Width,
				Height: selection.Height,
			})
		},
		chromeScale: func(selection Rect) float32 {
			return woxui.WindowsPhysicalRectScale(Rect{
				X:      float32(virtualBounds.Min.X) + selection.X,
				Y:      float32(virtualBounds.Min.Y) + selection.Y,
				Width:  selection.Width,
				Height: selection.Height,
			})
		},
		captureDesktop: func() (screenshotDesktopCapture, error) {
			capture, captureErr := woxui.CaptureWindowsVirtualDesktop()
			if captureErr != nil {
				return screenshotDesktopCapture{}, captureErr
			}
			return screenshotDesktopCapture{
				source: capture.Image,
				release: func() {
					_ = capture.Close()
				},
			}, nil
		},
		preparedWindow: preparedWindow,
		windowHost:     windowHost,
		afterShow: func() {
			util.GetLogger().Debug(context.Background(), fmt.Sprintf(
				"screenshot overlay ready: total=%s capture=%s dwm=%s setup=%s bitblt=%s convert=%s renderer=%s desktop=%dx%d",
				time.Since(startedAt).Round(time.Millisecond), captured.captureTotal.Round(time.Millisecond), captured.flush.Round(time.Millisecond),
				captured.capture.Timings.Setup.Round(time.Millisecond), captured.capture.Timings.BitBlt.Round(time.Millisecond), captured.capture.Timings.Convert.Round(time.Millisecond),
				prepareDuration.Round(time.Millisecond), virtualBounds.Dx(), virtualBounds.Dy(),
			))
		},
	}
	if capturedCursorPosition != nil {
		platform.cursorPixel = screenshotEditorCursorPixelFromDesktop(
			Point{X: float32(capturedCursorPosition.X), Y: float32(capturedCursorPosition.Y)},
			Rect{X: float32(virtualBounds.Min.X), Y: float32(virtualBounds.Min.Y), Width: float32(virtualBounds.Dx()), Height: float32(virtualBounds.Dy())},
			source,
		)
		platform.capturedCursor = capturedCursor
	}
	return runScreenshotEditor(options, source, platform)
}

// captureWindowsCursor preserves the visible native cursor image and its hotspot at capture time.
func captureWindowsCursor() (*win.POINT, *screenshotEditorCapturedCursor) {
	info := windowsCursorInfo{size: uint32(unsafe.Sizeof(windowsCursorInfo{}))}
	result, _, _ := getCursorInfo.Call(uintptr(unsafe.Pointer(&info)))
	if result == 0 || info.flags&windowsCursorShowing == 0 || info.cursor == 0 {
		var position win.POINT
		if win.GetCursorPos(&position) {
			return &position, nil
		}
		return nil, nil
	}
	raster, hotspot, err := renderWindowsCursor(info.cursor)
	if err != nil {
		return &info.position, nil
	}
	preview, err := NewImage(raster)
	if err != nil {
		return &info.position, nil
	}
	return &info.position, &screenshotEditorCapturedCursor{raster: raster, preview: preview, hotspot: hotspot}
}

// renderWindowsCursor reconstructs cursor alpha from black and white backgrounds for legacy and color cursors.
func renderWindowsCursor(cursor win.HCURSOR) (*image.RGBA, Point, error) {
	var iconInfo win.ICONINFO
	if !win.GetIconInfo(win.HICON(cursor), &iconInfo) {
		return nil, Point{}, errors.New("failed to read Windows cursor metadata")
	}
	if iconInfo.HbmColor != 0 {
		defer win.DeleteObject(win.HGDIOBJ(iconInfo.HbmColor))
	}
	if iconInfo.HbmMask != 0 {
		defer win.DeleteObject(win.HGDIOBJ(iconInfo.HbmMask))
	}
	bitmapHandle := iconInfo.HbmColor
	monochrome := false
	if bitmapHandle == 0 {
		bitmapHandle = iconInfo.HbmMask
		monochrome = true
	}
	var bitmap win.BITMAP
	if bitmapHandle == 0 || win.GetObject(win.HGDIOBJ(bitmapHandle), uintptr(unsafe.Sizeof(bitmap)), unsafe.Pointer(&bitmap)) == 0 {
		return nil, Point{}, errors.New("failed to resolve Windows cursor dimensions")
	}
	width, height := int(bitmap.BmWidth), int(bitmap.BmHeight)
	if monochrome {
		height /= 2
	}
	if width <= 0 || height <= 0 || width > 512 || height > 512 {
		return nil, Point{}, fmt.Errorf("invalid Windows cursor dimensions: %dx%d", width, height)
	}
	black, err := drawWindowsCursorBackground(cursor, width, height, 0)
	if err != nil {
		return nil, Point{}, err
	}
	white, err := drawWindowsCursorBackground(cursor, width, height, 255)
	if err != nil {
		return nil, Point{}, err
	}
	output := image.NewRGBA(image.Rect(0, 0, width, height))
	for offset := 0; offset < len(output.Pix); offset += 4 {
		deltaB := max(0, int(white[offset])-int(black[offset]))
		deltaG := max(0, int(white[offset+1])-int(black[offset+1]))
		deltaR := max(0, int(white[offset+2])-int(black[offset+2]))
		alpha := uint8(255 - min(255, (deltaR+deltaG+deltaB)/3))
		output.Pix[offset] = black[offset+2]
		output.Pix[offset+1] = black[offset+1]
		output.Pix[offset+2] = black[offset]
		output.Pix[offset+3] = alpha
	}
	return output, Point{X: float32(iconInfo.XHotspot), Y: float32(iconInfo.YHotspot)}, nil
}

func drawWindowsCursorBackground(cursor win.HCURSOR, width, height int, background byte) ([]byte, error) {
	screenDC := win.GetDC(0)
	if screenDC == 0 {
		return nil, errors.New("failed to open Windows cursor device context")
	}
	defer win.ReleaseDC(0, screenDC)
	memoryDC := win.CreateCompatibleDC(screenDC)
	if memoryDC == 0 {
		return nil, errors.New("failed to create Windows cursor device context")
	}
	defer win.DeleteDC(memoryDC)
	bitmapInfo := win.BITMAPINFOHEADER{
		BiSize: uint32(unsafe.Sizeof(win.BITMAPINFOHEADER{})), BiWidth: int32(width), BiHeight: -int32(height),
		BiPlanes: 1, BiBitCount: 32, BiCompression: win.BI_RGB,
	}
	var bits unsafe.Pointer
	bitmap := win.CreateDIBSection(screenDC, &bitmapInfo, win.DIB_RGB_COLORS, &bits, 0, 0)
	if bitmap == 0 || bits == nil {
		return nil, errors.New("failed to create Windows cursor bitmap")
	}
	defer win.DeleteObject(win.HGDIOBJ(bitmap))
	previous := win.SelectObject(memoryDC, win.HGDIOBJ(bitmap))
	if previous == 0 {
		return nil, errors.New("failed to select Windows cursor bitmap")
	}
	defer win.SelectObject(memoryDC, previous)
	pixels := unsafe.Slice((*byte)(bits), width*height*4)
	for offset := 0; offset < len(pixels); offset += 4 {
		pixels[offset], pixels[offset+1], pixels[offset+2], pixels[offset+3] = background, background, background, 255
	}
	if !win.DrawIconEx(memoryDC, 0, 0, win.HICON(cursor), int32(width), int32(height), 0, 0, win.DI_NORMAL) {
		return nil, errors.New("failed to draw Windows cursor")
	}
	return append([]byte(nil), pixels...), nil
}
