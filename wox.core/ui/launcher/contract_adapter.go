package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"wox/cloudsync"
	"wox/common"
	"wox/plugin"
	"wox/plugin/system/shell/terminal"
	"wox/ui/contract"
	woxui "wox/ui/runtime"
	"wox/util"
)

// SessionID identifies the launcher instance receiving core push updates.
func (a *App) SessionID() string {
	return a.sessionID
}

// Show presents the launcher without changing any independent management window.
func (a *App) Show(_ context.Context, options contract.ShowOptions) error {
	return a.showWindow(fromCoreShowOptions(options))
}

// Hide closes the visible launcher surface and reports the lifecycle transition.
func (a *App) Hide(_ context.Context) error {
	return a.hideWindow(true)
}

// Toggle changes launcher visibility using core-owned placement policy.
func (a *App) Toggle(_ context.Context, options contract.ShowOptions) error {
	visible := false
	if err := a.runOnUI("read launcher visibility", func() {
		visible = a.visible
	}); err != nil {
		return err
	}
	if visible {
		return a.hideWindow(true)
	}
	return a.showWindow(fromCoreShowOptions(options))
}

func fromCoreShowOptions(options contract.ShowOptions) showAppParams {
	return showAppParams{
		SelectAll: options.SelectAll,
		Position: position{
			Type: options.Position.Type,
			X:    options.Position.X,
			Y:    options.Position.Y,
		},
		WindowWidth: options.WindowWidth, MaxResultCount: options.MaxResultCount,
		LaunchMode: options.LaunchMode, StartPage: options.StartPage,
		HideQueryBox: options.HideQueryBox, HideToolbar: options.HideToolbar,
		QueryBoxAtBottom: options.QueryBoxAtBottom, HideOnBlur: options.HideOnBlur,
		ShowSource: options.ShowSource,
	}
}

// ChangeQuery replaces the launcher query and enters the normal typed query pipeline.
func (a *App) ChangeQuery(_ context.Context, query common.PlainQuery) error {
	converted := fromCorePlainQuery(query)
	if err := a.runOnUI("change query", func() {
		a.setQuery(converted)
	}); err != nil {
		return err
	}
	return a.sendCurrentQuery()
}

func fromCorePlainQuery(query common.PlainQuery) plainQuery {
	return plainQuery{
		QueryID:   query.QueryId,
		QueryType: query.QueryType,
		QueryText: query.QueryText,
		QuerySelection: selection{
			Type:      string(query.QuerySelection.Type),
			Text:      query.QuerySelection.Text,
			FilePaths: append([]string(nil), query.QuerySelection.FilePaths...),
		},
		QueryRefinements: cloneStringMap(query.QueryRefinements),
		ContextData:      cloneStringMap(map[string]string(query.ContextData)),
	}
}

// RefreshQuery starts a new query identity while optionally retaining the visible selection.
func (a *App) RefreshQuery(_ context.Context, preserveSelectedIndex bool) error {
	if err := a.runOnUI("refresh query", func() {
		selected := a.selected
		a.query.QueryID = newInputQuery("").QueryID
		a.queryContext = queryContext{}
		a.queryContextKnown = false
		a.completionHint = nil
		a.stopGlanceLocked(true)
		if !preserveSelectedIndex {
			a.pendingSelection = nil
			a.selected = -1
			a.resultScrollDetached = false
		} else {
			a.pendingSelection = &pendingResultSelection{queryID: a.query.QueryID, index: selected}
			a.selected = selected
		}
		a.reconcileSelectedPreview()
	}); err != nil {
		return err
	}
	return a.sendCurrentQuery()
}

// RefreshGlance schedules a stale-query-guarded accessory refresh.
func (a *App) RefreshGlance(_ context.Context, pluginID string, ids []string) error {
	util.Go(a.lifecycleCtx, "refresh glance manually", func() {
		a.refreshGlance("manualRefresh", pluginID, append([]string(nil), ids...))
	})
	return nil
}

