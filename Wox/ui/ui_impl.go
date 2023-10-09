package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"time"
	"wox/plugin"
	"wox/util"
)

type uiImpl struct {
}

func (u *uiImpl) ChangeQuery(ctx context.Context, query string) {
	u.send(ctx, "ChangeQuery", map[string]string{
		"Query": query,
	})
}

func (u *uiImpl) HideApp(ctx context.Context) {
	u.send(ctx, "HideApp", nil)
}

func (u *uiImpl) ShowApp(ctx context.Context) {
	u.send(ctx, "ShowApp", nil)
}

func (u *uiImpl) ToggleApp(ctx context.Context) {
	u.send(ctx, "ToggleApp", nil)
}

func (u *uiImpl) ShowMsg(ctx context.Context, title string, description string, icon string) {
	u.send(ctx, "ShowMsg", map[string]string{
		"Title":       title,
		"Description": description,
		"Icon":        icon,
	})
}

func (u *uiImpl) send(ctx context.Context, method string, params map[string]string) {
	if params == nil {
		params = make(map[string]string)
	}

	util.GetLogger().Info(ctx, fmt.Sprintf("[UI] %s", method))
	requestUI(ctx, websocketRequest{
		Id:     uuid.NewString(),
		Method: method,
		Params: params,
	})
}

func onUIRequest(ctx context.Context, request websocketRequest) {
	switch request.Method {
	case "Query":
		handleQuery(ctx, request)
	case "Action":
		handleAction(ctx, request)
	case "RegisterMainHotkey":
		handleRegisterMainHotkey(ctx, request)
	}
}

func handleQuery(ctx context.Context, request websocketRequest) {
	query, ok := request.Params["query"]
	if !ok {
		logger.Error(ctx, "query parameter not found")
		return
	}

	resultChan, doneChan := plugin.GetPluginManager().Query(ctx, plugin.NewQuery(query))
	select {
	case results := <-resultChan:
		logger.Info(ctx, fmt.Sprintf("query result count: %d", len(results)))
		if len(results) == 0 {
			return
		}

		response := websocketResponse{
			Id:     request.Id,
			Method: request.Method,
			Data:   plugin.NewQueryResultForUIs(results),
		}

		marshalData, marshalErr := json.Marshal(response)
		if marshalErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to marshal websocket response: %s", marshalErr.Error()))
			return
		}

		m.Broadcast(marshalData)
	case <-doneChan:
		logger.Info(ctx, "query done")
	case <-time.After(time.Second * 30):
		logger.Info(ctx, fmt.Sprintf("query timeout, query: %s, request id: %s", query, request.Id))
	}
}

func handleAction(ctx context.Context, request websocketRequest) {
	resultId, ok := request.Params["id"]
	if !ok {
		logger.Error(ctx, "id parameter not found")
		return
	}

	action := plugin.GetActionForResult(resultId)
	if action == nil {
		logger.Error(ctx, fmt.Sprintf("action not found for result id: %s", resultId))
		return
	}

	action()
}

func handleRegisterMainHotkey(ctx context.Context, request websocketRequest) {
	hotkey, ok := request.Params["hotkey"]
	if !ok {
		logger.Error(ctx, "hotkey parameter not found")
		return
	}

	registerErr := GetUIManager().RegisterMainHotkey(ctx, hotkey)
	if registerErr != nil {
		responseUI(ctx, websocketResponse{
			Id:     request.Id,
			Method: request.Method,
			Data:   registerErr.Error(),
		})
	} else {
		responseUI(ctx, websocketResponse{
			Id:     request.Id,
			Method: request.Method,
			Data:   "success",
		})
	}
}
