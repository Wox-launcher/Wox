package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"strings"
	"time"
	"wox/common"
	"wox/plugin"
	"wox/plugin/system/shell/terminal"
	"wox/setting"
	"wox/util"
	"wox/util/notifier"
	"wox/util/selection"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/tidwall/gjson"
)

func parseQueryRefinementsFromUI(rawJson string) (map[string]string, error) {
	refinements := map[string]string{}
	if rawJson == "" {
		return refinements, nil
	}

	var rawValues map[string]any
	if err := json.Unmarshal([]byte(rawJson), &rawValues); err != nil {
		return nil, err
	}

	for key, rawValue := range rawValues {
		switch value := rawValue.(type) {
		case string:
			refinements[key] = value
		case []any:
			// Protocol migration: older UI builds sent refinement selections as
			// string arrays. The public plugin API now uses map[string]string, so
			// the UI boundary joins legacy multi-select values once instead of
			// forcing every plugin and runtime host to understand both shapes.
			parts := []string{}
			for _, item := range value {
				text := fmt.Sprint(item)
				if text != "" {
					parts = append(parts, text)
				}
			}
			joined := strings.Join(parts, ",")
			if joined != "" {
				refinements[key] = joined
			}
		default:
			if rawValue != nil {
				refinements[key] = fmt.Sprint(rawValue)
			}
		}
	}

	return refinements, nil
}

type uiImpl struct {
	requestMap         *util.HashMap[string, chan WebsocketMsg]
	isVisible          bool // cached visibility state, updated by PostOnShow/PostOnHide
	isInSettingView    bool // cached setting-view state, updated by PostOnSetting
	isInOnboardingView bool // cached onboarding state, updated by PostOnOnboarding
	isRecordingHotkey  bool // cached hotkey-recorder focus state, updated by PostOnHotkeyRecording
}

func (u *uiImpl) ChangeQuery(ctx context.Context, query common.PlainQuery) {
	data := map[string]any{
		"QueryId":        query.QueryId,
		"QueryType":      query.QueryType,
		"QueryText":      query.QueryText,
		"QuerySelection": query.QuerySelection,
	}

	if showSource := util.GetContextShowSource(ctx); showSource != "" {
		data["ShowSource"] = showSource
	}

	u.invokeWebsocketMethod(ctx, "ChangeQuery", data)
}

func (u *uiImpl) RefreshQuery(ctx context.Context, preserveSelectedIndex bool) {
	u.invokeWebsocketMethod(ctx, "RefreshQuery", map[string]any{
		"preserveSelectedIndex": preserveSelectedIndex,
	})
}

func (u *uiImpl) RefreshGlance(ctx context.Context, pluginId string, ids []string) {
	u.invokeWebsocketMethod(ctx, "RefreshGlance", map[string]any{
		"PluginId": pluginId,
		"Ids":      ids,
	})
}

func (u *uiImpl) UpdateDiagnosticStatus(ctx context.Context, enabled bool) {
	// New feature: bug aware status is a global launcher decoration, so core
	// pushes it separately from plugin toolbar messages to avoid ownership
	// conflicts with normal plugin status updates.
	u.invokeWebsocketMethod(ctx, "DiagnosticStatusChanged", map[string]any{"enabled": enabled})
}

func (u *uiImpl) HideApp(ctx context.Context) {
	u.invokeWebsocketMethod(ctx, "HideApp", nil)
}

func (u *uiImpl) ShowApp(ctx context.Context, showContext common.ShowContext) {
	GetUIManager().RefreshActiveWindowSnapshot(ctx)
	u.invokeWebsocketMethod(ctx, "ShowApp", getShowAppParams(ctx, showContext))
}

func (u *uiImpl) ToggleApp(ctx context.Context, showContext common.ShowContext) {
	GetUIManager().RefreshActiveWindowSnapshot(ctx)
	u.invokeWebsocketMethod(ctx, "ToggleApp", getShowAppParams(ctx, showContext))
}

func (u *uiImpl) RecordHotkey(ctx context.Context, hotkey string) {
	logger.Info(ctx, fmt.Sprintf("send RecordHotkey to UI: hotkey=%s", hotkey))
	u.invokeWebsocketMethod(ctx, "RecordHotkey", map[string]any{
		"Hotkey": hotkey,
	})
}

func (u *uiImpl) GetServerPort(ctx context.Context) int {
	return GetUIManager().serverPort
}

