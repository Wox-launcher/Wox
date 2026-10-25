package host

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"wox/plugin"
	"wox/setting/definition"
	"wox/util"
)

// handleOnPluginSettingChanged adapts the OnPluginSettingChanged wire call to the plugin API.
func (w *WebsocketHost) handleOnPluginSettingChanged(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	callbackId, err := requirePluginParam(request.Params, "callbackId")
	if err != nil {
		return nil, err
	}
	metadata := pluginInstance.Metadata
	pluginInstance.API.OnSettingChanged(ctx, func(callbackCtx context.Context, key string, value string) {
		w.invokeMethod(callbackCtx, metadata, "onPluginSettingChange", map[string]string{
			"CallbackId": callbackId,
			"Key":        key,
			"Value":      value,
		})
	})
	return "", nil
}

// handleOnGetDynamicSetting adapts the OnGetDynamicSetting wire call to the plugin API.
func (w *WebsocketHost) handleOnGetDynamicSetting(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	callbackId, err := requirePluginParam(request.Params, "callbackId")
	if err != nil {
		return nil, err
	}

	metadata := pluginInstance.Metadata
	pluginInstance.API.OnGetDynamicSetting(ctx, func(callbackCtx context.Context, key string) definition.PluginSettingDefinitionItem {
		result, err := w.invokeMethod(callbackCtx, metadata, "onGetDynamicSetting", map[string]string{
			"CallbackId": callbackId,
			"Key":        key,
		})
		if err != nil {
			util.GetLogger().Error(callbackCtx, fmt.Sprintf("[%s] failed to get dynamic setting: %s", request.PluginName, err))
			return definition.PluginSettingDefinitionItem{
				Type: definition.PluginSettingDefinitionTypeLabel,
				Value: &definition.PluginSettingValueLabel{
					Content: fmt.Sprintf("failed to get dynamic setting: %s", err),
				},
			}
		}
		if isEmptyDynamicSettingHostResult(result) {
			return definition.PluginSettingDefinitionItem{}
		}

		var setting definition.PluginSettingDefinitionItem
		decodeErr := w.decodeHostResult(result, &setting)
		if decodeErr != nil {
			util.GetLogger().Error(callbackCtx, fmt.Sprintf("[%s] failed to decode dynamic setting: %s", request.PluginName, decodeErr))
			return definition.PluginSettingDefinitionItem{
				Type: definition.PluginSettingDefinitionTypeLabel,
				Value: &definition.PluginSettingValueLabel{
					Content: fmt.Sprintf("failed to decode dynamic setting: %s", decodeErr),
				},
			}
		}

		return setting
	})
	return "", nil
}

// handleOnDeepLink adapts the OnDeepLink wire call to the plugin API.
func (w *WebsocketHost) handleOnDeepLink(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	callbackId, err := requirePluginParam(request.Params, "callbackId")
	if err != nil {
		return nil, err
	}

	metadata := pluginInstance.Metadata
	pluginInstance.API.OnDeepLink(ctx, func(callbackCtx context.Context, arguments map[string]string) {
		args, marshalErr := json.Marshal(arguments)
		if marshalErr != nil {
			util.GetLogger().Error(callbackCtx, fmt.Sprintf("[%s] failed to marshal deep link arguments: %s", request.PluginName, marshalErr))
			return
		}

		w.invokeMethod(callbackCtx, metadata, "onDeepLink", map[string]string{
			"CallbackId": callbackId,
			"Arguments":  string(args),
		})
	})
	return "", nil
}

// handleOnUnload adapts the OnUnload wire call to the plugin API.
func (w *WebsocketHost) handleOnUnload(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	callbackId, err := requirePluginParam(request.Params, "callbackId")
	if err != nil {
		return nil, err
	}

	metadata := pluginInstance.Metadata
	pluginInstance.API.OnUnload(ctx, func(callbackCtx context.Context) {
		w.invokeMethod(callbackCtx, metadata, "onUnload", map[string]string{
			"CallbackId": callbackId,
		})
	})
	return "", nil
}