// UpdateDiagnosticStatus is reserved for the launcher-wide diagnostics decoration.
func (a *App) UpdateDiagnosticStatus(_ context.Context, _ bool) error {
	return nil
}

// RecordHotkey applies a raw core recorder result to the active settings field.
func (a *App) RecordHotkey(_ context.Context, hotkey string, kind string) error {
	var applyErr error
	if err := a.runOnUI("record hotkey", func() {
		applyErr = a.applyRecordedHotkey(recordedHotkeyPayload{Hotkey: hotkey, Kind: kind})
	}); err != nil {
		return err
	}
	return applyErr
}

// ChangeTheme applies a platform-resolved core theme without transport serialization.
func (a *App) ChangeTheme(_ context.Context, theme common.Theme) error {
	converted := fromCoreTheme(theme)
	if err := a.runOnUI("change launcher theme", func() {
		a.applyTheme(converted)
	}); err != nil {
		return err
	}
	a.publishSettingsChanged("theme")
	return nil
}

// OpenSetting opens the independent management window at one explicit path.
func (a *App) OpenSetting(_ context.Context, windowContext common.SettingWindowContext) error {
	if !a.isPrimary && a.primary != nil {
		return a.primary.OpenSetting(context.Background(), windowContext)
	}
	return a.openSettings(settingWindowContext{Path: windowContext.Path, Param: windowContext.Param, Source: string(windowContext.Source)})
}

// FocusSetting raises the current settings window without changing its management state.
func (a *App) FocusSetting(_ context.Context) error {
	if !a.isPrimary && a.primary != nil {
		return a.primary.FocusSetting(context.Background())
	}
	var settingsView *woxui.ManagedWindow
	if err := a.runOnUI("resolve settings window for focus", func() {
		settingsView = a.settingsView
	}); err != nil {
		return err
	}
	if settingsView == nil {
		return nil
	}
	_, err := settingsView.Show()
	return err
}

// OpenOnboarding opens the first-run guide in its dedicated window.
func (a *App) OpenOnboarding(_ context.Context) error {
	if !a.isPrimary && a.primary != nil {
		return a.primary.OpenOnboarding(context.Background())
	}
	return a.openOnboarding()
}

// OpenMacOSPermissionFlow opens the native privacy surface after a user action.
func (a *App) OpenMacOSPermissionFlow(ctx context.Context, permissionType string) error {
	return a.services.OpenMacOSPermission(ctx, a.sessionID, permissionType)
}

// ShowToolbarMessage displays a plugin-owned persistent toolbar snapshot.
func (a *App) ShowToolbarMessage(_ context.Context, message plugin.ToolbarMsgUI) error {
	converted := fromCoreToolbarMessage(message)
	return a.runOnUI("show toolbar message", func() {
		a.applyToolbarMessage(converted)
	})
}

// ShowNotificationMessage displays one transient notification in the launcher toolbar.
func (a *App) ShowNotificationMessage(_ context.Context, message common.NotifyMsg) error {
	converted := toolbarMessage{Text: message.Text, Icon: imageFromString(message.Icon), DisplaySeconds: message.DisplaySeconds}
	return a.runOnUI("show notification message", func() {
		a.applyToolbarMessage(converted)
	})
}

// ClearToolbarMessage removes the matching plugin-owned toolbar snapshot.
func (a *App) ClearToolbarMessage(_ context.Context, toolbarMessageID string) error {
	return a.runOnUI("clear toolbar message", func() {
		a.clearToolbarMessageByID(toolbarMessageID)
	})
}

// UpdateAttentionUnreadCount is reserved for the launcher-wide attention decoration.
func (a *App) UpdateAttentionUnreadCount(_ context.Context, _ int) error {
	return nil
}

// SendChatResponse reconciles a core chat snapshot with the active preview.
func (a *App) SendChatResponse(_ context.Context, chat common.AIChatData) error {
	converted := fromCoreChatData(chat)
	return a.runOnUI("apply chat response", func() {
		a.applyChatResponse(converted)
	})
}