func (u *uiImpl) ChangeTheme(ctx context.Context, theme common.Theme) {
	logger.Info(ctx, fmt.Sprintf("change theme: %s", theme.ThemeName))

	// If it's an auto appearance theme, delegate to manager for proper handling
	if theme.IsAutoAppearance {
		GetUIManager().ChangeTheme(ctx, theme)
		return
	}

	// For normal themes, save and apply directly
	// New feature: direct common.UI callers may bypass Manager.ChangeTheme, so
	// resolve platform overrides here as well before sending the flat payload to
	// Flutter.
	effectiveTheme := GetUIManager().resolvePlatformTheme(ctx, theme)
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	woxSetting.ThemeId.Set(effectiveTheme.ThemeId)
	u.invokeWebsocketMethod(ctx, "ChangeTheme", effectiveTheme)
}

// ChangeThemeWithoutSave applies the theme without saving to settings
// This is used for auto appearance theme switching
func (u *uiImpl) ChangeThemeWithoutSave(ctx context.Context, theme common.Theme) {
	logger.Info(ctx, fmt.Sprintf("change theme (without save): %s", theme.ThemeName))
	u.invokeWebsocketMethod(ctx, "ChangeTheme", GetUIManager().resolvePlatformTheme(ctx, theme))
}

func (u *uiImpl) InstallTheme(ctx context.Context, theme common.Theme) {
	logger.Info(ctx, fmt.Sprintf("install theme: %s", theme.ThemeName))
	GetStoreManager().Install(ctx, theme)
}

func (u *uiImpl) UninstallTheme(ctx context.Context, theme common.Theme) {
	logger.Info(ctx, fmt.Sprintf("uninstall theme: %s", theme.ThemeName))
	GetStoreManager().Uninstall(ctx, theme)
	GetUIManager().ChangeToDefaultTheme(ctx)
}

func (u *uiImpl) OpenSettingWindow(ctx context.Context, windowContext common.SettingWindowContext) {
	u.invokeWebsocketMethod(ctx, "OpenSettingWindow", windowContext)
}

func (u *uiImpl) OpenOnboardingWindow(ctx context.Context) {
	// Onboarding reuses the same UI process and WebSocket command path as the
	// settings window. Keeping it here avoids a second desktop window lifecycle
	// while still letting Flutter choose the dedicated onboarding view.
	u.invokeWebsocketMethod(ctx, "OpenOnboardingWindow", nil)
}

func (u *uiImpl) GetAllThemes(ctx context.Context) []common.Theme {
	return GetUIManager().GetAllThemes(ctx)
}

func (u *uiImpl) RestoreTheme(ctx context.Context) {
	GetUIManager().RestoreTheme(ctx)
}

func (u *uiImpl) Notify(ctx context.Context, msg common.NotifyMsg) {
	if u.IsVisible(ctx) && !u.IsInManagementView() && !plugin.GetPluginManager().HasVisibleToolbarMsg(ctx) {
		u.invokeWebsocketMethod(ctx, "ShowToolbarMsg", msg)
	} else {
		var icon image.Image
		if msg.Icon != "" {
			wimg, parseErr := common.ParseWoxImage(msg.Icon)
			if parseErr == nil {
				// System notifications should appear as soon as the action succeeds. The previous path used
				// ToImage(), which could synchronously download Twemoji assets for emoji icons and delay the
				// success notification by seconds on a cold cache. Keep the notify path local-only and let the
				// notifier fall back to the default Wox icon when the plugin icon is not already cached.
				img, imgErr := wimg.ToImageWithoutRemoteFetch()
				if imgErr == nil {
					icon = img
				}
			}
		}
		notifier.Notify(icon, msg.Text)
	}
}

func (u *uiImpl) UpdateAttentionUnreadCount(ctx context.Context, unreadCount int) {
	u.invokeWebsocketMethod(ctx, "AttentionUnreadCountChanged", map[string]any{
		"unreadCount": unreadCount,
	})
}

func (u *uiImpl) ShowToolbarMsg(ctx context.Context, msg interface{}) {
	u.invokeWebsocketMethod(ctx, "ShowToolbarMsg", msg)
}

func (u *uiImpl) ClearToolbarMsg(ctx context.Context, toolbarMsgId string) {
	u.invokeWebsocketMethod(ctx, "ClearToolbarMsg", map[string]any{
		"toolbarMsgId": toolbarMsgId,
	})
}

func (u *uiImpl) IsInSettingView() bool {
	return u.isInSettingView
}

