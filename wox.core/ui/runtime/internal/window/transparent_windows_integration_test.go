//go:build windows

package window

import (
	"fmt"
	"image"
	"os"
	"testing"
	"time"

	"github.com/lxn/win"
)

// TestNativeTransparentWindow checks actual compositor alpha, including after hide and show.
func TestNativeTransparentWindow(t *testing.T) {
	if os.Getenv("WOX_WINDOW_NATIVE_TEST") != "1" {
		t.Skip("set WOX_WINDOW_NATIVE_TEST=1 for native window integration")
	}
	result := make(chan error, 1)
	if err := Run(func() error {
		w, err := Open(WindowOptions{Title: "Wox transparent window test", Size: Size{Width: 120, Height: 80}, Transparent: true, Nonactivating: true, OnFrame: func(list *DisplayList, frame FrameInfo) {
			list.Clear(Color{})
			list.FillRoundedRect(Rect{X: 30, Y: 20, Width: 60, Height: 40}, 10, Color{R: 255, A: 255})
		}})
		if err != nil {
			return err
		}
		go func() {
			defer w.Close()
			for round := 0; round < 2; round++ {
				if _, err := w.Show(); err != nil {
					result <- err
					return
				}
				var captured bool
				for deadline := time.Now().Add(4 * time.Second); time.Now().Before(deadline); {
					FlushWindowsDesktopComposition()
					var bounds win.RECT
					if !win.GetWindowRect(w.native.hwnd, &bounds) {
						result <- fmt.Errorf("cannot read native window bounds")
						return
					}
					pixels, err := CaptureWindowsWindow(uintptr(w.native.hwnd), image.Rect(int(bounds.Left), int(bounds.Top), int(bounds.Right), int(bounds.Bottom)))
					if err == nil {
						corner := pixels.RGBAAt(pixels.Bounds().Min.X+5, pixels.Bounds().Min.Y+5)
						center := pixels.RGBAAt(pixels.Bounds().Dx()/2, pixels.Bounds().Dy()/2)
						if corner.A == 0 && center.R > 200 && center.A > 200 {
							captured = true
							break
						}
					}
					time.Sleep(50 * time.Millisecond)
				}
				if !captured {
					result <- fmt.Errorf("transparent corner and opaque content not captured on show %d", round+1)
					return
				}
				if err := w.Hide(); err != nil {
					result <- err
					return
				}
			}
			result <- nil
		}()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}
