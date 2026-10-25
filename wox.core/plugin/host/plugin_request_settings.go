package host

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"wox/plugin"
)

// handleGetSetting adapts the GetSetting wire call to the plugin API.
func (w *WebsocketHost) handleGetSetting(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	key, err := requirePluginParam(request.Params, "key")
	if err != nil {
		return nil, err
	}

	result := pluginInstance.API.GetSetting(ctx, key)
	return result, nil
}

// handleSaveSetting adapts the SaveSetting wire call to the plugin API.
func (w *WebsocketHost) handleSaveSetting(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	key, err := requirePluginParam(request.Params, "key")
	if err != nil {
		return nil, err
	}
	value, err := requirePluginParam(request.Params, "value")
	if err != nil {
		return nil, err
	}
	isPlatformSpecificStr, err := requirePluginParam(request.Params, "isPlatformSpecific")
	if err != nil {
		return nil, err
	}
	isPlatformSpecific := strings.ToLower(isPlatformSpecificStr) == "true"

	pluginInstance.API.SaveSetting(ctx, key, value, isPlatformSpecific)
	return "", nil
}

// handleSetSetting adapts the SetSetting wire call to the plugin API.
func (w *WebsocketHost) handleSetSetting(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	optionJSON, exist := request.Params["option"]
	if !exist {
		return nil, fmt.Errorf("[%s] SetSetting method must have an option parameter", request.PluginName)
	}

	var setSettingOption plugin.SetSettingOption
	if err := json.Unmarshal([]byte(optionJSON), &setSettingOption); err != nil {
		return nil, fmt.Errorf("failed to unmarshal SetSetting option: %w", err)
	}

	result := pluginInstance.API.SetSetting(ctx, setSettingOption)
	return result, nil
}