func (u *uiImpl) IsInManagementView() bool {
	// Settings and onboarding both occupy the shared Wox window as management
	// surfaces, so toolbar notifications should not overlay either of them.
	return u.isInSettingView || u.isInOnboardingView
}

func (u *uiImpl) FocusToChatInput(ctx context.Context) {
	u.invokeWebsocketMethod(ctx, "FocusToChatInput", nil)
}

func (u *uiImpl) GetActiveWindowSnapshot(ctx context.Context) common.ActiveWindowSnapshot {
	return GetUIManager().GetActiveWindowSnapshot(ctx)
}

func (u *uiImpl) SendChatResponse(ctx context.Context, aiChatData common.AIChatData) {
	u.invokeWebsocketMethod(ctx, "SendChatResponse", aiChatData)
}

func (u *uiImpl) ReloadChatResources(ctx context.Context, resouceName string) {
	u.invokeWebsocketMethod(ctx, "ReloadChatResources", resouceName)
}

func (u *uiImpl) ReloadSettingPlugins(ctx context.Context) {
	u.invokeWebsocketMethod(ctx, "ReloadSettingPlugins", nil)
}

func (u *uiImpl) ReloadSetting(ctx context.Context) {
	u.invokeWebsocketMethod(ctx, "ReloadSetting", nil)
}

func (u *uiImpl) UpdateResult(ctx context.Context, result interface{}) bool {
	// Type assert to plugin.UpdatableResult
	// We use interface{} in the signature to avoid circular dependency between common and plugin packages
	response, err := u.invokeWebsocketMethod(ctx, "UpdateResult", result)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("UpdateResult error: %s", err.Error()))
		return false
	}

	// The UI returns true if the result was found and updated, false otherwise
	if response == nil {
		return false
	}

	success, ok := response.(bool)
	if !ok {
		logger.Error(ctx, "UpdateResult response is not a boolean")
		return false
	}

	return success
}

func (u *uiImpl) PushResults(ctx context.Context, payload interface{}) bool {
	response, err := u.invokeWebsocketMethod(ctx, "PushResults", payload)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("PushResults error: %s", err.Error()))
		return false
	}
	if response == nil {
		return false
	}
	success, ok := response.(bool)
	if !ok {
		logger.Error(ctx, "PushResults response is not a boolean")
		return false
	}
	return success
}

func (u *uiImpl) IsVisible(ctx context.Context) bool {
	// Return cached visibility state instead of querying UI via WebSocket
	// The state is updated by PostOnShow/PostOnHide callbacks
	return u.isVisible
}

func (u *uiImpl) PickFiles(ctx context.Context, params common.PickFilesParams) []string {
	respData, err := u.invokeWebsocketMethod(ctx, "PickFiles", params)
	if err != nil {
		return nil
	}
	if _, ok := respData.([]any); !ok {
		logger.Error(ctx, fmt.Sprintf("pick files response data type error: %T", respData))
		return nil
	}

	var result []string
	lo.ForEach(respData.([]any), func(file any, _ int) {
		result = append(result, file.(string))
	})
	return result
}

func (u *uiImpl) CaptureScreenshot(ctx context.Context, request common.CaptureScreenshotRequest) (common.CaptureScreenshotResult, error) {
	if request.SessionId == "" {
		// The UI request itself needs a stable session identifier so Flutter can correlate this long-lived
		// screenshot session with the same window instance that owns the current query/action context.
		request.SessionId = util.GetContextSessionId(ctx)
	}
	if request.ExportFilePath == "" {
		// Screenshot export now depends on a backend-owned file target so Flutter writes into the
		// same woxDataDirectory policy regardless of which Go caller initiated the session.
		exportFilePath, err := reserveScreenshotExportFilePath()
		if err != nil {
			return common.CaptureScreenshotResult{}, err
		}
		request.ExportFilePath = exportFilePath
	}

	respData, err := u.invokeWebsocketMethod(ctx, "CaptureScreenshot", request)
	if err != nil {
		return common.CaptureScreenshotResult{}, err
	}

	result, mapErr := decodeWebsocketResponse[common.CaptureScreenshotResult](respData)
	if mapErr != nil {
		return common.CaptureScreenshotResult{}, mapErr
	}

	return result, nil
}

