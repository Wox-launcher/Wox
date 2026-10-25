package host

import (
	"context"
	"encoding/json"
	"strings"
	"wox/plugin"

	"github.com/google/uuid"
)

// handleRegisterPluginTool adapts the RegisterPluginTool wire call to the plugin API.
func (w *WebsocketHost) handleRegisterPluginTool(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	return w.registerHostPluginTool(ctx, pluginInstance, request.Params["option"]), nil
}

// handleUnregisterPluginTool adapts the UnregisterPluginTool wire call to the plugin API.
func (w *WebsocketHost) handleUnregisterPluginTool(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	var option plugin.UnregisterPluginToolOption
	if err := json.Unmarshal([]byte(request.Params["option"]), &option); err != nil {
		return plugin.UnregisterPluginToolResult{Error: &plugin.PluginToolError{Code: plugin.PluginToolErrorInvalidRegistration, Message: err.Error()}}, nil
	}
	return pluginInstance.API.UnregisterPluginTool(ctx, option), nil
}

// handleListPluginTools adapts the ListPluginTools wire call to the plugin API.
func (w *WebsocketHost) handleListPluginTools(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	var option plugin.ListPluginToolsOption
	if raw := request.Params["option"]; raw != "" {
		if err := json.Unmarshal([]byte(raw), &option); err != nil {
			return plugin.ListPluginToolsResult{Error: &plugin.PluginToolError{Code: plugin.PluginToolErrorInvalidArguments, Message: err.Error()}}, nil
		}
	}
	return pluginInstance.API.ListPluginTools(ctx, option), nil
}

// handleInvokePluginTool adapts the InvokePluginTool wire call to the plugin API.
func (w *WebsocketHost) handleInvokePluginTool(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	parentCtx, parentErr := w.pluginToolCallContext(ctx, request.PluginId, request.Params["PluginToolCallId"])
	if parentErr != nil {
		return plugin.InvokePluginToolResult{Error: parentErr}, nil
	}
	var option plugin.InvokePluginToolOption
	if err := json.Unmarshal([]byte(request.Params["option"]), &option); err != nil {
		return plugin.InvokePluginToolResult{Error: &plugin.PluginToolError{Code: plugin.PluginToolErrorInvalidArguments, Message: err.Error()}}, nil
	}
	return pluginInstance.API.InvokePluginTool(parentCtx, option), nil
}

// registerHostPluginTool binds a remote callback to the lifetime of each tool invocation.
func (w *WebsocketHost) registerHostPluginTool(ctx context.Context, pluginInstance *plugin.Instance, optionJSON string) plugin.RegisterPluginToolResult {
	var payload struct {
		Tool       plugin.PluginToolDescriptor
		CallbackId string
	}
	if err := json.Unmarshal([]byte(optionJSON), &payload); err != nil {
		return plugin.RegisterPluginToolResult{Error: &plugin.PluginToolError{Code: plugin.PluginToolErrorInvalidRegistration, Message: err.Error()}}
	}
	callbackId := strings.TrimSpace(payload.CallbackId)
	if callbackId == "" {
		return plugin.RegisterPluginToolResult{Error: &plugin.PluginToolError{Code: plugin.PluginToolErrorInvalidRegistration, Message: "callbackId is required"}}
	}

	metadata := pluginInstance.Metadata
	return pluginInstance.API.RegisterPluginTool(ctx, plugin.RegisterPluginToolOption{
		Tool: payload.Tool,
		Handler: func(callbackCtx context.Context, option plugin.InvokePluginToolHandlerOption) plugin.InvokePluginToolHandlerResult {
			token := uuid.NewString()
			w.pluginToolCalls.Store(token, hostPluginToolCall{ctx: callbackCtx, pluginID: metadata.Id})
			defer w.pluginToolCalls.Delete(token)
			argumentsJSON, marshalErr := json.Marshal(option.Arguments)
			if marshalErr != nil {
				return plugin.InvokePluginToolHandlerResult{Error: &plugin.PluginToolError{Code: plugin.PluginToolErrorInvalidArguments, Message: marshalErr.Error()}}
			}
			raw, invokeErr := w.invokeMethod(callbackCtx, metadata, "onInvokePluginTool", map[string]string{
				"CallbackId":       callbackId,
				"Arguments":        string(argumentsJSON),
				"PluginToolCallId": token,
			})
			if invokeErr != nil {
				return plugin.InvokePluginToolHandlerResult{Error: plugin.PluginToolErrorFromHost(invokeErr)}
			}
			var handlerResult plugin.InvokePluginToolHandlerResult
			if raw == nil {
				return plugin.InvokePluginToolHandlerResult{Output: map[string]any{}}
			}
			if decodeErr := w.decodeHostResult(raw, &handlerResult); decodeErr != nil {
				return plugin.InvokePluginToolHandlerResult{Error: &plugin.PluginToolError{Code: plugin.PluginToolErrorExecutionFailed, Message: decodeErr.Error()}}
			}
			return handlerResult
		},
	})
}

type hostPluginToolCall struct {
	ctx      context.Context
	pluginID string
}

// pluginToolCallContext restores trusted state rather than rebuilding deadlines or call chains from wire values.
func (w *WebsocketHost) pluginToolCallContext(ctx context.Context, pluginID, token string) (context.Context, *plugin.PluginToolError) {
	if token == "" {
		return ctx, nil
	}
	value, ok := w.pluginToolCalls.Load(token)
	if !ok || value.(hostPluginToolCall).pluginID != pluginID {
		return ctx, &plugin.PluginToolError{Code: plugin.PluginToolErrorPermissionDenied, Message: "plugin tool parent call is no longer available"}
	}
	parent := value.(hostPluginToolCall).ctx
	if err := parent.Err(); err != nil {
		return ctx, plugin.PluginToolErrorFromHost(err)
	}
	return parent, nil
}
