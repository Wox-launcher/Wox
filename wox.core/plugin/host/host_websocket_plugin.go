package host

import (
	"context"
	"encoding/json"
	"fmt"
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
		_, actionErr := w.websocketHost.invokeMethod(ctx, w.metadata, "action", map[string]string{
			"ResultId":       actionContext.ResultId,
			"ActionId":       actionId,
			"ResultActionId": actionContext.ResultActionId,
			"ContextData":    actionContext.ContextData,
		})
		if actionErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] action failed: %s", w.metadata.Name, actionErr.Error()))
		}
	}
}

func (w *WebsocketPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	selectionJson, marshalErr := json.Marshal(query.Selection)
	if marshalErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to marshal plugin query selection: %s", w.metadata.Name, marshalErr.Error()))
		return []plugin.QueryResult{}
	}

	envJson, marshalEnvErr := json.Marshal(query.Env)
	if marshalEnvErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to marshal plugin query env: %s", w.metadata.Name, marshalEnvErr.Error()))
		return []plugin.QueryResult{}
	}

	rawResults, queryErr := w.websocketHost.invokeMethod(ctx, w.metadata, "query", map[string]string{
		"Type":           query.Type,
		"RawQuery":       query.RawQuery,
		"TriggerKeyword": query.TriggerKeyword,
		"Command":        query.Command,
		"Search":         query.Search,
		"Selection":      string(selectionJson),
		"Env":            string(envJson),
	})
	if queryErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("[%s] query failed: %s", w.metadata.Name, queryErr.Error()))
		return []plugin.QueryResult{
			plugin.GetPluginManager().GetResultForFailedQuery(ctx, w.metadata, query, queryErr),
		}
	}

	var results []plugin.QueryResult
	marshalData, marshalErr := json.Marshal(rawResults)
	if marshalErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to marshal plugin query results: %s", w.metadata.Name, marshalErr.Error()))
		return nil
	}
	unmarshalErr := json.Unmarshal(marshalData, &results)
	if unmarshalErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to unmarshal query results: %s", w.metadata.Name, unmarshalErr.Error()))
		return []plugin.QueryResult{}
	}

	for i, r := range results {
		result := r
		for j, action := range result.Actions {
			result.Actions[j].Action = func(ctx context.Context, actionContext plugin.ActionContext) {
				_, actionErr := w.websocketHost.invokeMethod(ctx, w.metadata, "action", map[string]string{
					"ResultId":    actionContext.ResultId,
					"ActionId":    action.Id,
					"ContextData": actionContext.ContextData,
				})
				if actionErr != nil {
					util.GetLogger().Error(ctx, fmt.Sprintf("[%s] action failed: %s", w.metadata.Name, actionErr.Error()))
				}
			}
		}

		results[i] = result
	}

	return results
}
