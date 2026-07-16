package ui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"strings"
	"wox/cloudsync"
	"wox/common"
	"wox/plugin"
	"wox/setting"
	"wox/ui/contract"
	"wox/util"
	"wox/util/notifier"
	"wox/util/timetracking"
)

type uiImpl struct {
	isVisible          bool // cached visibility state, updated by PostOnShow/PostOnHide
	isInSettingView    bool // cached setting-view state, updated by PostOnSetting
	isInOnboardingView bool // cached onboarding state, updated by PostOnOnboarding
	isRecordingHotkey  bool // cached hotkey-recorder focus state, updated by PostOnHotkeyRecording
}

func (u *uiImpl) ChangeQuery(ctx context.Context, query common.PlainQuery) {
	u.applyView(ctx, "change query", func(view contract.View) error { return view.ChangeQuery(ctx, query) })
}

func (u *uiImpl) RefreshQuery(ctx context.Context, preserveSelectedIndex bool) {
	u.applyView(ctx, "refresh query", func(view contract.View) error { return view.RefreshQuery(ctx, preserveSelectedIndex) })
}

func (u *uiImpl) RefreshGlance(ctx context.Context, pluginId string, ids []string) {
	u.applyView(ctx, "refresh glance", func(view contract.View) error { return view.RefreshGlance(ctx, pluginId, ids) })
}

func (u *uiImpl) UpdateDiagnosticStatus(ctx context.Context, enabled bool) {
	// New feature: bug aware status is a global launcher decoration, so core
	// pushes it separately from plugin toolbar messages to avoid ownership
	// conflicts with normal plugin status updates.
	u.applyView(ctx, "update diagnostic status", func(view contract.View) error { return view.UpdateDiagnosticStatus(ctx, enabled) })
}

func (u *uiImpl) HideApp(ctx context.Context) {
	u.applyView(ctx, "hide app", func(view contract.View) error { return view.Hide(ctx) })
}

func (u *uiImpl) ShowApp(ctx context.Context, showContext common.ShowContext) {
	GetUIManager().RefreshActiveWindowSnapshot(ctx)
	options := getShowOptions(ctx, showContext)
	u.applyView(ctx, "show app", func(view contract.View) error { return view.Show(ctx, options) })
}

func (u *uiImpl) ToggleApp(ctx context.Context, showContext common.ShowContext) {
	GetUIManager().RefreshActiveWindowSnapshot(ctx)
	options := getShowOptions(ctx, showContext)
	u.applyView(ctx, "toggle app", func(view contract.View) error { return view.Toggle(ctx, options) })
}

func (u *uiImpl) RecordHotkey(ctx context.Context, hotkey string, kind string) {
	logger.Info(ctx, fmt.Sprintf("send RecordHotkey to UI: hotkey=%s kind=%s", hotkey, kind))
	u.applyView(ctx, "record hotkey", func(view contract.View) error { return view.RecordHotkey(ctx, hotkey, kind) })
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
	// UI.
	effectiveTheme := GetUIManager().resolvePlatformTheme(ctx, theme)
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	woxSetting.ThemeId.Set(effectiveTheme.ThemeId)
	u.applyView(ctx, "change theme", func(view contract.View) error { return view.ChangeTheme(ctx, effectiveTheme) })
}

// ChangeThemeWithoutSave applies the theme without saving to settings
// This is used for auto appearance theme switching
func (u *uiImpl) ChangeThemeWithoutSave(ctx context.Context, theme common.Theme) {
	logger.Info(ctx, fmt.Sprintf("change theme (without save): %s", theme.ThemeName))
	effectiveTheme := GetUIManager().resolvePlatformTheme(ctx, theme)
	u.applyView(ctx, "change theme without save", func(view contract.View) error { return view.ChangeTheme(ctx, effectiveTheme) })
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
	u.applyView(ctx, "open setting", func(view contract.View) error { return view.OpenSetting(ctx, windowContext) })
}

func (u *uiImpl) FocusSettingWindow(ctx context.Context) {
	u.applyView(ctx, "focus setting", func(view contract.View) error { return view.FocusSetting(ctx) })
}

