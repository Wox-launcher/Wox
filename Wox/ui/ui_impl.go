package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"time"
	"wox/i18n"
	"wox/plugin"
	"wox/share"
	"wox/util"
)

type uiImpl struct {
}

func (u *uiImpl) ChangeQuery(ctx context.Context, query string) {
	u.send(ctx, "ChangeQuery", query)
}

func (u *uiImpl) HideApp(ctx context.Context) {
	u.send(ctx, "HideApp", nil)
}

func (u *uiImpl) ShowApp(ctx context.Context, showContext share.ShowContext) {
	u.send(ctx, "ShowApp", map[string]any{
		"SelectAll": showContext.SelectAll,
		"Position":  NewMouseScreenPosition(),
	})
}

func (u *uiImpl) ToggleApp(ctx context.Context) {
	u.send(ctx, "ToggleApp", map[string]any{
		"SelectAll": true,
		"Position":  NewMouseScreenPosition(),
	})
}

func (u *uiImpl) ShowMsg(ctx context.Context, title string, description string, icon string) {
	u.send(ctx, "ShowMsg", map[string]any{
		"Title":       title,
		"Description": description,
		"Icon":        icon,
	})
}

func (u *uiImpl) GetServerPort(ctx context.Context) int {
	return GetUIManager().serverPort
}

func (u *uiImpl) send(ctx context.Context, method string, data any) {
	jsonData, _ := json.Marshal(data)
	util.GetLogger().Info(ctx, fmt.Sprintf("[->UI] %s: %s", method, jsonData))
	requestUI(ctx, WebsocketMsg{
		Id:     uuid.NewString(),
		Method: method,
		Data:   data,
	})
}

func onUIRequest(ctx context.Context, request WebsocketMsg) {
	switch request.Method {
	case "Ping":
		responseUISuccessWithData(ctx, request, "Pong")
	case "Query":
		handleQuery(ctx, request)
	case "Action":
		handleAction(ctx, request)
	case "Refresh":
		handleRefresh(ctx, request)
	case "RegisterMainHotkey":
		handleRegisterMainHotkey(ctx, request)
	case "IsHotkeyAvailable":
		handleIsHotkeyAvailable(ctx, request)
	case "ChangeLanguage":
		handleChangeLanguage(ctx, request)
	case "GetLanguageJson":
		handleGetLanguageJson(ctx, request)
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
	resultId, idErr := getWebsocketMsgParameter(ctx, request, "resultId")
	if idErr != nil {
		logger.Error(ctx, idErr.Error())
		responseUIError(ctx, request, idErr.Error())
		return
	}
	actionId, actionIdErr := getWebsocketMsgParameter(ctx, request, "actionId")
	if actionIdErr != nil {
		logger.Error(ctx, actionIdErr.Error())
		responseUIError(ctx, request, actionIdErr.Error())
		return
	}

	plugin.GetPluginManager().ExecuteAction(ctx, resultId, actionId)

	responseUISuccess(ctx, request)
}

func handleRefresh(ctx context.Context, request WebsocketMsg) {
	resultStr, resultErr := getWebsocketMsgParameter(ctx, request, "refreshableResult")
	if resultErr != nil {
		logger.Error(ctx, resultErr.Error())
		responseUIError(ctx, request, resultErr.Error())
		return
	}
	resultId, resultIdErr := getWebsocketMsgParameter(ctx, request, "resultId")
	if resultIdErr != nil {
		logger.Error(ctx, resultIdErr.Error())
		responseUIError(ctx, request, resultIdErr.Error())
		return
	}

	var result plugin.RefreshableResult
	unmarshalErr := json.Unmarshal([]byte(resultStr), &result)
	if unmarshalErr != nil {
		logger.Error(ctx, unmarshalErr.Error())
		responseUIError(ctx, request, unmarshalErr.Error())
		return
	}

	newResult, refreshErr := plugin.GetPluginManager().ExecuteRefresh(ctx, resultId, result)
	if refreshErr != nil {
		logger.Error(ctx, refreshErr.Error())
		responseUIError(ctx, request, refreshErr.Error())
		return
	}

	responseUISuccessWithData(ctx, request, newResult)
}

func handleChangeLanguage(ctx context.Context, request WebsocketMsg) {
	langCode, langCodeErr := getWebsocketMsgParameter(ctx, request, "langCode")
	if langCodeErr != nil {
		logger.Error(ctx, langCodeErr.Error())
		responseUIError(ctx, request, langCodeErr.Error())
		return
	}
	if !i18n.IsSupportedLangCode(langCode) {
		logger.Error(ctx, fmt.Sprintf("unsupported lang code: %s", langCode))
		responseUIError(ctx, request, fmt.Sprintf("unsupported lang code: %s", langCode))
		return
	}

	langErr := i18n.GetI18nManager().UpdateLang(ctx, i18n.LangCode(langCode))
	if langErr != nil {
		logger.Error(ctx, langErr.Error())
		responseUIError(ctx, request, langErr.Error())
		return
	}

	responseUISuccess(ctx, request)
}

func handleGetLanguageJson(ctx context.Context, request WebsocketMsg) {
	langCode, langCodeErr := getWebsocketMsgParameter(ctx, request, "langCode")
	if langCodeErr != nil {
		logger.Error(ctx, langCodeErr.Error())
		responseUIError(ctx, request, langCodeErr.Error())
		return
	}
	if !i18n.IsSupportedLangCode(langCode) {
		logger.Error(ctx, fmt.Sprintf("unsupported lang code: %s", langCode))
		responseUIError(ctx, request, fmt.Sprintf("unsupported lang code: %s", langCode))
		return
	}

	langJson, err := i18n.GetI18nManager().GetLangJson(ctx, i18n.LangCode(langCode))
	if err != nil {
		logger.Error(ctx, err.Error())
		responseUIError(ctx, request, err.Error())
		return
	}

	responseUISuccessWithData(ctx, request, langJson)
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

func handleIsHotkeyAvailable(ctx context.Context, request WebsocketMsg) {
	hotkey, hotkeyErr := getWebsocketMsgParameter(ctx, request, "hotkey")
	if hotkeyErr != nil {
		logger.Error(ctx, hotkeyErr.Error())
		responseUIError(ctx, request, hotkeyErr.Error())
		return
	}

	isAvailable := false
	hk := util.Hotkey{}
	registerErr := hk.Register(ctx, hotkey, func() {

	})
	if registerErr == nil {
		isAvailable = true
		hk.Unregister(ctx)
	}

	responseUISuccessWithData(ctx, request, isAvailable)
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
