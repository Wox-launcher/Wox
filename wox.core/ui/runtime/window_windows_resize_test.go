//go:build windows

package woxui

import (
	"fmt"
	"os"
	"testing"
	"unsafe"

	"github.com/lxn/win"
)

func TestWindowsResizeNeedsPreparedFrame(t *testing.T) {
	tests := []struct {
		name                string
		width, height       int
		preparedFrameNeeded bool
	}{
		{name: "same size", width: 640, height: 360},
		{name: "grow height", width: 640, height: 480, preparedFrameNeeded: true},
		{name: "grow width", width: 800, height: 360, preparedFrameNeeded: true},
		{name: "grow both", width: 800, height: 480, preparedFrameNeeded: true},
		{name: "shrink height", width: 640, height: 240},
		{name: "shrink width", width: 480, height: 360},
		{name: "mixed dimensions", width: 800, height: 240},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := windowsResizeNeedsPreparedFrame(640, 360, test.width, test.height); got != test.preparedFrameNeeded {
				t.Fatalf("prepared frame needed = %t, want %t", got, test.preparedFrameNeeded)
			}
		})
	}
}

// TestWindowsInteractiveResizeFrame exercises native sizing without the programmatic resize helper.
func TestWindowsInteractiveResizeFrame(t *testing.T) {
	if os.Getenv("WOX_WINDOWS_RESIZE_INTEGRATION") != "1" {
		t.Skip("set WOX_WINDOWS_RESIZE_INTEGRATION=1 to run the native resize test")
	}
	err := Run(func() error {
		var lastFrame FrameInfo
		window, err := Open(WindowOptions{
			Title: "Wox resize test", Size: Size{Width: 320, Height: 240}, Resizable: true,
			OnFrame: func(list *DisplayList, frame FrameInfo) {
				lastFrame = frame
				list.FillRect(Rect{Width: frame.Size.Width, Height: frame.Size.Height}, Color{A: 255})
			},
		})
		if err != nil {
			return err
		}
		defer window.Close()
		if _, err := window.Show(); err != nil {
			return err
		}
		for _, scale := range []float32{1, 1.5, 2} {
			window.native.scale = scale
			var client win.RECT
			win.GetClientRect(window.native.hwnd, &client)
			width, height := client.Right+120, client.Bottom+90
			// Negative origins must not affect the physical dimensions passed to rendering.
			target := win.RECT{Left: -1000, Top: -800, Right: -1000 + width, Bottom: -800 + height}
			win.SendMessage(window.native.hwnd, windowsWMSizing, 8, uintptr(unsafe.Pointer(&target)))
			if lastFrame.PixelSize != (PixelSize{Width: int(width), Height: int(height)}) || lastFrame.Size.Width != float32(width)/scale {
				return fmt.Errorf("resize was not prepared at scale %v: %+v", scale, lastFrame)
			}
			// Commit growth, then shrink through native messages rather than SetBounds.
			for _, size := range []PixelSize{{Width: int(width), Height: int(height)}, {Width: int(width - 60), Height: int(height - 40)}} {
				if !win.SetWindowPos(window.native.hwnd, 0, 0, 0, int32(size.Width), int32(size.Height), win.SWP_NOMOVE|win.SWP_NOZORDER|win.SWP_NOACTIVATE) {
					return fmt.Errorf("native resize failed")
				}
				if lastFrame.PixelSize != size {
					return fmt.Errorf("native resize returned before painting %v: %+v", size, lastFrame)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
