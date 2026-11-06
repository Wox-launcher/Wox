package host

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
	"wox/plugin"
)

type windowRequestDriver struct {
	emit    func(plugin.WindowEvent)
	stop    sync.Once
	data    json.RawMessage
	update  string
	command plugin.WindowContentCommandOption
}

func (d *windowRequestDriver) Show(plugin.WindowShowOption) error { return nil }
func (d *windowRequestDriver) Hide() error                        { return nil }
func (d *windowRequestDriver) Close() error {
	d.stop.Do(func() { d.emit(plugin.WindowEvent{Type: plugin.WindowEventClosed}) })
	return nil
}
func (d *windowRequestDriver) SetOptions(plugin.WindowSetOptionsOption) error { return nil }
func (d *windowRequestDriver) SetContent(plugin.WindowContent) error          { return nil }
func (d *windowRequestDriver) UpdateContent(o plugin.WindowContentUpdateOption) error {
	d.update = o.Data
	return nil
}
func (d *windowRequestDriver) ContentCommand(o plugin.WindowContentCommandOption) error {
	d.command = o
	return nil
}
func (d *windowRequestDriver) PostMessage(o plugin.WindowMessageOption) error {
	d.data = o.Data
	return nil
}
func (d *windowRequestDriver) AcceptEvent(plugin.WindowEvent) bool { return true }

// TestWindowRequestRoundTrip verifies real WebSocket replies, reverse callbacks and owner isolation.
func TestWindowRequestRoundTrip(t *testing.T) {
	host, responses, callbacks := newPluginRequestPeer(t)
	var driver *windowRequestDriver
	plugin.SetWindowFactory(func(_ string, _ plugin.CreateWindowOption, emit func(plugin.WindowEvent), _ func(string) string) (plugin.WindowDriver, error) {
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
	driver.emit(plugin.WindowEvent{Type: plugin.WindowEventWoxView, DocumentID: "doc", Data: json.RawMessage(`{"Handler":"click","Args":[true]}`)})
	select {
	case event := <-callbacks:
		var payload plugin.WindowEvent
		if err := json.Unmarshal([]byte(event.Params["Event"]), &payload); err != nil || payload.Type != plugin.WindowEventWoxView || string(payload.Data) != `{"Handler":"click","Args":[true]}` {
			t.Fatalf("woxview event: %+v, %v", event, err)
		}
	case <-time.After(time.Second):
		t.Fatal("missing woxview callback")
	}
	other := &plugin.Instance{}
	for _, dark := range []bool{false, true} {
		data, _ := json.Marshal(plugin.WindowThemeChangedData{Dark: dark})
		driver.emit(plugin.WindowEvent{Type: plugin.WindowEventThemeChanged, DocumentID: "doc", Data: data})
		select {
		case event := <-callbacks:
			var payload plugin.WindowEvent
			if err := json.Unmarshal([]byte(event.Params["Event"]), &payload); err != nil || payload.Type != plugin.WindowEventThemeChanged || payload.DocumentID != "doc" || string(payload.Data) != string(data) || event.Params["CallbackID"] != "callback" {
				t.Fatalf("theme callback: %+v, %v", event, err)
			}
		case <-time.After(time.Second):
			t.Fatal("missing theme callback")
		}
	}
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
	command := JsonRpcRequest{Id: "command-denied", Method: "WindowContentCommand", Params: map[string]string{"windowID": created.WindowID, "option": `{"DocumentID":"doc","Command":"selectAll","Key":"title"}`}}
	host.respondToPluginRequest(t.Context(), command, other)
	denied = receivePluginResponse(t, responses, command)
	raw, _ = json.Marshal(denied.Result)
	_ = json.Unmarshal(raw, &result)
	if result.Success || driver.command.Key != "" {
		t.Fatal("foreign plugin executed a command")
	}
	host.respondToPluginRequest(t.Context(), command, instance)
	allowed = receivePluginResponse(t, responses, command)
	if allowed.Error != "" || driver.command != (plugin.WindowContentCommandOption{DocumentID: "doc", Command: plugin.WindowContentCommandSelectAll, Key: "title"}) {
		t.Fatalf("command route: %+v, %+v", allowed, driver.command)
	}
	for _, method := range []string{"WindowShow", "WindowHide", "WindowSetOptions", "WindowContentSet", "WindowContentUpdate", "WindowClose", "WindowClose"} {
		option := "{}"
		if method == "WindowContentUpdate" {
			option = `{"Data":"{\"version\":1}"}`
		}
		request = JsonRpcRequest{Id: method, Method: method, Params: map[string]string{"windowID": created.WindowID, "option": option}}
		host.respondToPluginRequest(t.Context(), request, instance)
		response = receivePluginResponse(t, responses, request)
		raw, _ = json.Marshal(response.Result)
		_ = json.Unmarshal(raw, &result)
		if response.Error != "" || !result.Success {
			t.Fatalf("%s: %+v", method, response)
		}
	}
	if driver.update != `{"version":1}` {
		t.Fatalf("update data = %q", driver.update)
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
