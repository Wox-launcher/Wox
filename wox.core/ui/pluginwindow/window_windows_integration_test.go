package pluginwindow

import (
	"fmt"
	"os"
	"testing"
	"time"
	"wox/plugin"
	ui "wox/ui/runtime"
	"wox/util/overlay"
)

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
