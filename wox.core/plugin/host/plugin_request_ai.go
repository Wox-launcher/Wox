package host

import (
	"context"
	"encoding/json"
	"fmt"
	"wox/common"
	"wox/plugin"
	"wox/util"
)

// handleAIChatStream adapts the AIChatStream wire call to the plugin API.
func (w *WebsocketHost) handleAIChatStream(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	callbackId, callbackExist := request.Params["callbackId"]
	if !callbackExist || callbackId == "" {
		return w.failAIChatStream(ctx, request, pluginInstance, "", fmt.Errorf("AIChatStream method must have a callbackId parameter"))
	}
	conversationsStr, conversationsExist := request.Params["conversations"]
	if !conversationsExist {
		return w.failAIChatStream(ctx, request, pluginInstance, callbackId, fmt.Errorf("AIChatStream method must have a conversations parameter"))
	}
	optionsStr, optionsExist := request.Params["options"]
	if !optionsExist {
		return w.failAIChatStream(ctx, request, pluginInstance, callbackId, fmt.Errorf("AIChatStream method must have a options parameter"))
	}
	modelStr, modelExist := request.Params["model"]
	if !modelExist {
		return w.failAIChatStream(ctx, request, pluginInstance, callbackId, fmt.Errorf("AIChatStream method must have a model parameter"))
	}

	var model common.Model
	if unmarshalErr := json.Unmarshal([]byte(modelStr), &model); unmarshalErr != nil {
		return w.failAIChatStream(ctx, request, pluginInstance, callbackId, fmt.Errorf("failed to unmarshal model: %w", unmarshalErr))
	}

	var conversations []common.Conversation
	if unmarshalErr := json.Unmarshal([]byte(conversationsStr), &conversations); unmarshalErr != nil {
		return w.failAIChatStream(ctx, request, pluginInstance, callbackId, fmt.Errorf("failed to unmarshal conversations: %w", unmarshalErr))
	}

	var options common.ChatOptions
	if unmarshalErr := json.Unmarshal([]byte(optionsStr), &options); unmarshalErr != nil {
		return w.failAIChatStream(ctx, request, pluginInstance, callbackId, fmt.Errorf("failed to unmarshal options: %w", unmarshalErr))
	}

	llmErr := pluginInstance.API.AIChatStream(ctx, model, conversations, options, func(streamResult common.ChatStreamData) {
		w.invokeMethod(ctx, pluginInstance.Metadata, "onLLMStream", map[string]string{
			"CallbackId": callbackId,
			"StreamType": string(streamResult.Status),
			"Data":       streamResult.Data,
			"Reasoning":  streamResult.Reasoning,
			"ToolCalls":  "", // currently we don't stream toolcalls
		})
	})
	if llmErr != nil {
		return w.failAIChatStream(ctx, request, pluginInstance, callbackId, fmt.Errorf("failed to start LLM stream: %w", llmErr))
	}

	return "", nil
}

// failAIChatStream completes a chat request that never started.
// Hosts block on this RPC, and plugins only observe stream status through
// onLLMStream. A start failure has to finish the request and report the error,
// or the plugin callback never runs.
// Keep the empty acknowledgement: stream errors belong to the callback contract.
func (w *WebsocketHost) failAIChatStream(ctx context.Context, request JsonRpcRequest, pluginInstance *plugin.Instance, callbackId string, startErr error) (any, error) {
	util.GetLogger().Error(ctx, fmt.Sprintf("[%s] %s", request.PluginName, startErr))
	if callbackId != "" && pluginInstance != nil {
		errText := startErr.Error()
		util.Go(ctx, "ai chat stream start error", func() {
			_, invokeErr := w.invokeMethod(ctx, pluginInstance.Metadata, "onLLMStream", map[string]string{
				"CallbackId": callbackId,
				"StreamType": string(common.ChatStreamStatusError),
				"Data":       errText,
				"Reasoning":  "",
				"ToolCalls":  "",
			})
			if invokeErr != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to report AIChatStream error: %s", request.PluginName, invokeErr))
			}
		})
	}
	return "", nil
}
