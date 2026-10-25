package host

import (
	"context"
	"encoding/json"
	"fmt"
	"wox/network"
	"wox/plugin"
	"wox/util"
)

// handleRequestFromPlugin resolves the caller before entering the shared response boundary.
func (w *WebsocketHost) handleRequestFromPlugin(ctx context.Context, request JsonRpcRequest) {
	var caller *plugin.Instance
	if !network.IsOffline() {
		for _, instance := range plugin.GetPluginManager().GetPluginInstances() {
			if instance.Metadata.Id == request.PluginId {
				caller = instance
				break
			}
		}
	}
	w.respondToPluginRequest(ctx, request, caller)
}

// respondToPluginRequest owns context propagation and exactly one response attempt per call.
func (w *WebsocketHost) respondToPluginRequest(ctx context.Context, request JsonRpcRequest, caller *plugin.Instance) {
	if request.SessionId != "" {
		ctx = util.WithSessionContext(ctx, request.SessionId)
	}
	if request.QueryId != "" {
		ctx = util.WithQueryIdContext(ctx, request.QueryId)
	}
	if request.Method != "Log" {
		util.GetLogger().Info(ctx, fmt.Sprintf("got request from plugin <%s>, method: %s", request.PluginName, request.Method))
	}
	result, err := w.dispatchPluginRequest(ctx, request, caller)
	w.sendPluginResponse(ctx, request, result, err)
}

// dispatchPluginRequest keeps failures inside the RPC boundary so a recovered API panic cannot strand a host waiter.
func (w *WebsocketHost) dispatchPluginRequest(ctx context.Context, request JsonRpcRequest, caller *plugin.Instance) (result any, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result, err = nil, fmt.Errorf("plugin API panic: %v", recovered)
		}
	}()
	if network.IsOffline() {
		return nil, network.ErrOffline
	}
	if caller == nil {
		return nil, fmt.Errorf("plugin instance %q not found", request.PluginId)
	}
	handler, ok := pluginRequestHandlers[request.Method]
	if !ok {
		return nil, fmt.Errorf("unknown plugin method %q", request.Method)
	}
	return handler(w, ctx, caller, request)
}

// requirePluginParam checks presence rather than emptiness to preserve existing SDK string semantics.
func requirePluginParam(params map[string]string, key string) (string, error) {
	value, ok := params[key]
	if !ok {
		return "", fmt.Errorf("missing required parameter %q", key)
	}
	return value, nil
}

// sendPluginResponse preserves the published envelope, including null error results and empty-string acknowledgements.
func (w *WebsocketHost) sendPluginResponse(ctx context.Context, request JsonRpcRequest, result any, responseErr error) {
	// A call may finish while offline shutdown is beginning. Still settle the local host waiter.
	if responseErr == nil && network.IsOffline() {
		responseErr = network.ErrOffline
	}
	response := JsonRpcResponse{Id: request.Id, Method: request.Method, Type: JsonRpcTypeResponse, Result: result}
	if responseErr != nil {
		response.Result, response.Error = nil, responseErr.Error()
	}
	data, err := json.Marshal(response)
	if err != nil {
		response.Result, response.Error = nil, "failed to marshal response: "+err.Error()
		// With Result removed, the envelope contains only strings and cannot fail to marshal.
		data, _ = json.Marshal(response)
	}
	if response.Error != "" {
		util.GetLogger().Error(ctx, fmt.Sprintf("[%s] %s: %s", request.PluginName, request.Method, response.Error))
	}
	client := w.websocketClient()
	if client == nil {
		return
	}
	if err := client.Send(ctx, data); err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to send %s response: %s", request.PluginName, request.Method, err))
	}
}

// pluginRequestHandler returns the wire result or a transport error; handlers never send replies.
type pluginRequestHandler func(*WebsocketHost, context.Context, *plugin.Instance, JsonRpcRequest) (any, error)

var pluginRequestHandlers = map[string]pluginRequestHandler{
	"HideApp":                  (*WebsocketHost).handleHideApp,
	"ShowApp":                  (*WebsocketHost).handleShowApp,
	"IsVisible":                (*WebsocketHost).handleIsVisible,
	"ChangeQuery":              (*WebsocketHost).handleChangeQuery,
	"RefreshQuery":             (*WebsocketHost).handleRefreshQuery,
	"Copy":                     (*WebsocketHost).handleCopy,
	"Screenshot":               (*WebsocketHost).handleScreenshot,
	"GetCacheFolder":           (*WebsocketHost).handleGetCacheFolder,
	"GetThemeColors":           (*WebsocketHost).handleGetThemeColors,
	"Notify":                   (*WebsocketHost).handleNotify,
	"PushAttention":            (*WebsocketHost).handlePushAttention,
	"ShowToolbarMsg":           (*WebsocketHost).handleShowToolbarMsg,
	"ClearToolbarMsg":          (*WebsocketHost).handleClearToolbarMsg,
	"Log":                      (*WebsocketHost).handleLog,
	"GetTranslation":           (*WebsocketHost).handleGetTranslation,
	"GetSetting":               (*WebsocketHost).handleGetSetting,
	"SaveSetting":              (*WebsocketHost).handleSaveSetting,
	"SetSetting":               (*WebsocketHost).handleSetSetting,
	"OnPluginSettingChanged":   (*WebsocketHost).handleOnPluginSettingChanged,
	"OnGetDynamicSetting":      (*WebsocketHost).handleOnGetDynamicSetting,
	"OnDeepLink":               (*WebsocketHost).handleOnDeepLink,
	"OnUnload":                 (*WebsocketHost).handleOnUnload,
	"OnEnterPluginQuery":       (*WebsocketHost).handleOnEnterPluginQuery,
	"OnLeavePluginQuery":       (*WebsocketHost).handleOnLeavePluginQuery,
	"OnDragOut":                (*WebsocketHost).handleOnDragOut,
	"OnMRURestore":             (*WebsocketHost).handleOnMRURestore,
	"RegisterQueryCommands":    (*WebsocketHost).handleRegisterQueryCommands,
	"RegisterTriggerKeyword":   (*WebsocketHost).handleRegisterTriggerKeyword,
	"UnregisterTriggerKeyword": (*WebsocketHost).handleUnregisterTriggerKeyword,
	"RegisterPluginTool":       (*WebsocketHost).handleRegisterPluginTool,
	"UnregisterPluginTool":     (*WebsocketHost).handleUnregisterPluginTool,
	"ListPluginTools":          (*WebsocketHost).handleListPluginTools,
	"InvokePluginTool":         (*WebsocketHost).handleInvokePluginTool,
	"GetUpdatableResult":       (*WebsocketHost).handleGetUpdatableResult,
	"UpdateResult":             (*WebsocketHost).handleUpdateResult,
	"PushResults":              (*WebsocketHost).handlePushResults,
	"ExecuteToolbarMsgAction":  (*WebsocketHost).handleExecuteToolbarMsgAction,
	"AIChatStream":             (*WebsocketHost).handleAIChatStream,
}
