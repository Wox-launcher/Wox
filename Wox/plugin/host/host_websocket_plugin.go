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
	unmarshalErr := json.Unmarshal([]byte(rawResults), &results)
	if unmarshalErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to unmarshal query results: %s", w.metadata.Name, unmarshalErr.Error()))
		return []plugin.QueryResult{}
	}

	for _, result := range results {
		result.Action = func() bool {
			rawResult, actionErr := w.websocketHost.invokeMethod(ctx, w.metadata, "action", map[string]string{
				"ActionId": result.Id,
			})
			if actionErr != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("[%s] action failed: %s", w.metadata.Name, actionErr.Error()))
				return false
			}
			return rawResult == "true"
		}
	}

	return results
}
