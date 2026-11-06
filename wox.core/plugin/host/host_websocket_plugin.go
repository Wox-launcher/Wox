package host

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"wox/common"
	"wox/plugin"
	"wox/util"
)

type WebsocketPlugin struct {
	metadata      plugin.Metadata
	websocketHost *WebsocketHost
}

// DispatchWoxViewPreviewEvent sends a result-scoped event to its owning host.
func (w *WebsocketPlugin) DispatchWoxViewPreviewEvent(ctx context.Context, resultID, documentID string, event plugin.WoxViewPreviewEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	params := map[string]string{"ResultId": resultID, "DocumentID": documentID, "Event": string(data)}
	if event.Type == plugin.WoxViewPreviewEventShow || event.Type == plugin.WoxViewPreviewEventHide {
		params["Lifecycle"] = "true"
	}
	_, err = w.websocketHost.invokeMethod(ctx, w.metadata, "woxviewPreviewEvent", params)
	return err
}

func NewWebsocketPlugin(metadata plugin.Metadata, websocketHost *WebsocketHost) *WebsocketPlugin {
	return &WebsocketPlugin{
		metadata:      metadata,
		websocketHost: websocketHost,
	}
}

func (w *WebsocketPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	if err := w.InitWithError(ctx, initParams); err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("[%s] init failed: %s", w.metadata.GetName(ctx), err.Error()))
	}
}

// InitWithError runs the host init RPC and returns the transport or plugin error.
func (w *WebsocketPlugin) InitWithError(ctx context.Context, initParams plugin.InitParams) error {
	_, err := w.websocketHost.invokeMethod(ctx, w.metadata, "init", map[string]string{
		"PluginDirectory": initParams.PluginDirectory,
	})
	return err
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
	return w.CreateManagedToolbarMsgActionProxy(actionId, "")
}

// CreateManagedToolbarMsgActionProxy routes an action to its host-owned message snapshot.
func (w *WebsocketPlugin) CreateManagedToolbarMsgActionProxy(actionId, snapshotId string) func(context.Context, plugin.ToolbarMsgActionContext) {
	return func(ctx context.Context, actionContext plugin.ToolbarMsgActionContext) {
		_, actionErr := w.websocketHost.invokeMethod(ctx, w.metadata, "toolbarMsgAction", common.ContextData{
			"ToolbarMsgId":       actionContext.ToolbarMsgId,
			"ActionId":           actionId,
			"ToolbarMsgActionId": actionContext.ToolbarMsgActionId,
			"HostSnapshotId":     snapshotId,
			"ContextData":        actionContext.ContextData.Marshal(),
		})
		if actionErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] toolbar msg action failed: %s", w.metadata.GetName(ctx), actionErr.Error()))
		}
	}
}

// RetireToolbarSnapshot removes one retired message and all its host callbacks.
func (w *WebsocketPlugin) RetireToolbarSnapshot(ctx context.Context, snapshotId string) {
	client := w.websocketHost.websocketClient()
	logged := false
	for client != nil && w.websocketHost.websocketClient() == client {
		if _, err := w.websocketHost.invokeMethod(ctx, w.metadata, "retireToolbarSnapshot", common.ContextData{"HostSnapshotId": snapshotId}); err == nil {
			return
		} else if !logged {
			util.GetLogger().Debug(ctx, fmt.Sprintf("[%s] retrying toolbar callback cleanup: %s", w.metadata.GetName(ctx), err))
			logged = true
		}
		// The same host can reconnect; a replacement host owns a fresh callback store.
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

func (w *WebsocketPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	response, queryErr := w.QueryWithError(ctx, query)
	if queryErr != nil {
		return plugin.QueryResponse{
			Results: []plugin.QueryResult{plugin.GetPluginManager().GetResultForFailedQuery(ctx, w.metadata, query, queryErr)},
		}
	}
	return response
}

// QueryWithError runs the host query RPC and returns the transport or decode error.
func (w *WebsocketPlugin) QueryWithError(ctx context.Context, query plugin.Query) (plugin.QueryResponse, error) {
	selectionJson, marshalErr := json.Marshal(query.Selection)
	if marshalErr != nil {
		return plugin.QueryResponse{}, fmt.Errorf("failed to marshal plugin query selection: %w", marshalErr)
	}

	envJson, marshalEnvErr := json.Marshal(query.Env)
	if marshalEnvErr != nil {
		return plugin.QueryResponse{}, fmt.Errorf("failed to marshal plugin query env: %w", marshalEnvErr)
	}

	queryRefinements := query.Refinements
	if queryRefinements == nil {
		// External hosts normalize legacy query returns into QueryResponse, so
		// they also expect selected refinements to arrive as an object.
		queryRefinements = map[string]string{}
	}
	refinementsJson, marshalRefinementsErr := json.Marshal(queryRefinements)
	if marshalRefinementsErr != nil {
		return plugin.QueryResponse{}, fmt.Errorf("failed to marshal plugin query refinements: %w", marshalRefinementsErr)
	}
	queryContextData := query.ContextData
	if queryContextData == nil {
		queryContextData = common.ContextData{}
	}
	contextDataJson, marshalContextDataErr := json.Marshal(queryContextData)
	if marshalContextDataErr != nil {
		return plugin.QueryResponse{}, fmt.Errorf("failed to marshal plugin query context data: %w", marshalContextDataErr)
	}

	structureJSON, err := json.Marshal(query.QueryHint)
	if err != nil {
		return plugin.QueryResponse{}, err
	}
	// Send both Id and QueryId while hosts move to QueryResponse. Older host
	// code looked for QueryId, while the Go model field is Id.
	// QueryScope stays core-internal and is not forwarded to external plugin hosts.
	rawResults, queryErr := w.websocketHost.invokeMethod(ctx, w.metadata, "query", map[string]string{
		"Id":             query.Id,
		"QueryId":        query.Id,
		"SessionId":      query.SessionId,
		"Type":           query.Type,
		"RawQuery":       query.RawQuery,
		"QueryHint":      string(structureJSON),
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
		return plugin.QueryResponse{}, queryErr
	}

	var response plugin.QueryResponse
	marshalData, marshalErr := json.Marshal(rawResults)
	if marshalErr != nil {
		return plugin.QueryResponse{}, fmt.Errorf("failed to marshal plugin query response: %w", marshalErr)
	}
	// Node.js and Python hosts normalize legacy Result[] returns before they
	// cross back into Go, so core only accepts the QueryResponse object here.
	unmarshalErr := json.Unmarshal(marshalData, &response)
	if unmarshalErr != nil {
		return plugin.QueryResponse{}, fmt.Errorf("failed to unmarshal query response: %w", unmarshalErr)
	}

	return response, nil
}