// ReloadChatResources invalidates the requested AI catalogs.
func (a *App) ReloadChatResources(_ context.Context, resourceName string) error {
	return a.runOnUI("reload chat resources", func() {
		a.reloadChatResourceName(resourceName)
	})
}

// SendAIQuestion opens the typed ask-user overlay for the active chat preview.
func (a *App) SendAIQuestion(_ context.Context, questionID string, question string, options []common.AIQuestionOption) error {
	converted := make([]aiQuestionOption, len(options))
	for index, option := range options {
		converted[index] = aiQuestionOption{Value: option.Value, Title: option.Title, SubTitle: option.SubTitle, Recommended: option.Recommended, Extra: cloneStringMap(option.Extra)}
	}
	var applyErr error
	if err := a.runOnUI("apply AI question", func() {
		applyErr = a.applyTypedAIQuestion(aiQuestion{QuestionID: questionID, Question: question, Options: converted})
	}); err != nil {
		return err
	}
	return applyErr
}

// ReloadSettingPlugins refreshes plugin-backed settings and glance catalogs.
func (a *App) ReloadSettingPlugins(_ context.Context) error {
	util.Go(a.lifecycleCtx, "reload settings plugins", a.reloadGlanceCatalogFromCore)
	a.publishSettingsChanged("plugins")
	return nil
}

// ReloadSetting refreshes settings, translations, and any eligible glance item.
func (a *App) ReloadSetting(_ context.Context) error {
	if err := a.reloadSettings(); err != nil {
		return err
	}
	if err := a.reloadTranslations(); err != nil {
		return err
	}
	a.stopGlanceLocked(true)
	refreshGlance := a.glanceEligibleLocked()
	if refreshGlance {
		util.Go(a.lifecycleCtx, "refresh glance after settings change", func() {
			a.refreshGlance("settingsChanged", "", nil)
		})
	}
	a.publishSettingsChanged("settings")
	return nil
}

// ReloadSettingThemes is reserved until the Go settings theme catalog becomes push-driven.
func (a *App) ReloadSettingThemes(_ context.Context) error {
	return nil
}

// CloudSyncProgressChanged applies transient sync progress immediately.
func (a *App) CloudSyncProgressChanged(_ context.Context, progress cloudsync.CloudSyncProgress) error {
	return a.runOnUI("apply cloud sync progress", func() {
		a.applyTypedCloudSyncProgress(progress)
	})
}

// RefreshAccountStatus reloads the authoritative account and sync snapshot.
func (a *App) RefreshAccountStatus(_ context.Context) error {
	util.Go(a.lifecycleCtx, "refresh account status", a.reloadCloudSync)
	return nil
}

// UpdateResult patches the visible result with one already-polished core snapshot.
func (a *App) UpdateResult(_ context.Context, result plugin.UpdatableResult) (bool, error) {
	updated := false
	err := a.runOnUI("apply result update", func() {
		updated = a.applyTypedResultUpdate(result)
	})
	return updated, err
}

// PushResults appends a typed result batch only when its query is still visible.
func (a *App) PushResults(_ context.Context, payload plugin.PushResultsPayload) (bool, error) {
	results := make([]queryResult, len(payload.Results))
	for index := range payload.Results {
		results[index] = fromCoreQueryResult(payload.Results[index])
	}
	appended := false
	var appendErr error
	if err := a.runOnUI("append pushed results", func() {
		appended, appendErr = a.appendTypedResults(payload.QueryId, results)
	}); err != nil {
		return false, err
	}
	return appended, appendErr
}

// ToggleRecordingMode reports the missing native window-level implementation.
func (a *App) ToggleRecordingMode(_ context.Context) (bool, error) {
	return false, errors.New("Go UI recording mode is not implemented")
}

