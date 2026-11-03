package host

import (
	"context"
	"encoding/json"
	"errors"
	"wox/plugin"
	"wox/util"
)

// handleOnHotkey registers the plugin-scoped event endpoint; hosts acknowledge dispatch, not completion.
func (w *WebsocketHost) handleOnHotkey(ctx context.Context, instance *plugin.Instance, request JsonRpcRequest) (any, error) {
	var option struct{ CallbackID string }
	if err := json.Unmarshal([]byte(request.Params["option"]), &option); err != nil {
		return nil, err
	}
	if option.CallbackID == "" {
		return nil, errors.New("hotkey callback ID is required")
	}
	return instance.API.OnHotkey(ctx, plugin.OnHotkeyOption{Callback: func(eventCtx context.Context, event plugin.HotkeyEvent) {
		data, _ := json.Marshal(event)
		if _, err := w.invokeMethod(eventCtx, instance.Metadata, "hotkeyEvent", map[string]string{"CallbackID": option.CallbackID, "Event": string(data)}); err != nil {
			util.GetLogger().Warn(eventCtx, "hotkey event delivery failed: "+err.Error())
		}
	}}), nil
}
