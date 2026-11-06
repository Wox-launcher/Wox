package pluginwindow

import (
	"encoding/json"
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

// TestNativeWindowMessages runs the actual WebView2 document and native message bridge.
func TestNativeWindowMessages(t *testing.T) {
	if os.Getenv("WOX_WINDOW_NATIVE_TEST") != "1" {
		t.Skip("set WOX_WINDOW_NATIVE_TEST=1 for native WebView integration")
	}
	result := make(chan error, 1)
	err := ui.Run(func() error {
		overlay.SetWindowManager(ui.NewWindowManager())
		events := make(chan plugin.WindowEvent, 64)
		html := `<!doctype html><html><body><button style="position:absolute;left:8px;top:8px;width:60px;height:30px" onclick="woxWindow.postMessage({click:true})">Close</button><div id="value"></div><script>let count=0;woxWindow.onMessage(data=>{document.getElementById('value').textContent=JSON.stringify(data);woxWindow.postMessage({count:++count,data,own:Object.hasOwn(data,'__proto__')})});</script></body></html>`
		custom := true
		driver, err := Create("native-test", plugin.CreateWindowOption{Content: plugin.WindowContent{Type: plugin.WindowContentHTML, Data: html}, Window: plugin.WindowOptions{HideChrome: &custom, Movable: &custom, DragContent: &custom}}, func(e plugin.WindowEvent) { events <- e })
		if err != nil {
			return err
		}
		// Loading cannot complete while this UI callback runs, so queue bounds are deterministic.
		for range 64 {
			if err := driver.PostMessage(plugin.WindowMessageOption{Data: json.RawMessage(`null`)}); err != nil {
				return err
			}
		}
		if driver.PostMessage(plugin.WindowMessageOption{Data: json.RawMessage(`null`)}) == nil {
			return fmt.Errorf("unbounded pending queue")
		}
		if err := driver.SetContent(plugin.WindowContent{Type: plugin.WindowContentHTML, Data: html}); err != nil {
			return err
		}
		otherEvents := make(chan plugin.WindowEvent, 16)
		other, err := Create("native-test-other", plugin.CreateWindowOption{Content: plugin.WindowContent{Type: plugin.WindowContentHTML, Data: html}}, func(e plugin.WindowEvent) { otherEvents <- e })
		if err != nil {
			_ = driver.Close()
			return err
		}
		go func() {
			defer driver.Close()
			defer other.Close()
			err := exerciseNativeWindow(driver, events, html)
			if err == nil {
				err = exerciseNativeWindow(other, otherEvents, html)
			}
			result <- err
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

// exerciseNativeWindow verifies retained DOM state and replacement without exposing script execution.
func exerciseNativeWindow(driver plugin.WindowDriver, events <-chan plugin.WindowEvent, html string) error {
	wait := func(kind plugin.WindowEventType) (plugin.WindowEvent, error) {
		deadline := time.NewTimer(20 * time.Second)
		defer deadline.Stop()
		for {
			select {
			case event := <-events:
				if event.Type == plugin.WindowEventError {
					return event, fmt.Errorf("page error: %s", event.ErrMsg)
				}
				if event.Type == kind {
					return event, nil
				}
			case <-deadline.C:
				return plugin.WindowEvent{}, fmt.Errorf("timed out waiting for %s", kind)
			}
		}
	}
	data := json.RawMessage(`{"snake_case":false,"__proto__":{"x":1},"unicode":"你好"}`)
	if err := driver.PostMessage(plugin.WindowMessageOption{Data: data}); err != nil {
		return err
	}
	if err := driver.Show(plugin.WindowShowOption{}); err != nil {
		return err
	}
	first, err := wait(plugin.WindowEventReady)
	if err != nil {
		return err
	}

	for expected := 1; expected <= 2; expected++ {
		if expected == 2 {
			if err := driver.Hide(); err != nil {
				return err
			}
			if err := driver.PostMessage(plugin.WindowMessageOption{Data: data}); err != nil {
				return err
			}
			if err := driver.Show(plugin.WindowShowOption{}); err != nil {
				return err
			}
		}
		event, err := wait(plugin.WindowEventMessage)
		if err != nil {
			return err
		}
		var echo struct {
			Count int
			Own   bool
			Data  json.RawMessage
		}
		if json.Unmarshal(event.Data, &echo) != nil || echo.Count != expected || !echo.Own {
			return fmt.Errorf("invalid echo: %s", event.Data)
		}
		var value map[string]any
		_ = json.Unmarshal(echo.Data, &value)
		if value["snake_case"] != false || value["unicode"] != "你好" {
			return fmt.Errorf("payload changed: %s", event.Data)
		}
	}
	if s, ok := driver.(*session); ok && boolValue(s.options.DragContent, false) {
		// The browser must still receive an ordinary click when the content is draggable.
		if err := ui.Call(func() {
			position := ui.Point{X: 15, Y: 15}
			s.host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: position})
			s.host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Position: position, Button: ui.PointerButtonPrimary})
			s.host.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Position: position, Button: ui.PointerButtonPrimary})
		}); err != nil {
			return err
		}
		event, err := wait(plugin.WindowEventMessage)
		if err != nil {
			return err
		}
		var click struct{ Click bool }
		if json.Unmarshal(event.Data, &click) != nil || !click.Click {
			return fmt.Errorf("HTML click was not delivered: %s", event.Data)
		}
	}

	if err := driver.SetContent(plugin.WindowContent{Type: plugin.WindowContentHTML, Data: html}); err != nil {
		return err
	}
	second, err := wait(plugin.WindowEventReady)
	if err != nil {
		return err
	}
	if first.DocumentID == second.DocumentID || driver.AcceptEvent(first) {
		return fmt.Errorf("stale document still accepted")
	}
	if driver.PostMessage(plugin.WindowMessageOption{Data: data, DocumentID: first.DocumentID}) == nil {
		return fmt.Errorf("accepted message for replaced document")
	}
	if err := driver.PostMessage(plugin.WindowMessageOption{Data: data}); err != nil {
		return err
	}
	event, err := wait(plugin.WindowEventMessage)
	if err != nil {
		return err
	}
	var echo struct{ Count int }
	_ = json.Unmarshal(event.Data, &echo)
	if echo.Count != 1 {
		return fmt.Errorf("content replacement retained old DOM")
	}
	for _, kind := range []plugin.WindowContentType{plugin.WindowContentMarkdown, plugin.WindowContentText} {
		if err := driver.SetContent(plugin.WindowContent{Type: kind, Data: "# Preview content"}); err != nil {
			return err
		}
		if _, err := wait(plugin.WindowEventReady); err != nil {
			return err
		}
		if driver.PostMessage(plugin.WindowMessageOption{Data: data}) == nil {
			return fmt.Errorf("non-HTML content accepted message")
		}
	}
	return nil
}