func (u *uiImpl) invokeWebsocketMethod(ctx context.Context, method string, data any) (responseData any, responseErr error) {
	requestID := uuid.NewString()
	resultChan := make(chan WebsocketMsg)
	u.requestMap.Store(requestID, resultChan)
	defer u.requestMap.Delete(requestID)

	traceId := util.GetContextTraceId(ctx)
	sessionId := util.GetContextSessionId(ctx)

	err := requestUI(ctx, WebsocketMsg{
		RequestId: requestID,
		TraceId:   traceId,
		SessionId: sessionId,
		Method:    method,
		Data:      data,
	})
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("send message to UI error: %s", err.Error()))
		return "", err
	}

	timeout := getWebsocketMethodTimeout(method)
	select {
	case <-time.NewTimer(timeout).C:
		logger.Error(ctx, fmt.Sprintf("invoke ui method %s response timeout", method))
		return "", fmt.Errorf("request timeout, request id: %s", requestID)
	case response := <-resultChan:
		if !response.Success {
			return response.Data, errors.New("ui method response error")
		} else {
			return response.Data, nil
		}
	}
}

func getWebsocketMethodTimeout(method string) time.Duration {
	switch method {
	case "PickFiles", "CaptureScreenshot":
		// File pickers and screenshot sessions both wait on direct user interaction,
		// so the previous fixed 2s request timeout was not enough for these long-lived UI tasks.
		return 180 * time.Second
	default:
		return 2 * time.Second
	}
}

func decodeWebsocketResponse[T any](data any) (T, error) {
	var target T

	jsonBytes, marshalErr := json.Marshal(data)
	if marshalErr != nil {
		return target, fmt.Errorf("marshal websocket response failed: %w", marshalErr)
	}
	if unmarshalErr := json.Unmarshal(jsonBytes, &target); unmarshalErr != nil {
		return target, fmt.Errorf("unmarshal websocket response failed: %w", unmarshalErr)
	}

	return target, nil
}

func getShowAppParams(ctx context.Context, showContext common.ShowContext) map[string]any {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	var position Position
	showQueryBox := !showContext.HideQueryBox
	hideToolbar := showContext.HideToolbar
	showSource := showContext.ShowSource
	windowWidth := showContext.WindowWidth
	maxResultCount := showContext.MaxResultCount

	if showSource == "" {
		showSource = common.ShowSourceDefault
	}
	if windowWidth <= 0 {
		windowWidth = woxSetting.AppWidth.Get()
	}

	// if specific position provided, use it
	if showContext.WindowPosition != nil {
		position = Position{
			Type: setting.PositionTypeLastLocation,
			X:    showContext.WindowPosition.X,
			Y:    showContext.WindowPosition.Y,
		}
	} else {
		switch woxSetting.ShowPosition.Get() {
		case setting.PositionTypeActiveScreen:
			position = NewActiveScreenPositionWithOptions(ctx, windowWidth, maxResultCount, showQueryBox, !hideToolbar)
		case setting.PositionTypeLastLocation:
			// Use saved window position if available, otherwise use mouse screen position as fallback
			if woxSetting.LastWindowX.Get() != -1 && woxSetting.LastWindowY.Get() != -1 {
				logger.Info(ctx, fmt.Sprintf("Using saved window position: x=%d, y=%d", woxSetting.LastWindowX.Get(), woxSetting.LastWindowY.Get()))
				position = NewLastLocationPosition(woxSetting.LastWindowX.Get(), woxSetting.LastWindowY.Get())
			} else {
				logger.Info(ctx, "No saved window position, using mouse screen position as fallback")
				// No saved position, fallback to mouse screen position
				position = NewMouseScreenPositionWithOptions(ctx, windowWidth, maxResultCount, showQueryBox, !hideToolbar)
			}
		default: // Default to mouse screen
			position = NewMouseScreenPositionWithOptions(ctx, windowWidth, maxResultCount, showQueryBox, !hideToolbar)
		}
	}

	params := map[string]any{
		"SelectAll":            showContext.SelectAll,
		"IsQueryFocus":         showContext.IsQueryFocus,
		"HideQueryBox":         showContext.HideQueryBox,
		"HideToolbar":          hideToolbar,
		"QueryBoxAtBottom":     showContext.QueryBoxAtBottom,
		"HideOnBlur":           showContext.HideOnBlur,
		"Position":             position,
		"TrayAnchor":           showContext.TrayAnchor,
		"WindowWidth":          windowWidth,
		"MaxResultCount":       maxResultCount,
		"QueryHistories":       setting.GetSettingManager().GetLatestQueryHistory(ctx, 10),
		"LaunchMode":           woxSetting.LaunchMode.Get(),
		"StartPage":            woxSetting.StartPage.Get(),
		"ShowSource":           showSource,
		"ActivationStartedAt":  showContext.ActivationStartedAt,
		"AttentionUnreadCount": getAttentionUnreadCount(ctx),
	}

	return params
}