func (u *uiImpl) OpenOnboardingWindow(ctx context.Context) {
	// Onboarding reuses the same UI process and command path as the
	// settings window. Keeping it here avoids a second desktop window lifecycle
	// while still letting UI choose the dedicated onboarding view.
	u.applyView(ctx, "open onboarding", func(view contract.View) error { return view.OpenOnboarding(ctx) })
}

// OpenMacOSPermissionFlow asks the UI host to present the native permission guide for one permission.
func (u *uiImpl) OpenMacOSPermissionFlow(ctx context.Context, permissionType string) {
	u.applyView(ctx, "open macOS permission flow", func(view contract.View) error { return view.OpenMacOSPermissionFlow(ctx, permissionType) })
}

func (u *uiImpl) GetAllThemes(ctx context.Context) []common.Theme {
	return GetUIManager().GetAllThemes(ctx)
}

func (u *uiImpl) RestoreTheme(ctx context.Context) {
	GetUIManager().RestoreTheme(ctx)
}

func (u *uiImpl) Notify(ctx context.Context, msg common.NotifyMsg) {
	if u.IsVisible(ctx) && !u.IsInManagementView() && !plugin.GetPluginManager().HasVisibleToolbarMsg(ctx) {
		u.applyView(ctx, "show notification message", func(view contract.View) error { return view.ShowNotificationMessage(ctx, msg) })
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
	u.applyView(ctx, "update attention unread count", func(view contract.View) error { return view.UpdateAttentionUnreadCount(ctx, unreadCount) })
}

func (u *uiImpl) ShowToolbarMsg(ctx context.Context, msg interface{}) {
	message, ok := msg.(plugin.ToolbarMsgUI)
	if !ok {
		logger.Error(ctx, fmt.Sprintf("invalid toolbar message type: %T", msg))
		return
	}
	u.applyView(ctx, "show toolbar message", func(view contract.View) error { return view.ShowToolbarMessage(ctx, message) })
}

func (u *uiImpl) ClearToolbarMsg(ctx context.Context, toolbarMsgId string) {
	u.applyView(ctx, "clear toolbar message", func(view contract.View) error { return view.ClearToolbarMessage(ctx, toolbarMsgId) })
}

func (u *uiImpl) IsInSettingView() bool {
	return u.isInSettingView
}

func (u *uiImpl) IsInManagementView() bool {
	// Settings and onboarding both occupy the shared Wox window as management
	// surfaces, so toolbar notifications should not overlay either of them.
	return u.isInSettingView || u.isInOnboardingView
}

func (u *uiImpl) GetActiveWindowSnapshot(ctx context.Context) common.ActiveWindowSnapshot {
	return GetUIManager().GetActiveWindowSnapshot(ctx)
}

func (u *uiImpl) SendChatResponse(ctx context.Context, aiChatData common.AIChatData) {
	u.applyView(ctx, "send chat response", func(view contract.View) error { return view.SendChatResponse(ctx, aiChatData) })
}

func (u *uiImpl) ReloadChatResources(ctx context.Context, resouceName string) {
	u.applyView(ctx, "reload chat resources", func(view contract.View) error { return view.ReloadChatResources(ctx, resouceName) })
}

// SendAIQuestion pushes a question to the UI. The answer comes back via the
// /ai/question/answer HTTP route, which resolves the pending ask_user channel.
func (u *uiImpl) SendAIQuestion(ctx context.Context, questionId string, question string, options []common.AIQuestionOption) {
	u.applyView(ctx, "send AI question", func(view contract.View) error { return view.SendAIQuestion(ctx, questionId, question, options) })
}

func (u *uiImpl) ReloadSettingPlugins(ctx context.Context) {
	u.applyView(ctx, "reload setting plugins", func(view contract.View) error { return view.ReloadSettingPlugins(ctx) })
}

func (u *uiImpl) ReloadSetting(ctx context.Context) {
	u.applyView(ctx, "reload setting", func(view contract.View) error { return view.ReloadSetting(ctx) })
}

func (u *uiImpl) ReloadSettingThemes(ctx context.Context) {
	u.applyView(ctx, "reload setting themes", func(view contract.View) error { return view.ReloadSettingThemes(ctx) })
}

func (u *uiImpl) CloudSyncProgressChanged(ctx context.Context, progress any) {
	typedProgress, ok := progress.(cloudsync.CloudSyncProgress)
	if !ok {
		logger.Error(ctx, fmt.Sprintf("invalid cloud sync progress type: %T", progress))
		return
	}
	u.applyView(ctx, "cloud sync progress changed", func(view contract.View) error { return view.CloudSyncProgressChanged(ctx, typedProgress) })
}

func (u *uiImpl) RefreshAccountStatus(ctx context.Context) {
	u.applyView(ctx, "refresh account status", func(view contract.View) error { return view.RefreshAccountStatus(ctx) })
}

func (u *uiImpl) UpdateResult(ctx context.Context, result interface{}) bool {
	// Type assert to plugin.UpdatableResult
	// We use interface{} in the signature to avoid circular dependency between common and plugin packages
	typedResult, ok := result.(plugin.UpdatableResult)
	if !ok {
		logger.Error(ctx, fmt.Sprintf("invalid update result type: %T", result))
		return false
	}
	view, err := u.getView()
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("UpdateResult error: %s", err.Error()))
		return false
	}
	success, err := view.UpdateResult(ctx, typedResult)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("UpdateResult error: %s", err.Error()))
		return false
	}
	return success
}

