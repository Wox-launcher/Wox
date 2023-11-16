package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/tidwall/gjson"
	"time"
	"wox/i18n"
	"wox/plugin"
	"wox/setting"
	"wox/share"
	"wox/util"
	"wox/util/screen"
)

type uiImpl struct {
}

func (u *uiImpl) ChangeQuery(ctx context.Context, query share.ChangedQuery) {
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

func (u *uiImpl) ChangeTheme(ctx context.Context, theme string) {
	u.send(ctx, "ChangeTheme", theme)
}

func (u *uiImpl) send(ctx context.Context, method string, data any) {
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
	case "VisibilityChanged":
		handleOnVisibilityChanged(ctx, request)
	case "LostFocus":
		handleLostFocus(ctx, request)
	case "GetQueryHistories":
		handleGetQueryHistories(ctx, request)
	}
}

func handleQuery(ctx context.Context, request WebsocketMsg) {
	queryId, queryIdErr := getWebsocketMsgParameter(ctx, request, "queryId")
	if queryIdErr != nil {
		logger.Error(ctx, queryIdErr.Error())
		responseUIError(ctx, request, queryIdErr.Error())
		return
	}
	queryType, queryTypeErr := getWebsocketMsgParameter(ctx, request, "queryType")
	if queryTypeErr != nil {
		logger.Error(ctx, queryTypeErr.Error())
		responseUIError(ctx, request, queryTypeErr.Error())
		return
	}
	queryText, queryTextErr := getWebsocketMsgParameter(ctx, request, "queryText")
	if queryTextErr != nil {
		logger.Error(ctx, queryTextErr.Error())
		responseUIError(ctx, request, queryTextErr.Error())
		return
	}
	querySelectionJson, querySelectionErr := getWebsocketMsgParameter(ctx, request, "querySelection")
	if querySelectionErr != nil {
		logger.Error(ctx, querySelectionErr.Error())
		responseUIError(ctx, request, querySelectionErr.Error())
		return
	}
	var querySelection util.Selection
	json.Unmarshal([]byte(querySelectionJson), &querySelection)

	var changedQuery share.ChangedQuery
	if queryType == plugin.QueryTypeInput {
		changedQuery = share.ChangedQuery{
			QueryType: plugin.QueryTypeInput,
			QueryText: queryText,
		}
	} else if queryType == plugin.QueryTypeSelection {
		changedQuery = share.ChangedQuery{
			QueryType:      plugin.QueryTypeSelection,
			QuerySelection: querySelection,
		}
	} else {
		logger.Error(ctx, fmt.Sprintf("unsupported query type: %s", queryType))
		responseUIError(ctx, request, fmt.Sprintf("unsupported query type: %s", queryType))
		return
	}

	if changedQuery.QueryType == plugin.QueryTypeInput && changedQuery.QueryText == "" {
		responseUISuccessWithData(ctx, request, []string{})
		return
	}
	if changedQuery.QueryType == plugin.QueryTypeSelection && changedQuery.QuerySelection.String() == "" {
		responseUISuccessWithData(ctx, request, []string{})
		return
	}

	query, queryErr := plugin.GetPluginManager().NewQuery(ctx, changedQuery)
	if queryErr != nil {
		logger.Error(ctx, queryErr.Error())
		responseUIError(ctx, request, queryErr.Error())
		return
	}

	var totalResultCount int
	var startTimestamp = util.GetSystemTimestamp()
	var resultDebouncer = util.NewDebouncer(30, func(results []plugin.QueryResultUI, reason string) {
		logger.Info(ctx, fmt.Sprintf("query %s: %s, result flushed (reason: %s), total results: %d", query.Type, query.String(), reason, totalResultCount))
		responseUISuccessWithData(ctx, request, results)
	})
	resultDebouncer.Start(ctx)
	logger.Info(ctx, fmt.Sprintf("query %s: %s, result flushed (new start)", query.Type, query.String()))
	resultChan, doneChan := plugin.GetPluginManager().Query(ctx, query)
	for {
		select {
		case results := <-resultChan:
			if len(results) == 0 {
				continue
			}
			lo.ForEach(results, func(_ plugin.QueryResultUI, index int) {
				results[index].QueryId = queryId
			})
			totalResultCount += len(results)
			resultDebouncer.Add(ctx, results)
			//responseUISuccessWithData(ctx, request, results)
		case <-doneChan:
			logger.Info(ctx, fmt.Sprintf("query done, total results: %d, cost %d ms", totalResultCount, util.GetSystemTimestamp()-startTimestamp))
			resultDebouncer.Done(ctx)
			return
		case <-time.After(time.Second * 10):
			logger.Info(ctx, fmt.Sprintf("query timeout, query: %s, request id: %s", query, request.Id))
			resultDebouncer.Done(ctx)
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

func handleOnVisibilityChanged(ctx context.Context, request WebsocketMsg) {
	isVisible, isVisibleErr := getWebsocketMsgParameter(ctx, request, "isVisible")
	if isVisibleErr != nil {
		logger.Error(ctx, isVisibleErr.Error())
		responseUIError(ctx, request, isVisibleErr.Error())
		return
	}

	query, queryErr := getWebsocketMsgParameter(ctx, request, "query")
	if queryErr != nil {
		logger.Error(ctx, queryErr.Error())
		responseUIError(ctx, request, queryErr.Error())
		return
	}

	if isVisible == "true" {
		onAppShow(ctx)
	} else {
		onAppHide(ctx, query)
	}
}

func handleLostFocus(ctx context.Context, request WebsocketMsg) {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if woxSetting.HideOnLostFocus {
		GetUIManager().GetUI(ctx).HideApp(ctx)
	}
}

func handleGetQueryHistories(ctx context.Context, request WebsocketMsg) {
	queryHistories := setting.GetSettingManager().GetWoxAppData(ctx).QueryHistories
	responseUISuccessWithData(ctx, request, queryHistories)
}

func onAppShow(ctx context.Context) {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if woxSetting.SwitchInputMethodABC {
		util.GetLogger().Info(ctx, "switch input method to ABC")
		util.SwitchInputMethodABC()
	}
}

func onAppHide(ctx context.Context, query string) {
	setting.GetSettingManager().AddQueryHistory(ctx, query)
	if setting.GetSettingManager().GetWoxSetting(ctx).LastQueryMode == setting.LastQueryModeEmpty {
		GetUIManager().GetUI(ctx).ChangeQuery(ctx, share.ChangedQuery{
			QueryType: plugin.QueryTypeInput,
			QueryText: "",
		})
	}
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
	windowWidth := 800
	size := screen.GetMouseScreen()
	x := size.X + (size.Width-windowWidth)/2
	y := size.Height / 5
	return x, y
}
