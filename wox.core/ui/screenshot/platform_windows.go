//go:build windows

package screenshot

import (
	"context"
	"errors"
	"fmt"
	"image"
	"time"

	"github.com/lxn/win"
	woxui "wox/ui/runtime"
	"wox/util"
)

func captureScreenshotPlatform(options ScreenshotOptions) (ScreenshotResult, error) {
	startedAt := time.Now()
	if options.ExportFilePath == "" {
		return ScreenshotResult{}, errors.New("screenshot export file path is empty")
	}
	var capturedCursor win.POINT
	hasCapturedCursor := win.GetCursorPos(&capturedCursor)
	type desktopCapture struct {
		source   image.Image
		bounds   image.Rectangle
		duration time.Duration
		err      error
	}
	captureDone := make(chan desktopCapture, 1)
	go func() {
		captureStartedAt := time.Now()
		woxui.FlushWindowsDesktopComposition()
		source, bounds, err := woxui.CaptureWindowsVirtualDesktop()
		captureDone <- desktopCapture{source: source, bounds: bounds, duration: time.Since(captureStartedAt), err: err}
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
	if prepareErr != nil {
		return ScreenshotResult{}, prepareErr
	}
	source, virtualBounds := captured.source, captured.bounds
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
		captureDesktop: func() (image.Image, error) {
			captured, _, captureErr := woxui.CaptureWindowsVirtualDesktop()
			return captured, captureErr
		},
		preparedWindow: preparedWindow,
		windowHost:     windowHost,
		afterShow: func() {
			util.GetLogger().Debug(context.Background(), fmt.Sprintf(
				"screenshot overlay ready: total=%s capture=%s renderer=%s desktop=%dx%d",
				time.Since(startedAt).Round(time.Millisecond), captured.duration.Round(time.Millisecond), prepareDuration.Round(time.Millisecond), virtualBounds.Dx(), virtualBounds.Dy(),
			))
		},
	}
	if hasCapturedCursor {
		platform.cursorPixel = screenshotEditorCursorPixelFromDesktop(
			Point{X: float32(capturedCursor.X), Y: float32(capturedCursor.Y)},
			Rect{X: float32(virtualBounds.Min.X), Y: float32(virtualBounds.Min.Y), Width: float32(virtualBounds.Dx()), Height: float32(virtualBounds.Dy())},
			source,
		)
	}
	return runScreenshotEditor(options, source, platform)
}