func (u *uiImpl) PushResults(ctx context.Context, payload interface{}) bool {
	typedPayload, ok := payload.(plugin.PushResultsPayload)
	if !ok {
		logger.Error(ctx, fmt.Sprintf("invalid push results type: %T", payload))
		return false
	}
	view, err := u.getView()
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("PushResults error: %s", err.Error()))
		return false
	}
	success, err := view.PushResults(ctx, typedPayload)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("PushResults error: %s", err.Error()))
		return false
	}
	return success
}

func (u *uiImpl) IsVisible(ctx context.Context) bool {
	// Return cached visibility state instead of querying the UI.
	// The state is updated by PostOnShow/PostOnHide callbacks
	return u.isVisible
}

// ToggleRecordingMode asks the macOS UI to switch between launcher and capture-friendly window levels.
func (u *uiImpl) ToggleRecordingMode(ctx context.Context) (bool, error) {
	view, err := u.getView()
	if err != nil {
		return false, err
	}
	return view.ToggleRecordingMode(ctx)
}

func (u *uiImpl) PickFiles(ctx context.Context, params common.PickFilesParams) []string {
	view, err := u.getView()
	if err != nil {
		return nil
	}
	result, err := view.PickFiles(ctx, params)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("pick files failed: %v", err))
		return nil
	}
	return result
}

func (u *uiImpl) CaptureScreenshot(ctx context.Context, request common.CaptureScreenshotRequest) (common.CaptureScreenshotResult, error) {
	if request.SessionId == "" {
		// The UI request itself needs a stable session identifier so UI can correlate this long-lived
		// screenshot session with the same window instance that owns the current query/action context.
		request.SessionId = util.GetContextSessionId(ctx)
	}
	if request.ExportFilePath == "" {
		// Screenshot export now depends on a backend-owned file target so UI writes into the
		// same woxDataDirectory policy regardless of which Go caller initiated the session.
		exportFilePath, err := reserveScreenshotExportFilePath()
		if err != nil {
			return common.CaptureScreenshotResult{}, err
		}
		request.ExportFilePath = exportFilePath
	}

	view, err := u.getView()
	if err != nil {
		return common.CaptureScreenshotResult{}, err
	}
	return view.CaptureScreenshot(ctx, request)
}

// WriteClipboardImageFile delegates image clipboard ownership to the UI process.
func (u *uiImpl) WriteClipboardImageFile(ctx context.Context, filePath string) error {
	if strings.TrimSpace(filePath) == "" {
		return errors.New("clipboard image file path is empty")
	}

	view, err := u.getView()
	if err != nil {
		return err
	}
	return view.WriteClipboardImageFile(ctx, filePath)
}

func (u *uiImpl) getView() (contract.View, error) {
	view := GetUIManager().getView()
	if view == nil {
		return nil, errors.New("embedded UI view is not attached")
	}
	return view, nil
}

func (u *uiImpl) applyView(ctx context.Context, operation string, apply func(view contract.View) error) {
	view, err := u.getView()
	if err == nil {
		err = apply(view)
	}
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("%s failed: %v", operation, err))
	}
}

