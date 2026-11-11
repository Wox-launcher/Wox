//go:build linux

package launcher

import (
	"fmt"
	"os"
	"testing"
	"time"

	woxui "wox/ui/runtime"
)

// TestLauncherToolbarResizeSupersedesPendingShrink keeps a restored footer from
// being lost when GTK still reports the size preceding a queued shrink.
func TestLauncherToolbarResizeSupersedesPendingShrink(t *testing.T) {
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		t.Skip("native Linux display is unavailable")
	}
	app := New(false, nil)
	defer app.cancel()
	app.uiCall = nil
	app.show.WindowWidth = 750
	app.querySession.editor = woxui.NewTextEditor("wpm uninstall fixture")
	app.appearance.palette = defaultPalette()
	result := make(chan error, 1)
	var resizeErr error
	resized := false
	err := woxui.Run(func() error {
		window, err := woxui.Open(woxui.WindowOptions{
			Title: "Wox toolbar resize regression",
			Size:  woxui.Size{Width: 750, Height: 115},
			OnFrame: func(_ *woxui.DisplayList, _ woxui.FrameInfo) {
				if resized {
					return
				}
				resized = true
				// Resize during rendering, before GTK delivers ConfigureNotify.
				for _, visible := range []bool{true, false, true} {
					app.toolbar.toolbarMsg = nil
					if visible {
						app.toolbar.toolbarMsg = &toolbarMessage{ID: "uninstall", Title: "Plugin uninstalled"}
					}
					if err := app.applyWindowBounds(); err != nil {
						resizeErr = err
						return
					}
				}
			},
		})
		if err != nil {
			return err
		}
		app.window = window
		if _, err := window.Show(); err != nil {
			return err
		}
		go func() {
			time.Sleep(200 * time.Millisecond)
			var boundsErr error
			callErr := woxui.Call(func() {
				defer window.Close()
				if !resized || resizeErr != nil {
					boundsErr = fmt.Errorf("resize callback ran=%t: %v", resized, resizeErr)
					return
				}
				bounds, err := window.Bounds()
				if err != nil {
					boundsErr = err
				} else if bounds.Width != 750 || bounds.Height != 115 {
					boundsErr = fmt.Errorf("restored toolbar bounds = %+v, want 750x115 logical units", bounds)
				}
			})
			if callErr != nil {
				boundsErr = callErr
			}
			result <- boundsErr
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
