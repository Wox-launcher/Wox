//go:build windows

package screenshot

import (
	"context"
	"errors"
	"fmt"
	"image"
	"math"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/win"
	woxui "wox/ui/runtime"
	"wox/util"
	"wox/util/screen"
)

// restoreScreenshotDocumentDesktop uses the same physical desktop surface as a fresh capture.
// A single spanning window puts each monitor's saved pixels and mask back on that monitor.
func restoreScreenshotDocumentDesktop(platform *screenshotEditorPlatform, document *screenshotDocument, displays []screen.Display) error {
	bounds, available := screenshotDocumentDesktopBounds(document, displays, true)
	if !available {
		return ErrScreenshotDisplayLayoutChanged
	}
	platform.setWindowBounds = func(window *Window) error { return window.SetPhysicalBounds(bounds) }
	platform.logicalSelection = func(selection Rect, _ Size) Rect {
		selection.X += bounds.X
		selection.Y += bounds.Y
		return woxui.WindowsLogicalRectFromPhysical(selection)
	}
	platform.chromeScale = func(selection Rect) float32 {
		selection.X += bounds.X
		selection.Y += bounds.Y
		return woxui.WindowsPhysicalRectScale(selection)
	}
	return nil
}

const windowsCursorShowing = uint32(1)

const windowsScrollingBorderClassName = "WoxScrollingCaptureBorder"

var (
	getCursorInfo                         = syscall.NewLazyDLL("user32.dll").NewProc("GetCursorInfo")
	setCursorPos                          = syscall.NewLazyDLL("user32.dll").NewProc("SetCursorPos")
	createSolidBrush                      = syscall.NewLazyDLL("gdi32.dll").NewProc("CreateSolidBrush")
	windowsScrollingBorderClassOnce       sync.Once
	windowsScrollingBorderClassErr        error
	windowsScrollingBorderWindowProcedure = syscall.NewCallback(windowsScrollingBorderWindowProc)
)

type windowsCursorInfo struct {
	size     uint32
	flags    uint32
	cursor   win.HCURSOR
	position win.POINT
}

