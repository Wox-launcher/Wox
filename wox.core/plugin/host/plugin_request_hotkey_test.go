package host

import (
	"context"
	"encoding/json"
	"testing"
	"time"
	"wox/plugin"
)

type hotkeyRequestAPI struct {
	plugin.API
	callback func(context.Context, plugin.HotkeyEvent)
}

func (a *hotkeyRequestAPI) OnHotkey(_ context.Context, option plugin.OnHotkeyOption) plugin.OnHotkeyResult {
	a.callback = option.Callback
	return plugin.OnHotkeyResult{Success: true}
}

// TestHotkeyRequestRoundTrip covers callback ownership and reverse event serialization over WebSocket.
func TestHotkeyRequestRoundTrip(t *testing.T) {
	host, responses, callbacks := newPluginRequestPeer(t)
	api := &hotkeyRequestAPI{}
	instance := &plugin.Instance{API: api, Metadata: plugin.Metadata{Id: "hotkey-owner"}}
	request := JsonRpcRequest{Id: "subscribe", Method: "OnHotkey", Params: map[string]string{"option": `{"CallbackID":"listener"}`}}
	host.respondToPluginRequest(t.Context(), request, instance)
	response := receivePluginResponse(t, responses, request)
	if response.Error != "" || api.callback == nil {
		t.Fatalf("subscription: %+v", response)
	}
	expected := plugin.HotkeyEvent{SettingKey: "actions", ItemID: "stable-id", ColumnKey: "hotkey", Kind: plugin.HotkeyKindHoldModifier, Phase: plugin.HotkeyPhaseEnded, ActivationID: "activation", EndReason: plugin.HotkeyEndReleased, Timestamp: 123}
	done := make(chan struct{})
	go func() { defer close(done); api.callback(t.Context(), expected) }()
	select {
	case request := <-callbacks:
		if request.PluginId != "hotkey-owner" || request.Method != "hotkeyEvent" || request.Params["CallbackID"] != "listener" {
			t.Fatalf("event owner: %+v", request)
		}
		var event plugin.HotkeyEvent
		if err := json.Unmarshal([]byte(request.Params["Event"]), &event); err != nil || event != expected {
			t.Fatalf("event=%+v err=%v", event, err)
		}
	case <-time.After(time.Second):
		t.Fatal("missing event")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("missing event acknowledgement")
	}
	request.Id = "invalid"
	request.Params["option"] = `{}`
	host.respondToPluginRequest(t.Context(), request, instance)
	if response := receivePluginResponse(t, responses, request); response.Error == "" {
		t.Fatal("missing CallbackID accepted")
	}
}
