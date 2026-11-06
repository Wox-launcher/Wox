//go:build windows

package window

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"testing"
	"time"

	"github.com/lxn/win"
)

// TestInitiallyHiddenRendererLifecycle exercises real delayed cleanup and the first presented
// frame after recreation, including size/font changes made while GPU resources are absent.
func TestInitiallyHiddenRendererLifecycle(t *testing.T) {
	if os.Getenv("WOX_WINDOW_NATIVE_TEST") != "1" {
		t.Skip("set WOX_WINDOW_NATIVE_TEST=1 for native window integration")
	}
	result := make(chan error, 1)
	err := Run(func() error {
		window, err := Open(WindowOptions{
			Title: "Wox hidden renderer lifecycle test", Size: Size{Width: 240, Height: 100}, Nonactivating: true,
			OnFrame: func(list *DisplayList, frame FrameInfo) {
				list.Clear(Color{R: 32, G: 160, B: 96, A: 255})
				list.DrawText("Restored renderer", Rect{X: 8, Y: 8, Width: frame.Size.Width - 16, Height: 24}, TextStyle{Size: 16}, Color{A: 255})
			},
		})
		if err != nil {
			return err
		}
		if window.native.rendererTrimTimer == nil {
			_ = window.Close()
			return fmt.Errorf("new hidden window did not schedule renderer cleanup")
		}
		go func() {
			defer window.Close()
			result <- exerciseHiddenRendererLifecycle(t, window)
		}()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}

// TestHiddenRendererTrimCancellation covers showing and closing before the initial timer fires.
func TestHiddenRendererTrimCancellation(t *testing.T) {
	if os.Getenv("WOX_WINDOW_NATIVE_TEST") != "1" {
		t.Skip("set WOX_WINDOW_NATIVE_TEST=1 for native window integration")
	}
	err := Run(func() error {
		window, err := Open(WindowOptions{Title: "Wox renderer timer cancellation test", Nonactivating: true})
		if err != nil {
			return err
		}
		defer window.Close()
		timer := window.native.rendererTrimTimer
		if timer == nil {
			return fmt.Errorf("new hidden window has no renderer timer")
		}
		if _, err := window.ShowInactive(); err != nil {
			return err
		}
		if window.native.rendererTrimTimer != nil || timer.Stop() {
			return fmt.Errorf("show left the hidden renderer timer running")
		}
		if err := window.Hide(); err != nil {
			return err
		}
		timer = window.native.rendererTrimTimer
		if timer == nil {
			return fmt.Errorf("hide did not schedule renderer cleanup")
		}
		if err := window.Close(); err != nil {
			return err
		}
		if window.native.rendererTrimTimer != nil || timer.Stop() {
			return fmt.Errorf("close left the hidden renderer timer running")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// exerciseHiddenRendererLifecycle repeats the production delay without forcing GC or native trims.
func exerciseHiddenRendererLifecycle(t *testing.T, window *Window) error {
	for cycle := 0; cycle < 3; cycle++ {
		time.Sleep(windowsRendererTrimDelay + time.Second)
		var checkErr error
		err := Call(func() {
			native := window.native
			if native.focus.visible || native.renderer.handle != nil || native.renderer.residentBytes() != 0 {
				checkErr = fmt.Errorf("cycle %d: hidden renderer retained resources", cycle)
				return
			}
			bounds, err := window.Bounds()
			if err != nil {
				checkErr = err
				return
			}
			bounds.Width += 20
			if err := window.SetBounds(bounds); err != nil {
				checkErr = err
				return
			}
			if err := window.SetFontFamily("Segoe UI"); err != nil {
				checkErr = err
				return
			}
			frames := window.FrameMetrics().PresentedFrameCount
			started := time.Now()
			if _, err := window.ShowInactive(); err != nil {
				checkErr = err
				return
			}
			t.Logf("cycle %d: restored and presented in %s at scale %.2f", cycle, time.Since(started), native.scale)
			if native.renderer.handle == nil || native.rendererTrimTimer != nil || !win.IsWindowVisible(native.hwnd) || window.FrameMetrics().PresentedFrameCount <= frames {
				checkErr = fmt.Errorf("cycle %d: show did not restore and present the renderer", cycle)
				return
			}
			metrics, err := window.MeasureText("Restored renderer", TextStyle{Size: 16})
			if err != nil || metrics.Size.Width <= 0 || native.renderer.fontFamily != "Segoe UI" {
				checkErr = fmt.Errorf("cycle %d: restored text measurement failed: %v", cycle, err)
				return
			}
			// A timer that already queued its command must not tear down a visible window.
			checkErr = native.executeCommand(windowCommand{kind: windowCommandTrimRenderer}).err
			if checkErr == nil && native.renderer.handle == nil {
				checkErr = fmt.Errorf("queued trim destroyed a visible renderer")
			}
		})
		if err != nil {
			return err
		}
		if checkErr != nil {
			return checkErr
		}
		FlushWindowsDesktopComposition()
		var bounds win.RECT
		if !win.GetWindowRect(window.native.hwnd, &bounds) {
			return fmt.Errorf("read restored window bounds")
		}
		pixels, err := CaptureWindowsWindow(uintptr(window.native.hwnd), image.Rect(int(bounds.Left), int(bounds.Top), int(bounds.Right), int(bounds.Bottom)))
		if err != nil {
			return err
		}
		// Sample actual captured pixels; HWND bounds and image dimensions need not share a scale.
		if got := pixels.RGBAAt(pixels.Bounds().Dx()/2, pixels.Bounds().Dy()*3/4); got != (color.RGBA{R: 32, G: 160, B: 96, A: 255}) {
			return fmt.Errorf("cycle %d: restored frame pixel = %v", cycle, got)
		}
		if err := window.Hide(); err != nil {
			return err
		}
	}
	return nil
}