// handleOnEnterPluginQuery adapts the OnEnterPluginQuery wire call to the plugin API.
func (w *WebsocketHost) handleOnEnterPluginQuery(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	callbackId, err := requirePluginParam(request.Params, "callbackId")
	if err != nil {
		return nil, err
	}

	metadata := pluginInstance.Metadata
	pluginInstance.API.OnEnterPluginQuery(ctx, func(callbackCtx context.Context) {
		w.invokeMethod(callbackCtx, metadata, "onEnterPluginQuery", map[string]string{
			"CallbackId": callbackId,
		})
	})
	return "", nil
}

// handleOnLeavePluginQuery adapts the OnLeavePluginQuery wire call to the plugin API.
func (w *WebsocketHost) handleOnLeavePluginQuery(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	callbackId, err := requirePluginParam(request.Params, "callbackId")
	if err != nil {
		return nil, err
	}

	metadata := pluginInstance.Metadata
	pluginInstance.API.OnLeavePluginQuery(ctx, func(callbackCtx context.Context) {
		w.invokeMethod(callbackCtx, metadata, "onLeavePluginQuery", map[string]string{
			"CallbackId": callbackId,
		})
	})
	return "", nil
}

// handleOnDragOut adapts the OnDragOut wire call to the plugin API.
func (w *WebsocketHost) handleOnDragOut(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	callbackId, err := requirePluginParam(request.Params, "callbackId")
	if err != nil {
		return nil, err
	}

	metadata := pluginInstance.Metadata
	result := pluginInstance.API.OnDragOut(ctx, plugin.DragOutListenOption{
		Callback: func(callbackCtx context.Context, event plugin.DragOutEvent) {
			eventJSON, marshalErr := json.Marshal(event)
			if marshalErr != nil {
				util.GetLogger().Error(callbackCtx, fmt.Sprintf("[%s] failed to marshal drag out event: %s", request.PluginName, marshalErr))
				return
			}
			w.invokeMethod(callbackCtx, metadata, "onDragOut", map[string]string{
				"CallbackId": callbackId,
				"Event":      string(eventJSON),
			})
		},
	})
	return result, nil
}

// handleOnMRURestore adapts the OnMRURestore wire call to the plugin API.
func (w *WebsocketHost) handleOnMRURestore(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	callbackId, err := requirePluginParam(request.Params, "callbackId")
	if err != nil {
		return nil, err
	}

	metadata := pluginInstance.Metadata
	pluginInstance.API.OnMRURestore(ctx, func(callbackCtx context.Context, mruData plugin.MRUData) (*plugin.QueryResult, error) {
		mruDataJson, marshalErr := json.Marshal(mruData)
		if marshalErr != nil {
			util.GetLogger().Error(callbackCtx, fmt.Sprintf("[%s] failed to marshal MRU data: %s", request.PluginName, marshalErr))
			return nil, marshalErr
		}

		result, invokeErr := w.invokeMethod(callbackCtx, metadata, "onMRURestore", map[string]string{
			"CallbackId": callbackId,
			"MRUData":    string(mruDataJson),
		})
		if invokeErr != nil {
			util.GetLogger().Error(callbackCtx, fmt.Sprintf("[%s] failed to invoke MRU restore callback: %s", request.PluginName, invokeErr))
			return nil, invokeErr
		}

		if result == nil {
			return nil, nil
		}

		var queryResult plugin.QueryResult
		decodeErr := w.decodeHostResult(result, &queryResult)
		if decodeErr != nil {
			util.GetLogger().Error(callbackCtx, fmt.Sprintf("[%s] failed to decode MRU restore result: %s", request.PluginName, decodeErr))
			return nil, decodeErr
		}

		return &queryResult, nil
	})
	return "", nil
}

// isEmptyDynamicSettingHostResult maps host callback placeholders such as None,
// null, "", or {} to the same hidden dynamic setting contract used by Go plugins.
func isEmptyDynamicSettingHostResult(result any) bool {
	if result == nil {
		return true
	}

	var raw []byte
	switch typed := result.(type) {
	case string:
		raw = []byte(typed)
	default:
		marshalResult, marshalErr := json.Marshal(typed)
		if marshalErr != nil {
			return false
		}
		raw = marshalResult
	}

	trimmed := strings.TrimSpace(string(raw))
	return trimmed == "" || trimmed == "null" || trimmed == "{}"
}
