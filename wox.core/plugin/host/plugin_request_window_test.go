package host

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
	"wox/plugin"
)

type windowRequestDriver struct {
	emit func(plugin.WindowEvent)
	stop sync.Once
	data json.RawMessage
}

func (d *windowRequestDriver) Show(plugin.WindowShowOption) error { return nil }
func (d *windowRequestDriver) Hide() error                        { return nil }
func (d *windowRequestDriver) Close() error {
	d.stop.Do(func() { d.emit(plugin.WindowEvent{Type: plugin.WindowEventClosed}) })
	return nil
}
func (d *windowRequestDriver) SetOptions(plugin.WindowSetOptionsOption) error { return nil }
func (d *windowRequestDriver) SetContent(plugin.WindowContent) error          { return nil }
func (d *windowRequestDriver) PostMessage(o plugin.WindowMessageOption) error {
	d.data = o.Data
	return nil
}
func (d *windowRequestDriver) AcceptEvent(plugin.WindowEvent) bool { return true }

// TestWindowRequestRoundTrip verifies real WebSocket replies, reverse callbacks and owner isolation.
func TestWindowRequestRoundTrip(t *testing.T) {
	host, responses, callbacks := newPluginRequestPeer(t)
	var driver *windowRequestDriver
	plugin.SetWindowFactory(func(_ string, _ plugin.CreateWindowOption, emit func(plugin.WindowEvent)) (plugin.WindowDriver, error) {
		driver = &windowRequestDriver{emit: emit}
		emit(plugin.WindowEvent{Type: plugin.WindowEventReady, DocumentID: "doc"})
		return driver, nil
	})
	t.Cleanup(func() { plugin.SetWindowFactory(nil) })
	instance := &plugin.Instance{}
	instance.Metadata.Id = "owner"
	instance.API = plugin.NewAPI(instance)
	request := JsonRpcRequest{Id: "create", Method: "CreateWindow", Params: map[string]string{"option": `{"CallbackID":"callback","Content":{"Type":"html","Data":"<p>test</p>"}}`}}
	host.respondToPluginRequest(t.Context(), request, instance)
	response := receivePluginResponse(t, responses, request)
	raw, _ := json.Marshal(response.Result)
	var created plugin.CreateWindowResult
	if json.Unmarshal(raw, &created) != nil || !created.Success || created.WindowID == "" {
		t.Fatalf("create response: %+v", response)
	}
	select {
	case event := <-callbacks:
		if event.Method != "windowEvent" || event.Params["CallbackID"] != "callback" {
			t.Fatalf("callback: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("missing ready callback")
	}
	other := &plugin.Instance{}
	other.API = plugin.NewAPI(other)
	operation := JsonRpcRequest{Id: "message", Method: "WindowPostMessage", Params: map[string]string{"windowID": created.WindowID, "option": `{"Data":{"snake_case":false}}`}}
	host.respondToPluginRequest(t.Context(), operation, other)
	denied := receivePluginResponse(t, responses, operation)
	raw, _ = json.Marshal(denied.Result)
	var result plugin.WindowResult
	_ = json.Unmarshal(raw, &result)
	if result.Success || driver.data != nil {
		t.Fatal("foreign plugin accessed window")
	}
	host.respondToPluginRequest(t.Context(), operation, instance)
	allowed := receivePluginResponse(t, responses, operation)
	if allowed.Error != "" || string(driver.data) != `{"snake_case":false}` {
		t.Fatalf("message response: %+v, data: %s", allowed, driver.data)
	}
	for _, method := range []string{"WindowShow", "WindowHide", "WindowSetOptions", "WindowContentSet", "WindowClose", "WindowClose"} {
		request = JsonRpcRequest{Id: method, Method: method, Params: map[string]string{"windowID": created.WindowID, "option": "{}"}}
		host.respondToPluginRequest(t.Context(), request, instance)
		response = receivePluginResponse(t, responses, request)
		raw, _ = json.Marshal(response.Result)
		_ = json.Unmarshal(raw, &result)
		if response.Error != "" || !result.Success {
			t.Fatalf("%s: %+v", method, response)
		}
	}
	for {
		select {
		case event := <-callbacks:
			if event.Params["Release"] == "true" {
				return
			}
		case <-time.After(time.Second):
			t.Fatal("closed callback not released")
		}
	}
}
