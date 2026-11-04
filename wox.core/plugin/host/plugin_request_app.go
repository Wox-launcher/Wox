package host

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"wox/common"
	"wox/plugin"
	"wox/util"
)

// handleHideApp adapts the HideApp wire call to the plugin API.
func (w *WebsocketHost) handleHideApp(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	pluginInstance.API.HideApp(ctx)
	return "", nil
}

// handleShowApp adapts the ShowApp wire call to the plugin API.
func (w *WebsocketHost) handleShowApp(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	pluginInstance.API.ShowApp(ctx)
	return "", nil
}

// handleIsVisible adapts the IsVisible wire call to the plugin API.
func (w *WebsocketHost) handleIsVisible(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	result := pluginInstance.API.IsVisible(ctx)
	return result, nil
}

// handleCopy adapts the Copy wire call to the plugin API.
func (w *WebsocketHost) handleCopy(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	var params plugin.CopyParams
	params.Type = plugin.CopyType(request.Params["type"])
	params.Text = request.Params["text"]

	if woxImageStr, exists := request.Params["woxImage"]; exists {
		// wox image is empty, skip unmarshalling
		if strings.TrimSpace(woxImageStr) == "" {
			pluginInstance.API.Copy(ctx, params)
			return "", nil
		}

		var woxImage common.WoxImage
		if err := json.Unmarshal([]byte(woxImageStr), &woxImage); err != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to unmarshal woxImage: %s", request.PluginName, err))
		} else {
			params.WoxImage = &woxImage
		}
	}

	pluginInstance.API.Copy(ctx, params)
	return "", nil
}

// handleScreenshot adapts the Screenshot wire call to the plugin API.
func (w *WebsocketHost) handleScreenshot(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	var option plugin.ScreenshotOption
	if optionStr, exists := request.Params["option"]; exists && strings.TrimSpace(optionStr) != "" {
		// The public plugin API keeps screenshot options in a single JSON object so new fields
		// can be added without growing the websocket method signature for every SDK runtime.
		if err := json.Unmarshal([]byte(optionStr), &option); err != nil {
			return nil, fmt.Errorf("failed to unmarshal screenshot option: %w", err)
		}
	}

	result := pluginInstance.API.Screenshot(ctx, option)
	return result, nil
}

// handleGetCacheFolder adapts the GetCacheFolder wire call to the plugin API.
func (w *WebsocketHost) handleGetCacheFolder(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	result := pluginInstance.API.GetCacheFolder(ctx)
	return result, nil
}

// handleGetThemeColors adapts the GetThemeColors wire call to the plugin API.
func (w *WebsocketHost) handleGetThemeColors(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	var option plugin.GetThemeColorsOption
	if optionStr, exists := request.Params["option"]; exists && strings.TrimSpace(optionStr) != "" {
		if err := json.Unmarshal([]byte(optionStr), &option); err != nil {
			return nil, fmt.Errorf("failed to unmarshal theme colors option: %w", err)
		}
	}
	result := pluginInstance.API.GetThemeColors(ctx, option)
	return result, nil
}

// handleNotify preserves the legacy wire method while always displaying a toast.
func (w *WebsocketHost) handleNotify(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	message, err := requirePluginParam(request.Params, "message")
	if err != nil {
		return nil, err
	}
	pluginInstance.API.ShowToastMsg(ctx, plugin.ShowToastMsgOption{Message: message})
	return "", nil
}

