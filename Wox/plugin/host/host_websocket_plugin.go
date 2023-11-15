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

func (w *WebsocketPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	selectionJson, marshalErr := json.Marshal(query.Selection)
	if marshalErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to marshal plugin query selection: %s", w.metadata.Name, marshalErr.Error()))
		return []plugin.QueryResult{}
	}

	rawResults, queryErr := w.websocketHost.invokeMethod(ctx, w.metadata, "query", map[string]string{
		"Type":           query.Type,
		"RawQuery":       query.RawQuery,
		"TriggerKeyword": query.TriggerKeyword,
		"Command":        query.Command,
		"Search":         query.Search,
		"Selection":      string(selectionJson),
	})
	if queryErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("[%s] query failed: %s", w.metadata.Name, queryErr.Error()))
		return []plugin.QueryResult{}
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
			result.Actions[j].Action = func(actionContext plugin.ActionContext) {
				_, actionErr := w.websocketHost.invokeMethod(ctx, w.metadata, "action", map[string]string{
					"ActionId":    action.Id,
					"ContextData": actionContext.ContextData,
				})
				if actionErr != nil {
					util.GetLogger().Error(ctx, fmt.Sprintf("[%s] action failed: %s", w.metadata.Name, actionErr.Error()))
				}
			}
		}

		results[i].OnRefresh = func(refreshableResult plugin.RefreshableResult) plugin.RefreshableResult {
			refreshableJson, marshalErr2 := json.Marshal(refreshableResult)
			if marshalErr2 != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to marshal refreshable query results: %s", w.metadata.Name, marshalErr2.Error()))
				return refreshableResult
			}

			rawResult, refreshErr := w.websocketHost.invokeMethod(ctx, w.metadata, "refresh", map[string]string{
				"ResultId":          result.Id,
				"RefreshableResult": string(refreshableJson),
			})
			if refreshErr != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("[%s] refresh failed: %s", w.metadata.Name, refreshErr.Error()))
				return refreshableResult
			}

			var newResult plugin.RefreshableResult
			marshalData3, marshalErr3 := json.Marshal(rawResult)
			if marshalErr3 != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to marshal plugin refreshable results: %s", w.metadata.Name, marshalErr3.Error()))
				return refreshableResult
			}
			unmarshalErr3 := json.Unmarshal(marshalData3, &newResult)
			if unmarshalErr3 != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to unmarshal query refreshable results: %s", w.metadata.Name, unmarshalErr3.Error()))
				return refreshableResult
			}
			return newResult
		}
	}

	return results
}