func getShowOptions(ctx context.Context, showContext common.ShowContext) contract.ShowOptions {
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

	return contract.ShowOptions{
		SelectAll: showContext.SelectAll, HideQueryBox: showContext.HideQueryBox, HideToolbar: hideToolbar,
		QueryBoxAtBottom: showContext.QueryBoxAtBottom, HideOnBlur: showContext.HideOnBlur,
		Position:    contract.Position{Type: string(position.Type), X: position.X, Y: position.Y},
		WindowWidth: windowWidth, MaxResultCount: maxResultCount, LaunchMode: woxSetting.LaunchMode.Get(),
		StartPage: woxSetting.StartPage.Get(), ShowSource: string(showSource),
	}
}

// runUIQuery executes one decoded query and reports snapshots through a typed view.
func runUIQuery(ctx context.Context, request contract.QueryRequest, view contract.QueryView) {
	handlerStart := util.GetSystemTimestamp()
	changedQuery := request.Query
	queryId := changedQuery.QueryId
	sessionId := request.SessionID
	ctx = util.WithQueryIdContext(util.WithSessionContext(ctx, sessionId), queryId)

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
		view.ApplyQueryResponse(ctx, contract.QueryResponse{QueryID: queryId, Response: plugin.QueryResponseUI{
			Results: []plugin.QueryResultUI{},
			Context: plugin.BuildQueryContext(emptyInputQuery, nil),
		}, IsFinal: true})
		return
	}
	if changedQuery.QueryType == plugin.QueryTypeSelection && changedQuery.QuerySelection.String() == "" {
		plugin.GetPluginManager().HandleQueryLifecycle(ctx, plugin.Query{
			Id:        queryId,
			SessionId: sessionId,
			Type:      plugin.QueryTypeSelection,
		}, nil)
		view.ApplyQueryResponse(ctx, contract.QueryResponse{QueryID: queryId, Response: plugin.QueryResponseUI{Results: []plugin.QueryResultUI{}}, IsFinal: true})
		return
	}

	newQueryStart := util.GetSystemTimestamp()
	query, ownerPlugin, queryErr := plugin.GetPluginManager().NewQuery(ctx, changedQuery)
	if queryErr != nil {
		if conflictErr, ok := plugin.AsTriggerKeywordConflictError(queryErr); ok {
			plugin.GetPluginManager().HandleQueryLifecycle(ctx, query, nil)
			view.ApplyQueryResponse(ctx, contract.QueryResponse{QueryID: queryId, Response: plugin.GetPluginManager().BuildTriggerKeywordConflictResponse(ctx, query, conflictErr.Conflict), IsFinal: true})
			return
		}
		logger.Error(ctx, queryErr.Error())
		view.ApplyQueryError(ctx, queryId, queryErr)
		return
	}
	if tracker := timetracking.New("handle_query_new_query"); tracker.Enabled() {
		tracker.SetRawString("queryId", queryId)
		tracker.SetRawString("query", query.String())
		tracker.SetRawString("ownerPlugin", queryPipelinePluginLabel(ctx, ownerPlugin))
		tracker.SetInt64("costMs", util.GetSystemTimestamp()-newQueryStart)
		tracker.SetInt64("elapsedMs", util.GetSystemTimestamp()-handlerStart)
		tracker.Log(ctx)
	}

	completionHintScheduleStart := util.GetSystemTimestamp()
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if !request.SkipCompletionHint && woxSetting.EnableQueryCompletionHint.Get() {
		util.Go(ctx, "query completion hint", func() {
			view.ApplyQueryCompletionHint(
				ctx,
				queryId,
				plugin.BuildQueryCompletionHintForInputPrefixWithFeedback(
					query,
					ownerPlugin,
					setting.GetSettingManager().GetLatestQueryHistory(ctx, plugin.QueryCompletionHistoryLimit),
					setting.GetSettingManager().GetQueryCompletionFeedbacks(ctx),
					changedQuery.QueryText,
				),
			)
		})
	}
	if tracker := timetracking.New("handle_query_completion_hint_schedule"); tracker.Enabled() {
		tracker.SetRawString("queryId", queryId)
		tracker.SetBool("enabled", woxSetting.EnableQueryCompletionHint.Get())
		tracker.SetBool("skipped", request.SkipCompletionHint)
		tracker.SetInt64("costMs", util.GetSystemTimestamp()-completionHintScheduleStart)
		tracker.Log(ctx)
	}

	lifecycleStart := util.GetSystemTimestamp()
	plugin.GetPluginManager().HandleQueryLifecycle(ctx, query, ownerPlugin)
	if tracker := timetracking.New("handle_query_lifecycle"); tracker.Enabled() {
		tracker.SetRawString("queryId", queryId)
		tracker.SetInt64("costMs", util.GetSystemTimestamp()-lifecycleStart)
		tracker.SetInt64("elapsedMs", util.GetSystemTimestamp()-handlerStart)
		tracker.Log(ctx)
	}

	if tracker := timetracking.New("handle_query_run_starting"); tracker.Enabled() {
		tracker.SetRawString("queryId", queryId)
		tracker.SetInt64("elapsedMs", util.GetSystemTimestamp()-handlerStart)
		tracker.Log(ctx)
	}
	newQueryRun(ctx, request, view, query, ownerPlugin).start()
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

