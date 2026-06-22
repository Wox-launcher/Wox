package host

import (
	"context"
	"encoding/json"
	"fmt"
	"wox/common"
	"wox/plugin"
	"wox/util"
)

type WebsocketPlugin struct {
	metadata      plugin.Metadata
	websocketHost *WebsocketHost
}

func NewWebsocketPlugin(metadata plugin.Metadata, websocketHost *WebsocketHost) *WebsocketPlugin {
	return &WebsocketPlugin{
		metadata:      metadata,
		websocketHost: websocketHost,
	}
}

func (w *WebsocketPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	w.websocketHost.invokeMethod(ctx, w.metadata, "init", map[string]string{
		"PluginDirectory": initParams.PluginDirectory,
	})
}

// CreateActionProxy creates a proxy callback for an action that will invoke the host's action method
func (w *WebsocketPlugin) CreateActionProxy(actionId string) func(context.Context, plugin.ActionContext) {
	return func(ctx context.Context, actionContext plugin.ActionContext) {
		_, actionErr := w.websocketHost.invokeMethod(ctx, w.metadata, "action", common.ContextData{
			"ResultId":       actionContext.ResultId,
			"ActionId":       actionId,
			"ResultActionId": actionContext.ResultActionId,
			"ContextData":    actionContext.ContextData.Marshal(),
		})
		if actionErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] action failed: %s", w.metadata.GetName(ctx), actionErr.Error()))
		}
	}
}

// CreateFormActionProxy creates a proxy callback for a form action that will invoke the host's formAction method
func (w *WebsocketPlugin) CreateFormActionProxy(actionId string) func(context.Context, plugin.FormActionContext) {
	return func(ctx context.Context, actionContext plugin.FormActionContext) {
		valuesJson, _ := json.Marshal(actionContext.Values)
		_, actionErr := w.websocketHost.invokeMethod(ctx, w.metadata, "formAction", common.ContextData{
			"ResultId":       actionContext.ResultId,
			"ActionId":       actionId,
			"ResultActionId": actionContext.ResultActionId,
			"ContextData":    actionContext.ContextData.Marshal(),
			"Values":         string(valuesJson),
		})
		if actionErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] form action failed: %s", w.metadata.GetName(ctx), actionErr.Error()))
		}
	}
}

func (w *WebsocketPlugin) CreateToolbarMsgActionProxy(actionId string) func(context.Context, plugin.ToolbarMsgActionContext) {
	return func(ctx context.Context, actionContext plugin.ToolbarMsgActionContext) {
		_, actionErr := w.websocketHost.invokeMethod(ctx, w.metadata, "toolbarMsgAction", common.ContextData{
			"ToolbarMsgId":       actionContext.ToolbarMsgId,
			"ActionId":           actionId,
			"ToolbarMsgActionId": actionContext.ToolbarMsgActionId,
			"ContextData":        actionContext.ContextData.Marshal(),
		})
		if actionErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] toolbar msg action failed: %s", w.metadata.GetName(ctx), actionErr.Error()))
		}
	}
}

func (w *WebsocketPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	selectionJson, marshalErr := json.Marshal(query.Selection)
	if marshalErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to marshal plugin query selection: %s", w.metadata.GetName(ctx), marshalErr.Error()))
		return plugin.QueryResponse{}
	}

	envJson, marshalEnvErr := json.Marshal(query.Env)
	if marshalEnvErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to marshal plugin query env: %s", w.metadata.GetName(ctx), marshalEnvErr.Error()))
		return plugin.QueryResponse{}
	}

	queryRefinements := query.Refinements
	if queryRefinements == nil {
		// External hosts normalize legacy query returns into QueryResponse, so
		// they also expect selected refinements to arrive as an object.
		queryRefinements = map[string]string{}
	}
	refinementsJson, marshalRefinementsErr := json.Marshal(queryRefinements)
	if marshalRefinementsErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to marshal plugin query refinements: %s", w.metadata.GetName(ctx), marshalRefinementsErr.Error()))
		return plugin.QueryResponse{}
	}
	queryContextData := query.ContextData
	if queryContextData == nil {
		queryContextData = common.ContextData{}
	}
	contextDataJson, marshalContextDataErr := json.Marshal(queryContextData)
	if marshalContextDataErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to marshal plugin query context data: %s", w.metadata.GetName(ctx), marshalContextDataErr.Error()))
		return plugin.QueryResponse{}
	}

	// Send both Id and QueryId while hosts move to QueryResponse. Older host
	// code looked for QueryId, while the Go model field is Id.
	rawResults, queryErr := w.websocketHost.invokeMethod(ctx, w.metadata, "query", map[string]string{
		"Id":             query.Id,
		"QueryId":        query.Id,
		"SessionId":      query.SessionId,
		"Type":           query.Type,
		"RawQuery":       query.RawQuery,
		"TriggerKeyword": query.TriggerKeyword,
		"Command":        query.Command,
		"Search":         query.Search,
		"Selection":      string(selectionJson),
		"Env":            string(envJson),
		"Refinements":    string(refinementsJson),
		"ContextData":    string(contextDataJson),
	})
	if queryErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("[%s] query failed: %s", w.metadata.GetName(ctx), queryErr.Error()))
		return plugin.QueryResponse{
			Results: []plugin.QueryResult{plugin.GetPluginManager().GetResultForFailedQuery(ctx, w.metadata, query, queryErr)},
		}
	}

	var response plugin.QueryResponse
	marshalData, marshalErr := json.Marshal(rawResults)
	if marshalErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to marshal plugin query response: %s", w.metadata.GetName(ctx), marshalErr.Error()))
		return plugin.QueryResponse{}
	}
	// Node.js and Python hosts normalize legacy Result[] returns before they
	// cross back into Go, so core only accepts the QueryResponse object here.
	unmarshalErr := json.Unmarshal(marshalData, &response)
	if unmarshalErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to unmarshal query response: %s", w.metadata.GetName(ctx), unmarshalErr.Error()))
		return plugin.QueryResponse{}
	}

	return response
}