// handleShowToastMsg converts a host-owned callback ID into a one-shot proxy.
func (w *WebsocketHost) handleShowToastMsg(ctx context.Context, instance *plugin.Instance, request JsonRpcRequest) (any, error) {
	var payload struct {
		Message string
		Title   string
		Action  *struct {
			Name        string
			ContextData common.ContextData
			CallbackId  string
		}
	}
	if err := json.Unmarshal([]byte(request.Params["option"]), &payload); err != nil {
		return nil, fmt.Errorf("invalid toast option: %w", err)
	}
	option := plugin.ShowToastMsgOption{Message: payload.Message, Title: payload.Title}
	if payload.Action != nil {
		if payload.Action.CallbackId == "" {
			return plugin.ShowToastMsgResult{ErrMsg: "action callback is required"}, nil
		}
		id := payload.Action.CallbackId
		invoke := func(ctx context.Context, run bool, data common.ContextData) {
			_, err := w.invokeMethod(ctx, instance.Metadata, "toastMsgAction", map[string]string{"CallbackId": id, "Invoke": fmt.Sprint(run), "ContextData": data.Marshal()})
			if err != nil {
				util.GetLogger().Debug(ctx, "toast callback delivery failed: "+err.Error())
				if run {
					_, _ = w.invokeMethod(util.NewTraceContext(), instance.Metadata, "toastMsgAction", map[string]string{"CallbackId": id, "Invoke": "false"})
				}
			}
		}
		option.Action = &plugin.ToastMsgAction{Name: payload.Action.Name, ContextData: payload.Action.ContextData,
			Action:  func(ctx context.Context, action plugin.ToastMsgActionContext) { invoke(ctx, true, action.ContextData) },
			Release: func() { invoke(util.NewTraceContext(), false, nil) },
		}
	}
	return instance.API.ShowToastMsg(ctx, option), nil
}

// handlePushAttention adapts the PushAttention wire call to the plugin API.
func (w *WebsocketHost) handlePushAttention(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	rawRequest, err := requirePluginParam(request.Params, "request")
	if err != nil {
		return nil, err
	}

	var attentionRequest plugin.PushAttentionRequest
	if err := json.Unmarshal([]byte(rawRequest), &attentionRequest); err != nil {
		return nil, fmt.Errorf("failed to unmarshal push attention request: %w", err)
	}

	pluginInstance.API.PushAttention(ctx, attentionRequest)
	return "", nil
}

// handleShowToolbarMsg adapts the ShowToolbarMsg wire call to the plugin API.
func (w *WebsocketHost) handleShowToolbarMsg(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	rawMsg, err := requirePluginParam(request.Params, "msg")
	if err != nil {
		return nil, err
	}

	var msg plugin.ToolbarMsg
	if err := json.Unmarshal([]byte(rawMsg), &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal toolbar msg: %w", err)
	}

	pluginInstance.API.ShowToolbarMsg(ctx, msg)
	return "", nil
}

// handleClearToolbarMsg adapts the ClearToolbarMsg wire call to the plugin API.
func (w *WebsocketHost) handleClearToolbarMsg(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	toolbarMsgId, err := requirePluginParam(request.Params, "toolbarMsgId")
	if err != nil {
		return nil, err
	}
	pluginInstance.API.ClearToolbarMsg(ctx, toolbarMsgId)
	return "", nil
}

// handleLog adapts the Log wire call to the plugin API.
func (w *WebsocketHost) handleLog(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	msg, err := requirePluginParam(request.Params, "msg")
	if err != nil {
		return nil, err
	}
	level, err := requirePluginParam(request.Params, "level")
	if err != nil {
		return nil, err
	}

	pluginInstance.API.Log(ctx, level, msg)
	return "", nil
}

// handleGetTranslation adapts the GetTranslation wire call to the plugin API.
func (w *WebsocketHost) handleGetTranslation(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	key, err := requirePluginParam(request.Params, "key")
	if err != nil {
		return nil, err
	}
	result := pluginInstance.API.GetTranslation(ctx, key)
	return result, nil
}

// handleExecuteToolbarMsgAction adapts the ExecuteToolbarMsgAction wire call to the plugin API.
func (w *WebsocketHost) handleExecuteToolbarMsgAction(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	toolbarMsgId, err := requirePluginParam(request.Params, "toolbarMsgId")
	if err != nil {
		return nil, err
	}

	actionId, err := requirePluginParam(request.Params, "actionId")
	if err != nil {
		return nil, err
	}

	sessionId := util.GetContextSessionId(ctx)
	executeErr := plugin.GetPluginManager().ExecuteToolbarMsgAction(ctx, sessionId, toolbarMsgId, actionId)
	if executeErr != nil {
		return nil, executeErr
	}

	return "", nil
}