func appendQueryDebugTails(ctx context.Context, sessionId string, queryId string, snapshot []plugin.QueryResultUI, firstVisibleFlushElapsedMs int64, backendPreparedElapsedMs int64) []plugin.QueryResultUI {
	if len(snapshot) == 0 {
		return snapshot
	}
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if !woxSetting.ShowPerformanceTail.Get() {
		return snapshot
	}
	showBatchTail := woxSetting.ShowPerformanceTailBatch.Get()
	showPluginQueryTail := woxSetting.ShowPerformanceTailPluginQuery.Get()
	showBackendPreparedTail := woxSetting.ShowPerformanceTailBackendPrepared.Get()
	if !showBatchTail && !showPluginQueryTail && !showBackendPreparedTail {
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
		if batch, _, batchQueueElapsed, batchQueueElapsedSet, pluginQueryElapsed, pluginQueryElapsedSet, ok := plugin.GetPluginManager().GetQueryResultDebugInfo(sessionId, queryId, result.Id); ok {
			if showBatchTail {
				batchTail := plugin.NewQueryResultTailTextWithCategory(fmt.Sprintf("B%d", batch), queryDebugBatchTailTextCategory(batch))
				batchTail.Tooltip = fmt.Sprintf("First flush: %dms", firstVisibleFlushElapsedMs)
				if batchQueueElapsedSet {
					batchTail.Tooltip = fmt.Sprintf("First flush: %dms\nQueued for batch: %dms", firstVisibleFlushElapsedMs, batchQueueElapsed)
				}
				resultCopy.Tails = append(resultCopy.Tails, batchTail)
			}

			if showPluginQueryTail && pluginQueryElapsedSet {
				pluginQueryTail := plugin.NewQueryResultTailTextWithCategory(fmt.Sprintf("%dms", pluginQueryElapsed), queryDebugPluginQueryTailTextCategory(pluginQueryElapsed))
				pluginQueryTail.Tooltip = "Raw Plugin.Query duration"
				resultCopy.Tails = append(resultCopy.Tails, pluginQueryTail)
			}

			if showBackendPreparedTail {
				backendPreparedCategory := plugin.QueryResultTailTextCategoryDefault
				elapsedTail := plugin.NewQueryResultTailTextWithCategory(fmt.Sprintf("%dms", backendPreparedElapsedMs), backendPreparedCategory)
				elapsedTail.Tooltip = "Backend ready to send elapsed since UI query request"
				resultCopy.Tails = append(resultCopy.Tails, elapsedTail)
			}
		}
		annotated[i] = resultCopy
	}

	return annotated
}

// queryDebugBatchTailTextCategory highlights results that missed the first response batch.
func queryDebugBatchTailTextCategory(batch int) plugin.QueryResultTailTextCategory {
	if batch > 1 {
		return plugin.QueryResultTailTextCategoryWarning
	}
	return plugin.QueryResultTailTextCategoryDefault
}

// queryDebugPluginQueryTailTextCategory highlights raw plugin execution cost before backend/UI overhead.
func queryDebugPluginQueryTailTextCategory(elapsedMs int64) plugin.QueryResultTailTextCategory {
	if elapsedMs > 10 {
		return plugin.QueryResultTailTextCategoryDanger
	}
	if elapsedMs > 5 {
		return plugin.QueryResultTailTextCategoryWarning
	}
	return plugin.QueryResultTailTextCategoryDefault
}
