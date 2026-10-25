package host

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"wox/common"
	"wox/plugin"
	"wox/util/selection"
)

// handleChangeQuery adapts the ChangeQuery wire call to the plugin API.
func (w *WebsocketHost) handleChangeQuery(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	queryType, exist := request.Params["queryType"]
	if !exist || (queryType != plugin.QueryTypeInput && queryType != plugin.QueryTypeSelection) {
		return nil, fmt.Errorf("ChangeQuery requires queryType input or selection")
	}

	if queryType == plugin.QueryTypeInput {
		queryText, queryTextExist := request.Params["queryText"]
		var structure *common.QueryHint
		if raw := request.Params["queryHint"]; raw != "" && raw != "null" {
			if err := json.Unmarshal([]byte(raw), &structure); err != nil {
				// Optional decoration must never reject an otherwise valid query.
				structure = nil
			}
			structure = structure.NormalizeForQuery(queryType, queryText)
		}
		if !queryTextExist {
			return nil, fmt.Errorf("ChangeQuery input requires queryText, even when queryHint is supplied")
		}
		pluginInstance.API.ChangeQuery(ctx, common.PlainQuery{
			QueryType:   plugin.QueryTypeInput,
			QueryText:   queryText,
			QueryHint:   structure,
			ContextData: common.UnmarshalContextData(request.Params["queryContextData"]),
		})
	}
	if queryType == plugin.QueryTypeSelection {
		querySelection, querySelectionExist := request.Params["querySelection"]
		if !querySelectionExist || querySelection == "null" || querySelection == "" {
			return nil, fmt.Errorf("ChangeQuery selection requires querySelection")
		}

		var selection selection.Selection
		unmarshalSelectionErr := json.Unmarshal([]byte(querySelection), &selection)
		if unmarshalSelectionErr != nil {
			return nil, fmt.Errorf("failed to unmarshal selection: %w", unmarshalSelectionErr)
		}

		pluginInstance.API.ChangeQuery(ctx, common.PlainQuery{
			QueryType:      plugin.QueryTypeSelection,
			QuerySelection: selection,
			ContextData:    common.UnmarshalContextData(request.Params["queryContextData"]),
		})
	}

	return "", nil
}

// handleRefreshQuery adapts the RefreshQuery wire call to the plugin API.
func (w *WebsocketHost) handleRefreshQuery(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	// Parse PreserveSelectedIndex parameter (optional, defaults to false)
	preserveSelectedIndex := false
	if preserveSelectedIndexStr, exists := request.Params["preserveSelectedIndex"]; exists {
		preserveSelectedIndex = preserveSelectedIndexStr == "true"
	}
	selectedResultId := strings.TrimSpace(request.Params["selectedResultId"])
	if selectedResultId == "" {
		selectedResultId = strings.TrimSpace(request.Params["SelectedResultId"])
	}

	pluginInstance.API.RefreshQuery(ctx, plugin.RefreshQueryParam{
		PreserveSelectedIndex: preserveSelectedIndex,
		SelectedResultId:      selectedResultId,
	})

	return "", nil
}

// handleRegisterQueryCommands adapts the RegisterQueryCommands wire call to the plugin API.
func (w *WebsocketHost) handleRegisterQueryCommands(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	var commands []plugin.MetadataCommand
	unmarshalErr := json.Unmarshal([]byte(request.Params["commands"]), &commands)
	if unmarshalErr != nil {
		return nil, fmt.Errorf("failed to unmarshal commands: %w", unmarshalErr)
	}

	pluginInstance.API.RegisterQueryCommands(ctx, commands)
	return "", nil
}

// handleRegisterTriggerKeyword adapts the RegisterTriggerKeyword wire call to the plugin API.
func (w *WebsocketHost) handleRegisterTriggerKeyword(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	var option plugin.RegisterTriggerKeywordOption
	if err := json.Unmarshal([]byte(request.Params["option"]), &option); err != nil {
		return plugin.RegisterTriggerKeywordResult{Success: false}, nil
	}
	return pluginInstance.API.RegisterTriggerKeyword(ctx, option), nil
}

// handleUnregisterTriggerKeyword adapts the UnregisterTriggerKeyword wire call to the plugin API.
func (w *WebsocketHost) handleUnregisterTriggerKeyword(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	var option plugin.UnregisterTriggerKeywordOption
	if err := json.Unmarshal([]byte(request.Params["option"]), &option); err != nil {
		return plugin.UnregisterTriggerKeywordResult{Success: false}, nil
	}
	return pluginInstance.API.UnregisterTriggerKeyword(ctx, option), nil
}

// handleGetUpdatableResult adapts the GetUpdatableResult wire call to the plugin API.
func (w *WebsocketHost) handleGetUpdatableResult(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	resultId, err := requirePluginParam(request.Params, "resultId")
	if err != nil {
		return nil, err
	}

	result := pluginInstance.API.GetUpdatableResult(ctx, resultId)
	if result == nil {
		return nil, nil
	}

	// Hosts expect JSON inside a string here, not a nested result object.
	resultJson, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		return nil, fmt.Errorf("failed to marshal updatable result: %w", marshalErr)
	}

	return string(resultJson), nil
}

// handleUpdateResult adapts the UpdateResult wire call to the plugin API.
func (w *WebsocketHost) handleUpdateResult(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	resultStr, err := requirePluginParam(request.Params, "result")
	if err != nil {
		return nil, err
	}

	var result plugin.UpdatableResult
	unmarshalErr := json.Unmarshal([]byte(resultStr), &result)
	if unmarshalErr != nil {
		return nil, fmt.Errorf("failed to unmarshal updatable result: %w", unmarshalErr)
	}

	success := pluginInstance.API.UpdateResult(ctx, result)
	return success, nil
}

// handlePushResults adapts the PushResults wire call to the plugin API.
func (w *WebsocketHost) handlePushResults(ctx context.Context, pluginInstance *plugin.Instance, request JsonRpcRequest) (any, error) {
	queryStr, err := requirePluginParam(request.Params, "query")
	if err != nil {
		return nil, err
	}
	resultsStr, err := requirePluginParam(request.Params, "results")
	if err != nil {
		return nil, err
	}

	var query plugin.Query
	unmarshalQueryErr := json.Unmarshal([]byte(queryStr), &query)
	if unmarshalQueryErr != nil {
		return nil, fmt.Errorf("failed to unmarshal query: %w", unmarshalQueryErr)
	}

	var results []plugin.QueryResult
	unmarshalResultsErr := json.Unmarshal([]byte(resultsStr), &results)
	if unmarshalResultsErr != nil {
		return nil, fmt.Errorf("failed to unmarshal results: %w", unmarshalResultsErr)
	}

	success := pluginInstance.API.PushResults(ctx, query, results)
	return success, nil
}