func captureScreenshotPlatform(options ScreenshotOptions) (ScreenshotResult, error) {
	startedAt := time.Now()
	windowCandidates := captureWindowsWindowCandidates()
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
	recordingWorkAreas := windowsRecordingWorkAreas(options.recordingDisplays, virtualBounds)
	objectQuery, objectQueryClose := windowsScreenshotObjectQuery(windowCandidates, virtualBounds)
	platform := screenshotEditorPlatform{
		windowCandidates: windowsScreenshotWindowCandidates(windowCandidates, virtualBounds),
		displayCandidates: screenshotSelectionDisplayBounds(options.capturedDisplays, Rect{
			X: float32(virtualBounds.Min.X), Y: float32(virtualBounds.Min.Y), Width: float32(virtualBounds.Dx()), Height: float32(virtualBounds.Dy()),
		}, true),
		objectQuery:      objectQuery,
		objectQueryClose: objectQueryClose,
		captureWindow: func(selection Rect) (*image.RGBA, error) {
			return captureWindowsSelectedWindow(windowCandidates, virtualBounds, selection)
		},
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
		desktopPixelOrigin: Point{X: float32(virtualBounds.Min.X), Y: float32(virtualBounds.Min.Y)},
		setPointerPosition: func(point Point) error {
			x := windowsScreenshotPointerCoordinate(virtualBounds.Min.X, point.X)
			y := windowsScreenshotPointerCoordinate(virtualBounds.Min.Y, point.Y)
			result, _, callErr := setCursorPos.Call(uintptr(uint32(x)), uintptr(uint32(y)))
			if result == 0 {
				return fmt.Errorf("set Windows cursor position: %w", callErr)
			}
			return nil
		},
		cursorPosition: func() *Point {
			var position win.POINT
			if !win.GetCursorPos(&position) {
				return nil
			}
			return &Point{X: float32(position.X - int32(virtualBounds.Min.X)), Y: float32(position.Y - int32(virtualBounds.Min.Y))}
		},
		setRecordingBounds: func(window *Window, selection Rect, _ Size, margin float32) error {
			return window.SetPhysicalBounds(Rect{
				X: float32(virtualBounds.Min.X) + selection.X - margin, Y: float32(virtualBounds.Min.Y) + selection.Y - margin,
				Width: selection.Width + margin*2, Height: selection.Height + margin*2,
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
		captureDesktopRect: func(pixelBounds image.Rectangle) (*image.RGBA, error) {
			screen := pixelBounds.Add(image.Pt(virtualBounds.Min.X, virtualBounds.Min.Y))
			return woxui.CaptureWindowsRect(screen)
		},
		openRecordingCapture: func(pixelBounds image.Rectangle) (func(*image.RGBA) error, func(), error) {
			screen := pixelBounds.Add(image.Pt(virtualBounds.Min.X, virtualBounds.Min.Y))
			capturer, err := woxui.NewWindowsRectCapturer(screen)
			if err != nil {
				return nil, nil, err
			}
			return capturer.CaptureInto, func() { _ = capturer.Close() }, nil
		},
		setScrollBounds: func(window *Window, controls Rect, _ Size) error {
			return window.SetPhysicalBounds(Rect{
				X:      float32(virtualBounds.Min.X) + controls.X,
				Y:      float32(virtualBounds.Min.Y) + controls.Y,
				Width:  controls.Width,
				Height: controls.Height,
			})
		},
		showScrollBorder: func(selection Rect, _ Size) (func(), error) {
			globalSelection := Rect{
				X:      float32(virtualBounds.Min.X) + selection.X,
				Y:      float32(virtualBounds.Min.Y) + selection.Y,
				Width:  selection.Width,
				Height: selection.Height,
			}
			thickness := int32(math.Round(float64(2 * woxui.WindowsPhysicalRectScale(globalSelection))))
			rects := windowsScrollingCaptureBorderRects(globalSelection, max(int32(2), thickness))
			return showWindowsCaptureBorder(rects[:])
		},
		showRecordingBorder: func(selection Rect, _ Size) (func(), error) {
			return showWindowsRecordingBorder(selection, virtualBounds.Min, recordingWorkAreas)
		},
		recordingWorkAreas: recordingWorkAreas,
		preparedWindow:     preparedWindow,
		windowHost:         windowHost,
		afterShow: func() {
			util.GetLogger().Debug(context.Background(), fmt.Sprintf(
				"screenshot overlay ready: total=%s capture=%s dwm=%s setup=%s bitblt=%s renderer=%s desktop=%dx%d",
				time.Since(startedAt).Round(time.Millisecond), captured.captureTotal.Round(time.Millisecond), captured.flush.Round(time.Millisecond),
				captured.capture.Timings.Setup.Round(time.Millisecond), captured.capture.Timings.BitBlt.Round(time.Millisecond),
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

// windowsScreenshotPointerCoordinate keeps a sampled pixel center inside that pixel when converting to SetCursorPos integers.
func windowsScreenshotPointerCoordinate(origin int, point float32) int32 {
	return int32(math.Floor(float64(float32(origin) + point)))
}

// windowsRecordingWorkAreas maps physical work areas into the captured virtual desktop.
func windowsRecordingWorkAreas(displays []screen.Display, desktop image.Rectangle) []Rect {
	var areas []Rect
	for _, display := range displays {
		work := display.PixelWorkArea
		if work.IsEmpty() {
			work = display.PixelBounds
		}
		visible := image.Rect(work.X, work.Y, work.X+work.Width, work.Y+work.Height).Intersect(desktop)
		if !visible.Empty() {
			areas = append(areas, Rect{
				X: float32(visible.Min.X - desktop.Min.X), Y: float32(visible.Min.Y - desktop.Min.Y),
				Width: float32(visible.Dx()), Height: float32(visible.Dy()),
			})
		}
	}
	return areas
}

// showWindowsRecordingBorder keeps the green edges visible on full-display recordings.
func showWindowsRecordingBorder(selection Rect, origin image.Point, workAreas []Rect) (func(), error) {
	global := selection
	global.X += float32(origin.X)
	global.Y += float32(origin.Y)
	thickness := int32(math.Round(float64(2 * woxui.WindowsPhysicalRectScale(global))))
	rects := windowsRecordingCaptureBorderRects(selection, workAreas, max(int32(2), thickness))
	for i := range rects {
		rects[i].X += float32(origin.X)
		rects[i].Y += float32(origin.Y)
	}
	return showWindowsCaptureBorder(rects)
}

// showWindowsCaptureBorder presents passive strips and returns their session-owned cleanup.
func showWindowsCaptureBorder(rects []Rect) (func(), error) {
	var windows []win.HWND
	var createErr error
	if err := Call(func() {
		windows, createErr = createWindowsCaptureBorder(rects)
	}); err != nil {
		return nil, err
	}
	if createErr != nil {
		return nil, createErr
	}
	var closeOnce sync.Once
	return func() {
		closeOnce.Do(func() {
			_ = Call(func() {
				for _, window := range windows {
					if window != 0 {
						win.DestroyWindow(window)
					}
				}
			})
		})
	}, nil
}

// createWindowsCaptureBorder creates passive topmost edges around the live viewport.
func createWindowsCaptureBorder(rects []Rect) ([]win.HWND, error) {
	if err := ensureWindowsScrollingBorderClass(); err != nil {
		return nil, err
	}
	className, err := syscall.UTF16PtrFromString(windowsScrollingBorderClassName)
	if err != nil {
		return nil, err
	}
	title, err := syscall.UTF16PtrFromString("")
	if err != nil {
		return nil, err
	}
	instance := win.GetModuleHandle(nil)
	windows := make([]win.HWND, 0, len(rects))
	for _, rect := range rects {
		window := win.CreateWindowEx(
			uint32(win.WS_EX_TOPMOST|win.WS_EX_TOOLWINDOW|win.WS_EX_TRANSPARENT|win.WS_EX_NOACTIVATE),
			className,
			title,
			win.WS_POPUP,
			int32(math.Round(float64(rect.X))),
			int32(math.Round(float64(rect.Y))),
			int32(math.Round(float64(rect.Width))),
			int32(math.Round(float64(rect.Height))),
			0,
			0,
			instance,
			nil,
		)
		if window == 0 {
			for _, created := range windows {
				win.DestroyWindow(created)
			}
			return nil, fmt.Errorf("create scrolling capture border window: %w", syscall.GetLastError())
		}
		windows = append(windows, window)
		win.ShowWindow(window, win.SW_SHOWNOACTIVATE)
		win.RedrawWindow(window, nil, 0, win.RDW_INVALIDATE|win.RDW_ERASE|win.RDW_UPDATENOW)
	}
	return windows, nil
}

// windowsScrollingCaptureBorderRects places every border pixel outside the captured viewport.
func windowsScrollingCaptureBorderRects(selection Rect, thickness int32) [4]Rect {
	border := float32(max(int32(1), thickness))
	return [4]Rect{
		{X: selection.X - border, Y: selection.Y - border, Width: selection.Width + border*2, Height: border},
		{X: selection.X + selection.Width, Y: selection.Y, Width: border, Height: selection.Height},
		{X: selection.X - border, Y: selection.Y + selection.Height, Width: selection.Width + border*2, Height: border},
		{X: selection.X - border, Y: selection.Y, Width: border, Height: selection.Height},
	}
}

// windowsRecordingCaptureBorderRects uses outside strips where possible and moves clipped edges
// inward at the screen or taskbar boundary. Each work area is already in capture coordinates.
func windowsRecordingCaptureBorderRects(selection Rect, workAreas []Rect, thickness int32) []Rect {
	var rects []Rect
	for _, area := range workAreas {
		if !screenshotEditorRectsOverlap(selection, area) {
			continue
		}
		areaRight, areaBottom := area.X+area.Width, area.Y+area.Height
		border := min(float32(max(int32(1), thickness)), area.Width/2, area.Height/2)
		left := max(area.X, selection.X-border)
		top := max(area.Y, selection.Y-border)
		right := min(areaRight-border, selection.X+selection.Width)
		bottom := min(areaBottom-border, selection.Y+selection.Height)
		rects = append(rects,
			Rect{X: left, Y: top, Width: right + border - left, Height: border},
			Rect{X: right, Y: top + border, Width: border, Height: bottom - top - border},
			Rect{X: left, Y: bottom, Width: right + border - left, Height: border},
			Rect{X: left, Y: top + border, Width: border, Height: bottom - top - border},
		)
	}
	if len(rects) == 0 {
		outside := windowsScrollingCaptureBorderRects(selection, thickness)
		return outside[:]
	}
	return rects
}

// ensureWindowsScrollingBorderClass registers the process-wide class with the Flutter border color.
func ensureWindowsScrollingBorderClass() error {
	windowsScrollingBorderClassOnce.Do(func() {
		className, err := syscall.UTF16PtrFromString(windowsScrollingBorderClassName)
		if err != nil {
			windowsScrollingBorderClassErr = err
			return
		}
		brush, _, _ := createSolidBrush.Call(uintptr(win.RGB(41, 255, 114)))
		if brush == 0 {
			windowsScrollingBorderClassErr = errors.New("create scrolling capture border brush")
			return
		}
		windowClass := win.WNDCLASSEX{
			CbSize:        uint32(unsafe.Sizeof(win.WNDCLASSEX{})),
			LpfnWndProc:   windowsScrollingBorderWindowProcedure,
			HInstance:     win.GetModuleHandle(nil),
			HbrBackground: win.HBRUSH(brush),
			LpszClassName: className,
		}
		if win.RegisterClassEx(&windowClass) == 0 {
			windowsScrollingBorderClassErr = fmt.Errorf("register scrolling capture border class: %w", syscall.GetLastError())
		}
	})
	return windowsScrollingBorderClassErr
}

func windowsScrollingBorderWindowProc(hwnd win.HWND, message uint32, wParam, lParam uintptr) uintptr {
	if message == win.WM_NCHITTEST {
		result := int32(win.HTTRANSPARENT)
		return uintptr(result)
	}
	return win.DefWindowProc(hwnd, message, wParam, lParam)
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