// PickFiles runs the native picker and normalizes cancellation to an empty list.
func (a *App) PickFiles(_ context.Context, params common.PickFilesParams) ([]string, error) {
	path, err := a.window.PickFile(woxui.FileDialogOptions{Directory: params.IsDirectory})
	if err != nil {
		return nil, err
	}
	if path == "" {
		return []string{}, nil
	}
	return []string{path}, nil
}

// CaptureScreenshot hides the launcher before starting the native capture session.
func (a *App) CaptureScreenshot(_ context.Context, request common.CaptureScreenshotRequest) (common.CaptureScreenshotResult, error) {
	var restoreParams showAppParams
	wasVisible := false
	if err := a.runOnUI("prepare screenshot launcher state", func() {
		wasVisible = a.visible
		restoreParams = a.show
	}); err != nil {
		return common.CaptureScreenshotResult{Status: common.CaptureScreenshotStatusFailed, ErrorCode: "capture_failed", ErrorMessage: err.Error()}, nil
	}
	if err := a.hideWindow(true); err != nil {
		return common.CaptureScreenshotResult{Status: common.CaptureScreenshotStatusFailed, ErrorCode: "hide_launcher_failed", ErrorMessage: err.Error()}, nil
	}
	restoreLauncher := func() error {
		if !wasVisible {
			return nil
		}
		return a.showWindow(restoreParams)
	}
	result, err := woxui.CaptureScreenshot(woxui.ScreenshotOptions{
		ExportFilePath: request.ExportFilePath, CopyToClipboard: request.Output == "" || strings.EqualFold(request.Output, "clipboard"),
		HideAnnotationToolbar: request.HideAnnotationToolbar, AutoConfirm: request.AutoConfirm, WindowManager: a.windows,
	})
	if err != nil {
		errorCode := "capture_failed"
		if strings.Contains(strings.ToLower(err.Error()), "screen recording permission") {
			errorCode = "permission_denied"
		}
		errorMessage := err.Error()
		if restoreErr := restoreLauncher(); restoreErr != nil {
			errorMessage += "; failed to restore launcher: " + restoreErr.Error()
		}
		return common.CaptureScreenshotResult{Status: common.CaptureScreenshotStatusFailed, ErrorCode: errorCode, ErrorMessage: errorMessage}, nil
	}
	if result.Cancelled {
		if err := restoreLauncher(); err != nil {
			return common.CaptureScreenshotResult{Status: common.CaptureScreenshotStatusFailed, ErrorCode: "restore_launcher_failed", ErrorMessage: err.Error()}, nil
		}
		return common.CaptureScreenshotResult{Status: common.CaptureScreenshotStatusCancelled}, nil
	}
	selection := common.ScreenshotRect{X: float64(result.LogicalSelection.X), Y: float64(result.LogicalSelection.Y), Width: float64(result.LogicalSelection.Width), Height: float64(result.LogicalSelection.Height)}
	return common.CaptureScreenshotResult{
		Status: common.CaptureScreenshotStatusCompleted, ScreenshotPath: result.ScreenshotPath, LogicalSelectionRect: &selection,
		PinToScreen: result.PinToScreen, ClipboardWriteSucceeded: result.ClipboardWriteSucceeded, ClipboardWarningMessage: result.ClipboardWarningMessage,
	}, nil
}

// WriteClipboardImageFile gives native clipboard ownership to the embedded window.
func (a *App) WriteClipboardImageFile(_ context.Context, filePath string) error {
	return a.window.WriteClipboardImageFile(filePath)
}

// ApplyTerminalChunk merges one core ring-buffer update into the matching launcher session.
func (a *App) ApplyTerminalChunk(_ context.Context, sessionID string, chunk terminal.TerminalChunk) error {
	if sessionID != "" && sessionID != a.sessionID {
		return nil
	}
	converted := terminalChunk{SessionID: chunk.SessionID, CursorStart: chunk.CursorStart, CursorEnd: chunk.CursorEnd, Content: chunk.Content, Truncated: chunk.Truncated}
	return a.runOnUI("apply terminal chunk", func() {
		a.applyTerminalChunk(converted)
	})
}

