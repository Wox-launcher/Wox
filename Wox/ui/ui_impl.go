package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
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
	x, y := getWindowShowLocation()
	u.send(ctx, "ShowApp", map[string]string{
		"X": fmt.Sprintf("%d", x),
		"Y": fmt.Sprintf("%d", y),
	})
}

func (u *uiImpl) ToggleApp(ctx context.Context) {
	x, y := getWindowShowLocation()
	u.send(ctx, "ToggleApp", map[string]string{
		"X": fmt.Sprintf("%d", x),
		"Y": fmt.Sprintf("%d", y),
	})
}

func (u *uiImpl) ShowMsg(ctx context.Context, title string, description string, icon string) {
	u.send(ctx, "ShowMsg", map[string]string{
		"Title":       title,
		"Description": description,
		"Icon":        icon,
	})
}

func (u *uiImpl) GetServerPort(ctx context.Context) int {
	return GetUIManager().serverPort
}

func (u *uiImpl) send(ctx context.Context, method string, params map[string]string) {
	if params == nil {
		params = make(map[string]string)
	}

	util.GetLogger().Info(ctx, fmt.Sprintf("[UI] %s", method))
	requestUI(ctx, WebsocketMsg{
		Id:     uuid.NewString(),
		Method: method,
		Data:   params,
	})
}

func onUIRequest(ctx context.Context, request WebsocketMsg) {
	switch request.Method {
	case "Query":
		handleQuery(ctx, request)
	case "Action":
		handleAction(ctx, request)
	case "RegisterMainHotkey":
		handleRegisterMainHotkey(ctx, request)
	}
}

func handleQuery(ctx context.Context, request WebsocketMsg) {
	query, queryErr := getWebsocketMsgParameter(ctx, request, "query")
	if queryErr != nil {
		logger.Error(ctx, queryErr.Error())
		responseUIError(ctx, request, queryErr.Error())
		return
	}
	queryType, queryTypeErr := getWebsocketMsgParameter(ctx, request, "type")
	if queryTypeErr != nil {
		logger.Error(ctx, queryTypeErr.Error())
		responseUIError(ctx, request, queryTypeErr.Error())
		return
	}

	if query == "" {
		responseUISuccessWithData(ctx, request, []string{})
		return
	}

	var totalResultCount int
	var startTimestamp = util.GetSystemTimestamp()
	resultChan, doneChan := plugin.GetPluginManager().Query(ctx, plugin.NewQuery(query, queryType))
	for {
		select {
		case results := <-resultChan:
			if len(results) == 0 {
				continue
			}
			totalResultCount += len(results)
			responseUISuccessWithData(ctx, request, results)
		case <-doneChan:
			logger.Info(ctx, fmt.Sprintf("query done, total results: %d, cost %d ms", totalResultCount, util.GetSystemTimestamp()-startTimestamp))
			responseUISuccessWithData(ctx, request, []string{})
			return
		case <-time.After(time.Second * 10):
			logger.Info(ctx, fmt.Sprintf("query timeout, query: %s, request id: %s", query, request.Id))
			responseUIError(ctx, request, fmt.Sprintf("query timeout, query: %s, request id: %s", query, request.Id))
			return
		}
	}

}

func handleAction(ctx context.Context, request WebsocketMsg) {
	resultId, idErr := getWebsocketMsgParameter(ctx, request, "id")
	if idErr != nil {
		logger.Error(ctx, idErr.Error())
		responseUIError(ctx, request, idErr.Error())
		return
	}

	action := plugin.GetPluginManager().GetAction(resultId)
	if action == nil {
		logger.Error(ctx, fmt.Sprintf("action not found for result id: %s", resultId))
		responseUIError(ctx, request, fmt.Sprintf("action not found for result id: %s", resultId))
		return
	}

	action()
	responseUISuccess(ctx, request)
}

func handleRegisterMainHotkey(ctx context.Context, request WebsocketMsg) {
	hotkey, hotkeyErr := getWebsocketMsgParameter(ctx, request, "hotkey")
	if hotkeyErr != nil {
		logger.Error(ctx, hotkeyErr.Error())
		responseUIError(ctx, request, hotkeyErr.Error())
		return
	}

	registerErr := GetUIManager().RegisterMainHotkey(ctx, hotkey)
	if registerErr != nil {
		responseUIError(ctx, request, registerErr.Error())
	} else {
		responseUISuccess(ctx, request)
	}
}

func getWebsocketMsgParameter(ctx context.Context, msg WebsocketMsg, key string) (string, error) {
	jsonData, marshalErr := json.Marshal(msg.Data)
	if marshalErr != nil {
		return "", marshalErr
	}

	paramterData := gjson.GetBytes(jsonData, key)
	if !paramterData.Exists() {
		return "", errors.New(fmt.Sprintf("%s parameter not found", key))
	}

	return paramterData.String(), nil
}

func getWindowShowLocation() (int, int) {
	return util.GetWindowShowLocation(800)
}