func getAttentionUnreadCount(ctx context.Context) int {
	count, err := plugin.GetAttentionManager().UnreadCount(ctx)
	if err != nil {
		logger.Warn(ctx, fmt.Sprintf("failed to count unread attention items for show app: %v", err))
		return 0
	}
	return int(count)
}

func onUIWebsocketRequest(ctx context.Context, request WebsocketMsg) {
	if request.Method != "Log" {
		logger.Debug(ctx, fmt.Sprintf("got <%s> request from ui", request.Method))
	}

	// we handle time/amount sensitive requests in websocket, other requests in http (see router.go)
	switch request.Method {
	case "Log":
		handleWebsocketLog(ctx, request)
	case "Query":
		handleWebsocketQuery(ctx, request)
	case "QueryMRU":
		handleWebsocketQueryMRU(ctx, request)
	case "Action":
		handleWebsocketAction(ctx, request)
	case "FormAction":
		handleWebsocketFormAction(ctx, request)
	case "ToolbarMsgAction":
		handleWebsocketToolbarMsgAction(ctx, request)
	case "TerminalSubscribe":
		handleWebsocketTerminalSubscribe(ctx, request)
	case "TerminalUnsubscribe":
		handleWebsocketTerminalUnsubscribe(ctx, request)
	case "TerminalSearch":
		handleWebsocketTerminalSearch(ctx, request)
	}
}

func onUIWebsocketResponse(ctx context.Context, response WebsocketMsg) {
	// ShowToolbarMsg acknowledgements arrive at very high frequency during file
	// indexing, and logging each one added noise without helping diagnose UI
	// behavior because the request side already knows which toolbar snapshot it sent.
	if response.Method != "ShowToolbarMsg" {
		logger.Debug(ctx, fmt.Sprintf("got <%s> response from ui", response.Method))
	}

	requestID := response.RequestId
	if requestID == "" {
		logger.Error(ctx, "response id not found")
		return
	}

	resultChan, exist := GetUIManager().GetUI(ctx).(*uiImpl).requestMap.Load(requestID)
	if !exist {
		logger.Error(ctx, fmt.Sprintf("response id not found: %s", requestID))
		return
	}

	resultChan <- response
}

func handleWebsocketLog(ctx context.Context, request WebsocketMsg) {
	traceId, traceIdErr := getWebsocketMsgParameter(ctx, request, "traceId")
	if traceIdErr != nil {
		logger.Error(ctx, traceIdErr.Error())
		responseUIError(ctx, request, traceIdErr.Error())
		return
	}
	level, levelErr := getWebsocketMsgParameter(ctx, request, "level")
	if levelErr != nil {
		logger.Error(ctx, levelErr.Error())
		responseUIError(ctx, request, levelErr.Error())
		return
	}
	message, messageErr := getWebsocketMsgParameter(ctx, request, "message")
	if messageErr != nil {
		logger.Error(ctx, messageErr.Error())
		responseUIError(ctx, request, messageErr.Error())
		return
	}

	// UI log should use its own conponent name
	logCtx := util.WithComponentContext(util.NewTraceContextWith(traceId), " UI")

	switch level {
	case "debug":
		logger.Debug(logCtx, message)
	case "info":
		logger.Info(logCtx, message)
	case "warn":
		logger.Warn(logCtx, message)
	case "error":
		logger.Error(logCtx, message)
	default:
		logger.Error(ctx, fmt.Sprintf("unsupported log level: %s", level))
		responseUIError(ctx, request, fmt.Sprintf("unsupported log level: %s", level))
	}
	responseUISuccess(ctx, request)
}

