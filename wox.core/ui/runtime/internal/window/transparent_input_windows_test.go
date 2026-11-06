//go:build windows

package window

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/lxn/win"
)

func TestTransparentRegionData(t *testing.T) {
	pixels := image.NewRGBA(image.Rect(0, 0, 5, 3))
	pixels.SetRGBA(1, 1, color.RGBA{A: 255})
	pixels.SetRGBA(2, 1, color.RGBA{A: 255})
	pixels.SetRGBA(4, 2, color.RGBA{A: 255})
	data := transparentRegionData(pixels)
	if n := binary.LittleEndian.Uint32(data[8:12]); n != 2 {
		t.Fatalf("regions = %d", n)
	}
	want := []uint32{1, 1, 3, 2, 4, 2, 5, 3}
	for i, value := range want {
		if got := binary.LittleEndian.Uint32(data[32+i*4:]); got != value {
			t.Fatalf("coordinate %d: %d, want %d", i, got, value)
		}
	}
}

// TestNativeTransparentInputRegion checks actual Windows hit testing, not only the painted alpha.
func TestNativeTransparentInputRegion(t *testing.T) {
	if os.Getenv("WOX_WINDOW_NATIVE_TEST") != "1" {
		t.Skip("set WOX_WINDOW_NATIVE_TEST=1 for native window integration")
	}
	result := make(chan error, 1)
	var gapEnabled atomic.Bool
	var paintEnabled atomic.Bool
	if err := Run(func() error {
		w, err := Open(WindowOptions{Title: "Wox transparent input test", Size: Size{Width: 120, Height: 80}, Transparent: true, PassThroughTransparent: true, Topmost: true, OnFrame: func(list *DisplayList, _ FrameInfo) {
			list.Clear(Color{})
			if paintEnabled.Load() {
				list.FillRoundedRect(Rect{X: 30, Y: 20, Width: 60, Height: 40}, 10, Color{R: 255, A: 255})
			}
			if gapEnabled.Load() {
				list.AddInputRegion(Rect{X: 95, Y: 20, Width: 10, Height: 40})
			}
		}})
		if err != nil {
			return err
		}
		go func() {
			defer w.Close()
			if _, err := w.Show(); err != nil {
				result <- err
				return
			}
			// A fully empty first frame must still be able to grow into a painted input region.
			time.Sleep(100 * time.Millisecond)
			paintEnabled.Store(true)
			if err := w.Invalidate(); err != nil {
				result <- err
				return
			}
			windowFromPoint := syscall.NewLazyDLL("user32.dll").NewProc("WindowFromPoint")
			windowAt := func(p win.POINT) win.HWND {
				packed := uint64(uint32(p.X)) | uint64(uint32(p.Y))<<32
				hwnd, _, _ := windowFromPoint.Call(uintptr(packed))
				return win.HWND(hwnd)
			}
			check := func(gap bool) error {
				for deadline := time.Now().Add(4 * time.Second); time.Now().Before(deadline); {
					var bounds win.RECT
					if !win.GetWindowRect(w.native.hwnd, &bounds) {
						return fmt.Errorf("read window bounds")
					}
					corner := win.POINT{X: bounds.Left + 5, Y: bounds.Top + 5}
					center := win.POINT{X: (bounds.Left + bounds.Right) / 2, Y: (bounds.Top + bounds.Bottom) / 2}
					centerWindow, cornerWindow := windowAt(center), windowAt(corner)
					gapPoint := win.POINT{X: bounds.Left + (bounds.Right-bounds.Left)*100/120, Y: center.Y}
					if centerWindow == w.native.hwnd && cornerWindow != w.native.hwnd && (windowAt(gapPoint) == w.native.hwnd) == gap {
						return nil
					}
					time.Sleep(50 * time.Millisecond)
				}
				return fmt.Errorf("native input region did not follow painted pixels and explicit gap=%v", gap)
			}
			if err := check(false); err != nil {
				result <- err
				return
			}
			gapEnabled.Store(true)
			if err := w.Invalidate(); err != nil {
				result <- err
				return
			}
			if err := check(true); err != nil {
				result <- err
				return
			}
			var bounds win.RECT
			if !win.GetWindowRect(w.native.hwnd, &bounds) {
				result <- fmt.Errorf("read window bounds")
				return
			}
			pixels, err := CaptureWindowsWindow(uintptr(w.native.hwnd), image.Rect(int(bounds.Left), int(bounds.Top), int(bounds.Right), int(bounds.Bottom)))
			if err != nil {
				result <- err
				return
			}
			if alpha := pixels.RGBAAt(pixels.Bounds().Dx()*100/120, pixels.Bounds().Dy()/2).A; alpha != 0 {
				result <- fmt.Errorf("input-only region painted an alpha of %d", alpha)
				return
			}
			gapEnabled.Store(false)
			if err := w.Invalidate(); err != nil {
				result <- err
				return
			}
			result <- check(false)
		}()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}