// ApplyTerminalState updates one visible terminal preview state.
func (a *App) ApplyTerminalState(_ context.Context, sessionID string, state terminal.SessionState) error {
	if sessionID != "" && sessionID != a.sessionID {
		return nil
	}
	converted := terminalSessionState{
		SessionID: state.SessionID, Command: state.Command, Interpreter: state.Interpreter,
		WorkingDirectory: state.WorkingDirectory, Status: string(state.Status), ExitCode: state.ExitCode, Error: state.Error,
	}
	return a.runOnUI("apply terminal state", func() {
		a.applyTerminalState(converted)
	})
}

func fromCoreTheme(theme common.Theme) themeData {
	return themeData{
		AppBackgroundColor: theme.AppBackgroundColor, AppPaddingLeft: theme.AppPaddingLeft, AppPaddingTop: theme.AppPaddingTop, AppPaddingRight: theme.AppPaddingRight, AppPaddingBottom: theme.AppPaddingBottom,
		ResultContainerPaddingLeft: theme.ResultContainerPaddingLeft, ResultContainerPaddingTop: theme.ResultContainerPaddingTop, ResultContainerPaddingRight: theme.ResultContainerPaddingRight, ResultContainerPaddingBottom: theme.ResultContainerPaddingBottom,
		ResultItemBorderRadius: theme.ResultItemBorderRadius, ResultItemPaddingLeft: theme.ResultItemPaddingLeft, ResultItemPaddingTop: theme.ResultItemPaddingTop, ResultItemPaddingRight: theme.ResultItemPaddingRight, ResultItemPaddingBottom: theme.ResultItemPaddingBottom,
		ResultItemTitleColor: theme.ResultItemTitleColor, ResultItemSubTitleColor: theme.ResultItemSubTitleColor, ResultItemTailTextColor: theme.ResultItemTailTextColor,
		ResultItemActiveBackgroundColor: theme.ResultItemActiveBackgroundColor, ResultItemActiveTitleColor: theme.ResultItemActiveTitleColor, ResultItemActiveSubTitleColor: theme.ResultItemActiveSubTitleColor, ResultItemActiveTailTextColor: theme.ResultItemActiveTailTextColor,
		QueryBoxFontColor: theme.QueryBoxFontColor, QueryBoxBackgroundColor: theme.QueryBoxBackgroundColor, QueryBoxBorderRadius: theme.QueryBoxBorderRadius, QueryBoxCursorColor: theme.QueryBoxCursorColor,
		QueryBoxTextSelectionBackgroundColor: theme.QueryBoxTextSelectionBackgroundColor, QueryBoxTextSelectionColor: theme.QueryBoxTextSelectionColor,
		ActionContainerBackgroundColor: theme.ActionContainerBackgroundColor, ActionContainerHeaderFontColor: theme.ActionContainerHeaderFontColor,
		ActionContainerPaddingLeft: theme.ActionContainerPaddingLeft, ActionContainerPaddingTop: theme.ActionContainerPaddingTop, ActionContainerPaddingRight: theme.ActionContainerPaddingRight, ActionContainerPaddingBottom: theme.ActionContainerPaddingBottom,
		ActionItemActiveBackgroundColor: theme.ActionItemActiveBackgroundColor, ActionItemActiveFontColor: theme.ActionItemActiveFontColor, ActionItemFontColor: theme.ActionItemFontColor,
		ActionQueryBoxFontColor: theme.ActionQueryBoxFontColor, ActionQueryBoxBackgroundColor: theme.ActionQueryBoxBackgroundColor, ActionQueryBoxBorderRadius: theme.ActionQueryBoxBorderRadius,
		PreviewFontColor: theme.PreviewFontColor, PreviewSplitLineColor: theme.PreviewSplitLineColor, PreviewPropertyTitleColor: theme.PreviewPropertyTitleColor, PreviewPropertyContentColor: theme.PreviewPropertyContentColor,
		ToolbarFontColor: theme.ToolbarFontColor, ToolbarBackgroundColor: theme.ToolbarBackgroundColor, ToolbarPaddingLeft: theme.ToolbarPaddingLeft, ToolbarPaddingRight: theme.ToolbarPaddingRight,
	}
}

