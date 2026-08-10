//go:build windows

package screenshot

import (
	"context"
	"errors"
	"fmt"
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
	if hasCapturedCursor {
		platform.cursorPixel = screenshotEditorCursorPixelFromDesktop(
			Point{X: float32(capturedCursor.X), Y: float32(capturedCursor.Y)},
			Rect{X: float32(virtualBounds.Min.X), Y: float32(virtualBounds.Min.Y), Width: float32(virtualBounds.Dx()), Height: float32(virtualBounds.Dy())},
			source,
		)
	}
	return runScreenshotEditor(options, source, platform)
}
