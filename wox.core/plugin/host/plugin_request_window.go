package host

import (
	"context"
	"encoding/json"
	"errors"
	"wox/plugin"
	"wox/util"
)

// handleCreateWindow binds a host callback to the lifetime of the returned window capability.
func (w *WebsocketHost) handleCreateWindow(ctx context.Context, instance *plugin.Instance, request JsonRpcRequest) (any, error) {
	var payload struct {
		plugin.CreateWindowOption
		CallbackID string
	}
	if err := json.Unmarshal([]byte(request.Params["option"]), &payload); err != nil {
		return nil, err
	}
	if payload.CallbackID == "" {
		return nil, errors.New("window callback ID is required")
	}
	payload.OnEvent = func(eventCtx context.Context, event plugin.WindowEvent) {
		data, _ := json.Marshal(event)
		if _, err := w.invokeMethod(eventCtx, instance.Metadata, "windowEvent", map[string]string{"CallbackID": payload.CallbackID, "Event": string(data)}); err != nil {
			util.GetLogger().Debug(eventCtx, "window event delivery failed: "+err.Error())
		}
	}
	payload.Release = func() {
		_, _ = w.invokeMethod(util.NewTraceContext(), instance.Metadata, "windowEvent", map[string]string{"CallbackID": payload.CallbackID, "Release": "true"})
	}
	return instance.API.CreateWindow(ctx, payload.CreateWindowOption), nil
}

// handleWindowOperation resolves the capability only within the calling plugin instance.
func (w *WebsocketHost) handleWindowOperation(ctx context.Context, instance *plugin.Instance, request JsonRpcRequest) (any, error) {
	owner, ok := instance.API.(interface {
		PluginWindowByID(string) (*plugin.PluginWindow, bool)
	})
	if !ok {
		return nil, errors.New("window API unavailable")
	}
	window, found := owner.PluginWindowByID(request.Params["windowID"])
	if !found {
		if request.Method == "WindowClose" {
			return plugin.WindowCloseResult{WindowResult: plugin.WindowResult{Success: true}}, nil
		}
		return plugin.WindowResult{ErrMsg: "window not found"}, nil
	}
	raw := []byte(request.Params["option"])
	switch request.Method {
	case "WindowShow":
		var o plugin.WindowShowOption
		if err := json.Unmarshal(raw, &o); err != nil {
			return nil, err
		}
		return window.Show(ctx, o), nil
	case "WindowHide":
		var o plugin.WindowHideOption
		if err := json.Unmarshal(raw, &o); err != nil {
			return nil, err
		}
		return window.Hide(ctx, o), nil
	case "WindowClose":
		var o plugin.WindowCloseOption
		if err := json.Unmarshal(raw, &o); err != nil {
			return nil, err
		}
		return window.Close(ctx, o), nil
	case "WindowSetOptions":
		var o plugin.WindowSetOptionsOption
		if err := json.Unmarshal(raw, &o); err != nil {
			return nil, err
		}
		return window.SetOptions(ctx, o), nil
	case "WindowContentSet":
		var o plugin.WindowContentSetOption
		if err := json.Unmarshal(raw, &o); err != nil {
			return nil, err
		}
		return window.Content.Set(ctx, o), nil
	case "WindowPostMessage":
		var o plugin.WindowMessageOption
		if err := json.Unmarshal(raw, &o); err != nil {
			return nil, err
		}
		return window.Content.PostMessage(ctx, o), nil
	}
	return nil, errors.New("unknown window operation")
}
