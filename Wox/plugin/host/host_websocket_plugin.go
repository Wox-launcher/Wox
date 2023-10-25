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
	w.websocketHost.invokeMethod(ctx, w.metadata, "init", make(map[string]string))
}

func (w *WebsocketPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	rawResults, queryErr := w.websocketHost.invokeMethod(ctx, w.metadata, "query", map[string]string{
		"RawQuery":       query.RawQuery,
		"TriggerKeyword": query.TriggerKeyword,
		"Command":        query.Command,
		"Search":         query.Search,
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
			result.Actions[j].Action = func() {
				_, actionErr := w.websocketHost.invokeMethod(ctx, w.metadata, "action", map[string]string{
					"ActionId": action.Id,
				})
				if actionErr != nil {
					util.GetLogger().Error(ctx, fmt.Sprintf("[%s] action failed: %s", w.metadata.Name, actionErr.Error()))
				}
			}
		}

		resultJson, marshalErr2 := json.Marshal(result.ToUI(query.RawQuery))
		if marshalErr2 != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] marshal result err: %s", w.metadata.Name, marshalErr2.Error()))
			continue
		}
		results[i].OnRefresh = func(result plugin.QueryResult) plugin.QueryResult {
			rawResult, refreshErr := w.websocketHost.invokeMethod(ctx, w.metadata, "refresh", map[string]string{
				"Result": string(resultJson),
			})
			if refreshErr != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("[%s] refresh failed: %s", w.metadata.Name, refreshErr.Error()))
				return result
			}

			var newResult plugin.QueryResult
			marshalData3, marshalErr3 := json.Marshal(rawResult)
			if marshalErr3 != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to marshal plugin query results: %s", w.metadata.Name, marshalErr3.Error()))
				return result
			}
			unmarshalErr3 := json.Unmarshal(marshalData3, &newResult)
			if unmarshalErr3 != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to unmarshal query results: %s", w.metadata.Name, unmarshalErr3.Error()))
				return result
			}
			newResult.OnRefresh = result.OnRefresh
			for k, action := range newResult.Actions {
				newResult.Actions[k].Action = func() {
					_, actionErr := w.websocketHost.invokeMethod(ctx, w.metadata, "action", map[string]string{
						"ActionId": action.Id,
					})
					if actionErr != nil {
						util.GetLogger().Error(ctx, fmt.Sprintf("[%s] action (from refresh) failed: %s", w.metadata.Name, actionErr.Error()))
					}
				}
			}
			return newResult
		}
	}

	return results
}