func handleWebsocketQuery(ctx context.Context, request WebsocketMsg) {
	sessionId := request.SessionId
	queryId, queryIdErr := getWebsocketMsgParameter(ctx, request, "queryId")
	if queryIdErr != nil {
		logger.Error(ctx, queryIdErr.Error())
		responseUIError(ctx, request, queryIdErr.Error())
		return
	} else {
		ctx = util.WithQueryIdContext(ctx, queryId)
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
	var querySelection selection.Selection
	json.Unmarshal([]byte(querySelectionJson), &querySelection)

	queryRefinements := map[string]string{}
	queryRequestJson, queryRequestMarshalErr := json.Marshal(request.Data)
	if queryRequestMarshalErr != nil {
		logger.Error(ctx, queryRequestMarshalErr.Error())
		responseUIError(ctx, request, queryRequestMarshalErr.Error())
		return
	}
	refinementsData := gjson.GetBytes(queryRequestJson, "queryRefinements")
	if refinementsData.Exists() {
		// queryRefinements is optional for compatibility with older UI clients.
		// When present, keep the map value shape simple and let each plugin
		// interpret single or comma-separated multi-select values.
		parsedRefinements, parseRefinementsErr := parseQueryRefinementsFromUI(refinementsData.Raw)
		if parseRefinementsErr != nil {
			logger.Error(ctx, parseRefinementsErr.Error())
			responseUIError(ctx, request, parseRefinementsErr.Error())
			return
		}
		queryRefinements = parsedRefinements
	}
	skipCompletionHint := gjson.GetBytes(queryRequestJson, "skipCompletionHint").Bool()

	var changedQuery common.PlainQuery
	switch queryType {
	case plugin.QueryTypeInput:
		changedQuery = common.PlainQuery{
			QueryId:          queryId,
			QueryType:        plugin.QueryTypeInput,
			QueryText:        queryText,
			QueryRefinements: queryRefinements,
		}
	case plugin.QueryTypeSelection:
		changedQuery = common.PlainQuery{
			QueryId:          queryId,
			QueryType:        plugin.QueryTypeSelection,
			QueryText:        queryText,
			QuerySelection:   querySelection,
			QueryRefinements: queryRefinements,
		}
	default:
		logger.Error(ctx, fmt.Sprintf("unsupported query type: %s", queryType))
		responseUIError(ctx, request, fmt.Sprintf("unsupported query type: %s", queryType))
		return
	}

	logger.Info(ctx, fmt.Sprintf("start to handle query changed: %s, queryId: %s", changedQuery.String(), queryId))

	if changedQuery.QueryType == plugin.QueryTypeInput && changedQuery.QueryText == "" {
		emptyInputQuery := plugin.Query{
			Id:        queryId,
			SessionId: sessionId,
			Type:      plugin.QueryTypeInput,
		}
		plugin.GetPluginManager().HandleQueryLifecycle(ctx, emptyInputQuery, nil)
		// Bug fix: blank-page empty input still occupies the global query box.
		// The previous zero-value QueryContext serialized as IsGlobalQuery=false,
		// so the UI treated a cleared search as plugin/selection context and hid
		// Glance. Return the same backend-owned classification used by normal
		// queries so clearing search keeps the global accessory visible.
		responseUIQueryResponse(ctx, request, queryId, plugin.QueryResponseUI{
			Results: []plugin.QueryResultUI{},
			Context: plugin.BuildQueryContext(emptyInputQuery, nil),
		}, true)
		return
	}
	if changedQuery.QueryType == plugin.QueryTypeSelection && changedQuery.QuerySelection.String() == "" {
		plugin.GetPluginManager().HandleQueryLifecycle(ctx, plugin.Query{
			Id:        queryId,
			SessionId: sessionId,
			Type:      plugin.QueryTypeSelection,
		}, nil)
		responseUIQueryResults(ctx, request, queryId, []plugin.QueryResultUI{}, true)
		return
	}

	query, ownerPlugin, queryErr := plugin.GetPluginManager().NewQuery(ctx, changedQuery)
	if queryErr != nil {
		if conflictErr, ok := plugin.AsTriggerKeywordConflictError(queryErr); ok {
			plugin.GetPluginManager().HandleQueryLifecycle(ctx, query, nil)
			responseUIQueryResponse(ctx, request, queryId, plugin.GetPluginManager().BuildTriggerKeywordConflictResponse(ctx, query, conflictErr.Conflict), true)
			return
		}
		logger.Error(ctx, queryErr.Error())
		responseUIError(ctx, request, queryErr.Error())
		return
	}

	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if !skipCompletionHint && woxSetting.EnableQueryCompletionHint.Get() {
		util.Go(ctx, "query completion hint", func() {
			responseUIQueryCompletionHint(
				ctx,
				request,
				queryId,
				plugin.BuildQueryCompletionHintForInputPrefix(query, ownerPlugin, woxSetting.QueryHistories.Get(), changedQuery.QueryText),
			)
		})
	}

	plugin.GetPluginManager().HandleQueryLifecycle(ctx, query, ownerPlugin)

	newQueryRun(ctx, request, query, ownerPlugin).start()
}

func queryPipelinePluginLabel(ctx context.Context, pluginInstance *plugin.Instance) string {
	if pluginInstance == nil {
		return "<global>"
	}
	name := pluginInstance.GetName(ctx)
	if name == "" {
		name = pluginInstance.Metadata.Id
	}
	return fmt.Sprintf("%s(%s)", name, pluginInstance.Metadata.Id)
}

func appendQueryDebugTails(ctx context.Context, sessionId string, queryId string, snapshot []plugin.QueryResultUI) []plugin.QueryResultUI {
	if len(snapshot) == 0 {
		return snapshot
	}
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if !woxSetting.ShowPerformanceTail.Get() {
		return snapshot
	}

	annotated := make([]plugin.QueryResultUI, len(snapshot))
	for i, result := range snapshot {
		if result.IsGroup {
			annotated[i] = result
			continue
		}

		resultCopy := result
		resultCopy.Tails = append([]plugin.QueryResultTail{}, result.Tails...)
		if batch, queryElapsed, ok := plugin.GetPluginManager().GetQueryResultDebugInfo(sessionId, queryId, result.Id); ok {
			category := plugin.QueryResultTailTextCategoryDefault
			if queryElapsed > 10 {
				category = plugin.QueryResultTailTextCategoryWarning
			}
			if queryElapsed > 20 {
				category = plugin.QueryResultTailTextCategoryDanger
			}

			resultCopy.Tails = append(
				resultCopy.Tails,
				plugin.NewQueryResultTailText(fmt.Sprintf("P%d", batch)),
				plugin.NewQueryResultTailTextWithCategory(fmt.Sprintf("%dms", queryElapsed), category),
			)
		}
		annotated[i] = resultCopy
	}

	return annotated
}

func handleWebsocketAction(ctx context.Context, request WebsocketMsg) {
	sessionId := request.SessionId
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
	queryId, queryErr := getWebsocketMsgParameter(ctx, request, "queryId")
	if queryErr != nil {
		logger.Error(ctx, queryErr.Error())
		responseUIError(ctx, request, queryErr.Error())
		return
	}

	actionCtx := util.WithQueryIdContext(util.WithSessionContext(ctx, sessionId), queryId)
	executeErr := plugin.GetPluginManager().ExecuteAction(actionCtx, sessionId, queryId, resultId, actionId)
	if executeErr != nil {
		responseUIError(ctx, request, executeErr.Error())
		return
	}

	responseUISuccess(ctx, request)
}

func handleWebsocketFormAction(ctx context.Context, request WebsocketMsg) {
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
	values, valuesErr := getWebsocketMsgParameterMap(ctx, request, "values")
	if valuesErr != nil {
		logger.Error(ctx, valuesErr.Error())
		responseUIError(ctx, request, valuesErr.Error())
		return
	}
	queryId, queryErr := getWebsocketMsgParameter(ctx, request, "queryId")
	if queryErr != nil {
		logger.Error(ctx, queryErr.Error())
		responseUIError(ctx, request, queryErr.Error())
		return
	} else {
		ctx = util.WithQueryIdContext(ctx, queryId)
	}

	executeErr := plugin.GetPluginManager().SubmitFormAction(ctx, request.SessionId, queryId, resultId, actionId, values)
	if executeErr != nil {
		responseUIError(ctx, request, executeErr.Error())
		return
	}

	responseUISuccess(ctx, request)
}

func handleWebsocketToolbarMsgAction(ctx context.Context, request WebsocketMsg) {
	toolbarMsgId, statusErr := getWebsocketMsgParameter(ctx, request, "toolbarMsgId")
	if statusErr != nil {
		logger.Error(ctx, statusErr.Error())
		responseUIError(ctx, request, statusErr.Error())
		return
	}

	actionId, actionErr := getWebsocketMsgParameter(ctx, request, "actionId")
	if actionErr != nil {
		logger.Error(ctx, actionErr.Error())
		responseUIError(ctx, request, actionErr.Error())
		return
	}

	executeErr := plugin.GetPluginManager().ExecuteToolbarMsgAction(ctx, request.SessionId, toolbarMsgId, actionId)
	if executeErr != nil {
		responseUIError(ctx, request, executeErr.Error())
		return
	}

	responseUISuccess(ctx, request)
}

func handleWebsocketQueryMRU(ctx context.Context, request WebsocketMsg) {
	queryId, _ := getWebsocketMsgParameter(ctx, request, "queryId")
	mruResults := plugin.GetPluginManager().QueryMRU(ctx, request.SessionId, queryId)
	logger.Info(ctx, fmt.Sprintf("found %d MRU results via websocket", len(mruResults)))
	responseUISuccessWithData(ctx, request, mruResults)
}

func handleWebsocketTerminalSubscribe(ctx context.Context, request WebsocketMsg) {
	dataMap, ok := request.Data.(map[string]any)
	if !ok {
		responseUIError(ctx, request, "invalid terminal subscribe payload")
		return
	}

	sessionID, _ := dataMap["sessionId"].(string)
	if sessionID == "" {
		responseUIError(ctx, request, "sessionId is required")
		return
	}

	cursor := int64(0)
	if rawCursor, exists := dataMap["cursor"]; exists {
		switch value := rawCursor.(type) {
		case float64:
			cursor = int64(value)
		case int64:
			cursor = value
		case int:
			cursor = int64(value)
		}
	}

	state, err := terminal.GetSessionManager().Subscribe(ctx, request.SessionId, sessionID, cursor)
	if err != nil {
		responseUIError(ctx, request, err.Error())
		return
	}

	responseUISuccessWithData(ctx, request, state)
}

func handleWebsocketTerminalUnsubscribe(ctx context.Context, request WebsocketMsg) {
	dataMap, ok := request.Data.(map[string]any)
	if !ok {
		responseUIError(ctx, request, "invalid terminal unsubscribe payload")
		return
	}

	sessionID, _ := dataMap["sessionId"].(string)
	if sessionID == "" {
		responseUIError(ctx, request, "sessionId is required")
		return
	}

	terminal.GetSessionManager().Unsubscribe(request.SessionId, sessionID)
	responseUISuccess(ctx, request)
}

func handleWebsocketTerminalSearch(ctx context.Context, request WebsocketMsg) {
	dataMap, ok := request.Data.(map[string]any)
	if !ok {
		responseUIError(ctx, request, "invalid terminal search payload")
		return
	}

	sessionID, _ := dataMap["sessionId"].(string)
	pattern, _ := dataMap["pattern"].(string)
	if sessionID == "" || pattern == "" {
		responseUIError(ctx, request, "sessionId and pattern are required")
		return
	}

	cursor := int64(0)
	if rawCursor, exists := dataMap["cursor"]; exists {
		switch value := rawCursor.(type) {
		case float64:
			cursor = int64(value)
		case int64:
			cursor = value
		case int:
			cursor = int64(value)
		}
	}
	backward, _ := dataMap["backward"].(bool)
	caseSensitive, _ := dataMap["caseSensitive"].(bool)

	result, err := terminal.GetSessionManager().Search(ctx, terminal.TerminalSearchRequest{
		SessionID:     sessionID,
		Pattern:       pattern,
		Cursor:        cursor,
		Backward:      backward,
		CaseSensitive: caseSensitive,
	})
	if err != nil {
		responseUIError(ctx, request, err.Error())
		return
	}
	responseUISuccessWithData(ctx, request, result)
}

func getWebsocketMsgParameter(ctx context.Context, msg WebsocketMsg, key string) (string, error) {
	jsonData, marshalErr := json.Marshal(msg.Data)
	if marshalErr != nil {
		return "", marshalErr
	}

	paramterData := gjson.GetBytes(jsonData, key)
	if !paramterData.Exists() {
		return "", fmt.Errorf("%s parameter not found", key)
	}

	return paramterData.String(), nil
}

func getWebsocketMsgParameterMap(ctx context.Context, msg WebsocketMsg, key string) (map[string]string, error) {
	jsonData, marshalErr := json.Marshal(msg.Data)
	if marshalErr != nil {
		return nil, marshalErr
	}

	paramterData := gjson.GetBytes(jsonData, key)
	if !paramterData.Exists() {
		return nil, fmt.Errorf("%s parameter not found", key)
	}
	if !paramterData.IsObject() {
		return nil, fmt.Errorf("%s parameter must be an object", key)
	}

	var values map[string]string
	if unmarshalErr := json.Unmarshal([]byte(paramterData.Raw), &values); unmarshalErr != nil {
		return nil, unmarshalErr
	}

	return values, nil
}
