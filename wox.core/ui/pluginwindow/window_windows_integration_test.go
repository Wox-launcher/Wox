//go:build windows

package pluginwindow

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"syscall"
	"testing"
	"time"
	"wox/plugin"
	ui "wox/ui/runtime"
	"wox/util/overlay"

	"github.com/lxn/win"
)

// TestNativeMorphPath renders and updates a vector outline through the native compositor.
func TestNativeMorphPath(t *testing.T) {
	if os.Getenv("WOX_WINDOW_NATIVE_TEST") != "1" {
		t.Skip("set WOX_WINDOW_NATIVE_TEST=1 for native window integration")
	}
	result := make(chan error, 1)
	if err := ui.Run(func() error {
		overlay.SetWindowManager(ui.NewWindowManager())
		yes := true
		width, height := float64(120), float64(80)
		title := "Wox vector morph integration test"
		path := func(right float64) []map[string]any {
			return []map[string]any{{"op": "M", "values": []float64{0, 0}}, {"op": "C", "values": []float64{right, 0, right, 1, 0, 1}}, {"op": "Z"}}
		}
		document := func(progress float64) string {
			data, _ := json.Marshal(map[string]any{"version": 1, "root": map[string]any{"type": "morph-path", "key": "shape", "props": map[string]any{"keyframes": []any{map[string]any{"at": 0, "path": path(.2)}, map[string]any{"at": 1, "path": path(.8)}}, "progress": progress, "duration": 200, "style": map[string]any{"width": width, "height": height, "color": "#ff0000"}}}})
			return string(data)
		}
		driver, err := Create("morph-integration", plugin.CreateWindowOption{Window: plugin.WindowOptions{Title: title, Width: &width, Height: &height, Transparent: &yes, HideChrome: &yes}, Content: plugin.WindowContent{Type: plugin.WindowContentWoxView, Data: document(0)}}, func(plugin.WindowEvent) {})
		if err != nil {
			return err
		}
		go func() {
			defer driver.Close()
			if err := driver.Show(plugin.WindowShowOption{}); err != nil {
				result <- err
				return
			}
			name, _ := syscall.UTF16PtrFromString(title)
			check := func(wantRed bool) error {
				for deadline := time.Now().Add(4 * time.Second); time.Now().Before(deadline); {
					ui.FlushWindowsDesktopComposition()
					hwnd := win.FindWindow(nil, name)
					var bounds win.RECT
					if hwnd != 0 && win.GetWindowRect(hwnd, &bounds) {
						pixels, err := ui.CaptureWindowsWindow(uintptr(hwnd), image.Rect(int(bounds.Left), int(bounds.Top), int(bounds.Right), int(bounds.Bottom)))
						if err == nil {
							pixel := pixels.RGBAAt(pixels.Bounds().Dx()*2/5, pixels.Bounds().Dy()/2)
							if wantRed && pixel.R > 200 && pixel.A > 200 || !wantRed && pixel.A == 0 {
								return nil
							}
						}
					}
					time.Sleep(50 * time.Millisecond)
				}
				return fmt.Errorf("native vector did not render target pose (red=%v)", wantRed)
			}
			if err := check(false); err != nil {
				result <- err
				return
			}
			if err := driver.UpdateContent(plugin.WindowContentUpdateOption{Data: document(1), Sequence: 1}); err != nil {
				result <- err
				return
			}
			result <- check(true)
		}()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}

// TestNativeTransparentPluginWindow exercises the public option through the desktop compositor.
func TestNativeTransparentPluginWindow(t *testing.T) {
	if os.Getenv("WOX_WINDOW_NATIVE_TEST") != "1" {
		t.Skip("set WOX_WINDOW_NATIVE_TEST=1 for native window integration")
	}
	result := make(chan error, 1)
	if err := ui.Run(func() error {
		overlay.SetWindowManager(ui.NewWindowManager())
		yes := true
		width, height := float64(120), float64(80)
		title := "Wox transparent plugin integration test"
		tree := `{"version":1,"root":{"type":"stack","props":{"style":{"height":80}},"children":[{"type":"view","props":{"style":{"left":30,"top":20,"width":60,"height":40,"backgroundColor":"#ff0000"}}}]}}`
		driver, err := Create("transparent-integration", plugin.CreateWindowOption{Window: plugin.WindowOptions{Title: title, Width: &width, Height: &height, Transparent: &yes, HideChrome: &yes, Focusable: &yes}, Content: plugin.WindowContent{Type: plugin.WindowContentWoxView, Data: tree}}, func(plugin.WindowEvent) {})
		if err != nil {
			return err
		}
		go func() {
			defer driver.Close()
			if err := driver.Show(plugin.WindowShowOption{}); err != nil {
				result <- err
				return
			}
			name, _ := syscall.UTF16PtrFromString(title)
			for deadline := time.Now().Add(4 * time.Second); time.Now().Before(deadline); {
				ui.FlushWindowsDesktopComposition()
				hwnd := win.FindWindow(nil, name)
				var bounds win.RECT
				if hwnd != 0 && win.GetWindowRect(hwnd, &bounds) {
					pixels, err := ui.CaptureWindowsWindow(uintptr(hwnd), image.Rect(int(bounds.Left), int(bounds.Top), int(bounds.Right), int(bounds.Bottom)))
					if err == nil && pixels.RGBAAt(5, 5).A == 0 {
						center := pixels.RGBAAt(pixels.Bounds().Dx()/2, pixels.Bounds().Dy()/2)
						if center.R > 200 && center.A > 200 {
							result <- nil
							return
						}
					}
				}
				time.Sleep(50 * time.Millisecond)
			}
			result <- fmt.Errorf("transparent background or WoxView content did not reach the compositor")
		}()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}

// TestNativeWoxView exercises a native window, interaction, update and document isolation.
func TestNativeWoxView(t *testing.T) {
	if os.Getenv("WOX_WINDOW_NATIVE_TEST") != "1" {
		t.Skip("set WOX_WINDOW_NATIVE_TEST=1 for native window integration")
	}
	result := make(chan error, 1)
	err := ui.Run(func() error {
		overlay.SetWindowManager(ui.NewWindowManager())
		events := make(chan plugin.WindowEvent, 16)
		tree := `{"version":1,"root":{"type":"button","key":"go","props":{"label":"Go","onPress":{"$handler":"go"}}}}`
		driver, err := Create("native-woxview", plugin.CreateWindowOption{Content: plugin.WindowContent{Type: plugin.WindowContentWoxView, Data: tree}}, func(e plugin.WindowEvent) { events <- e })
		if err != nil {
			return err
		}
		go func() {
			defer driver.Close()
			ready := <-events
			if err := driver.Show(plugin.WindowShowOption{}); err != nil {
				result <- err
				return
			}
			s := driver.(*session)
			id := "plugin-window/" + ready.DocumentID + "/k:go/button"
			clicked := false
			for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline) && !clicked; {
				if err := ui.Call(func() {
					for _, node := range s.host.Snapshot().Tree.Nodes {
						if node.AutomationID == id {
							p := ui.Point{X: node.Bounds.X + node.Bounds.Width/2, Y: node.Bounds.Y + node.Bounds.Height/2}
							s.host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Position: p})
							s.host.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, Position: p})
							clicked = true
							break
						}
					}
				}); err != nil {
					result <- err
					return
				}
				if !clicked {
					time.Sleep(20 * time.Millisecond)
				}
			}
			if !clicked {
				result <- fmt.Errorf("button was not rendered")
				return
			}
			var clickedEvent plugin.WindowEvent
			select {
			case event := <-events:
				if event.Type != plugin.WindowEventWoxView || !driver.AcceptEvent(event) {
					result <- fmt.Errorf("invalid callback: %+v", event)
					return
				}
				clickedEvent = event
			case <-time.After(time.Second):
				result <- fmt.Errorf("missing callback")
				return
			}
			if err := driver.UpdateContent(plugin.WindowContentUpdateOption{Data: tree, DocumentID: ready.DocumentID, Sequence: 2}); err != nil {
				result <- err
				return
			}
			if driver.UpdateContent(plugin.WindowContentUpdateOption{Data: tree, DocumentID: ready.DocumentID, Sequence: 1}) == nil {
				result <- fmt.Errorf("accepted stale sequence")
				return
			}
			withoutHandler := `{"version":1,"root":{"type":"text","props":{"text":"done"}}}`
			if err := driver.UpdateContent(plugin.WindowContentUpdateOption{Data: withoutHandler, DocumentID: ready.DocumentID, Sequence: 3}); err != nil {
				result <- err
				return
			}
			if driver.AcceptEvent(clickedEvent) {
				result <- fmt.Errorf("accepted removed handler")
				return
			}
			if err := driver.SetContent(plugin.WindowContent{Type: plugin.WindowContentWoxView, Data: tree}); err != nil {
				result <- err
				return
			}
			if driver.UpdateContent(plugin.WindowContentUpdateOption{Data: tree, DocumentID: ready.DocumentID}) == nil {
				result <- fmt.Errorf("accepted stale document")
				return
			}
			result <- nil
		}()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("native test did not finish")
	}
}