func fromCoreToolbarMessage(message plugin.ToolbarMsgUI) toolbarMessage {
	actions := make([]toolbarMessageAction, len(message.Actions))
	for index, action := range message.Actions {
		actions[index] = toolbarMessageAction{
			ID: action.Id, Name: action.Name, Icon: fromCoreImage(action.Icon), Hotkey: action.Hotkey,
			IsDefault: action.IsDefault, PreventHideAfterAction: action.PreventHideAfterAction, ContextData: cloneStringMap(map[string]string(action.ContextData)),
		}
	}
	return toolbarMessage{ID: message.Id, Title: message.Title, Icon: fromCoreImage(message.Icon), Progress: message.Progress, Indeterminate: message.Indeterminate, Actions: actions}
}

func imageFromString(value string) woxImage {
	if value == "" {
		return woxImage{}
	}
	imageType, imageData, ok := strings.Cut(value, ":")
	if !ok {
		return woxImage{}
	}
	return woxImage{ImageType: imageType, ImageData: imageData}
}

func fromCoreChatData(chat common.AIChatData) chatData {
	conversations := make([]chatConversation, len(chat.Conversations))
	for index, conversation := range chat.Conversations {
		images := make([]woxImage, len(conversation.Images))
		for imageIndex := range conversation.Images {
			images[imageIndex] = fromCoreImage(conversation.Images[imageIndex])
		}
		skillRefs := make([]chatSkillRef, len(conversation.SkillRefs))
		for skillIndex, skill := range conversation.SkillRefs {
			skillRefs[skillIndex] = chatSkillRef{ID: skill.Id, Name: skill.Name, Path: skill.Path, Source: skill.Source}
		}
		conversations[index] = chatConversation{
			ID: conversation.Id, Role: string(conversation.Role), Text: conversation.Text, Reasoning: conversation.Reasoning,
			Images: images, SkillRefs: skillRefs,
			ToolCallInfo: chatToolCallInfo{
				ID: conversation.ToolCallInfo.Id, Name: conversation.ToolCallInfo.Name, Arguments: cloneAnyMap(conversation.ToolCallInfo.Arguments),
				Status: string(conversation.ToolCallInfo.Status), Delta: conversation.ToolCallInfo.Delta, Response: conversation.ToolCallInfo.Response,
				StartTimestamp: conversation.ToolCallInfo.StartTimestamp, EndTimestamp: conversation.ToolCallInfo.EndTimestamp,
			},
			Timestamp: conversation.Timestamp,
		}
	}
	compactions := make([]json.RawMessage, len(chat.CompactionEntries))
	for index := range chat.CompactionEntries {
		compactions[index], _ = json.Marshal(chat.CompactionEntries[index])
	}
	var debugTrace json.RawMessage
	if chat.DebugTrace != nil {
		snapshot := chat.DebugTrace.Snapshot()
		debugTrace, _ = json.Marshal(snapshot)
	}
	return chatData{
		ID: chat.Id, Title: chat.Title, Conversations: conversations, CompactionEntries: compactions,
		Model: aiModel{Name: chat.Model.Name, Provider: string(chat.Model.Provider), ProviderAlias: chat.Model.ProviderAlias}, DebugTrace: debugTrace,
		CreatedAt: chat.CreatedAt, UpdatedAt: chat.UpdatedAt, IsStreaming: chat.IsStreaming, IsSummary: chat.IsSummary,
	}
}

func cloneAnyMap(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	copy := make(map[string]any, len(values))
	for key, value := range values {
		copy[key] = value
	}
	return copy
}

func (a *App) applyTypedCloudSyncProgress(progress cloudsync.CloudSyncProgress) {
	if progress.Active {
		copy := cloudSyncProgress{Active: true, Operation: progress.Operation, EntityType: progress.EntityType, PluginID: progress.PluginID, Key: progress.Key, Current: progress.Current, Total: progress.Total}
		a.cloudSettings.SetSyncProgress(&copy)
	} else {
		a.cloudSettings.SetSyncProgress(nil)
	}
	_ = a.window.Invalidate()
	if !progress.Active {
		util.Go(a.lifecycleCtx, "reload cloud sync after progress", a.reloadCloudSyncSilently)
	}
}

func (a *App) applyTypedResultUpdate(result plugin.UpdatableResult) bool {
	updated := false
	for index := range a.results {
		if a.results[index].ID != result.Id {
			continue
		}
		if result.Title != nil {
			a.results[index].Title = *result.Title
		}
		if result.SubTitle != nil {
			a.results[index].SubTitle = *result.SubTitle
		}
		if result.Icon != nil {
			a.results[index].Icon = fromCoreImage(*result.Icon)
		}
		if result.Preview != nil {
			a.results[index].Preview = fromCorePreview(*result.Preview)
		}
		if result.Tails != nil {
			a.results[index].Tails = fromCoreTails(*result.Tails)
		}
		if result.Actions != nil {
			queryResult := plugin.QueryResult{Actions: *result.Actions}
			uiActions := queryResult.ToUI().Actions
			actions := make([]resultAction, len(uiActions))
			for actionIndex := range uiActions {
				actions[actionIndex] = fromCoreResultAction(uiActions[actionIndex])
			}
			a.results[index].Actions = actions
		}
		updated = true
		break
	}
	if updated {
		a.reconcileSelectedPreview()
		_ = a.window.Invalidate()
	}
	return updated
}

func (a *App) appendTypedResults(queryID string, results []queryResult) (bool, error) {
	if queryID != a.query.QueryID {
		return false, nil
	}
	for index := range results {
		if results[index].QueryID == "" {
			results[index].QueryID = queryID
		}
	}
	a.resetQueryTransitionLocked()
	if a.resultsQueryID != queryID {
		a.results = nil
		a.selected = -1
		a.hoveredResult = -1
		a.resultScroll.reset()
		a.resultScrollDetached = false
		a.layout = queryLayout{}
	}
	a.resultsQueryID = queryID
	a.results = append(a.results, results...)
	if a.selected < 0 {
		a.selected = selectableIndex(a.results)
	}
	a.reconcileSelectedPreview()
	if err := a.applyWindowBounds(); err != nil {
		return false, err
	}
	_ = a.window.Invalidate()
	return true, nil
}

func fromCorePreview(preview plugin.WoxPreview) queryPreview {
	tags := make([]previewTag, len(preview.PreviewTags))
	for index, tag := range preview.PreviewTags {
		tags[index] = previewTag{Label: tag.Label, Tooltip: tag.Tooltip}
	}
	return queryPreview{
		PreviewType: preview.PreviewType, PreviewData: preview.PreviewData, PreviewOverlayData: preview.PreviewOverlayData,
		PreviewTags: tags, PreviewProperties: cloneStringMap(preview.PreviewProperties), ScrollPosition: preview.ScrollPosition,
	}
}

func fromCoreTails(tails []plugin.QueryResultTail) []resultTail {
	converted := make([]resultTail, len(tails))
	for index, tail := range tails {
		converted[index] = resultTail{
			Type: tail.Type, Text: tail.Text, TextCategory: tail.TextCategory, Image: fromCoreImage(tail.Image),
			ImageWidth: tail.ImageWidth, ImageHeight: tail.ImageHeight, Tooltip: tail.Tooltip, ContextData: cloneStringMap(tail.ContextData),
		}
	}
	return converted
}

var _ contract.View = (*App)(nil)
