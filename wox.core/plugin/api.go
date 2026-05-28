package plugin

import (
	"context"
	"fmt"
	"path"
	"sync"
	"time"
	"wox/ai"
	"wox/common"
	"wox/setting/definition"
	"wox/util"
	"wox/util/clipboard"

	"github.com/samber/lo"
)

type LogLevel = string

const (
	LogLevelInfo    LogLevel = "Info"
	LogLevelError   LogLevel = "Error"
	LogLevelDebug   LogLevel = "Debug"
	LogLevelWarning LogLevel = "Warning"
)

type CopyType string

const (
	CopyTypePlainText CopyType = "text"
	CopyTypeImage     CopyType = "image"
)

// API exposes the runtime services that plugins can call back into.
type API interface {
	ChangeQuery(ctx context.Context, query common.PlainQuery)
	HideApp(ctx context.Context)
	ShowApp(ctx context.Context)
	Notify(ctx context.Context, description string)
	PushAttention(ctx context.Context, request PushAttentionRequest)
	Log(ctx context.Context, level LogLevel, msg string)
	GetTranslation(ctx context.Context, key string) string
	GetSetting(ctx context.Context, key string) string
	SaveSetting(ctx context.Context, key string, value string, isPlatformSpecific bool)
	OnSettingChanged(ctx context.Context, callback func(ctx context.Context, key string, value string))
	OnGetDynamicSetting(ctx context.Context, callback func(ctx context.Context, key string) definition.PluginSettingDefinitionItem)
	OnDeepLink(ctx context.Context, callback func(ctx context.Context, arguments map[string]string))
	OnUnload(ctx context.Context, callback func(ctx context.Context))
	OnMRURestore(ctx context.Context, callback func(ctx context.Context, mruData MRUData) (*QueryResult, error))

	// ShowToolbarMsg creates or updates the toolbar msg for the current plugin query context.
	// It is only accepted while the caller is the active plugin in the current session.
	// Leaving that plugin query context clears the toolbar msg automatically.
	ShowToolbarMsg(ctx context.Context, msg ToolbarMsg)

	// ClearToolbarMsg removes a toolbar msg previously shown by this plugin by its id.
	ClearToolbarMsg(ctx context.Context, toolbarMsgId string)

	// OnEnterPluginQuery registers a callback that fires once when the session enters
	// this plugin's query context.
	OnEnterPluginQuery(ctx context.Context, callback func(ctx context.Context))

	// OnLeavePluginQuery registers a callback that fires once when the session leaves
	// this plugin's query context.
	OnLeavePluginQuery(ctx context.Context, callback func(ctx context.Context))
	RegisterQueryCommands(ctx context.Context, commands []MetadataCommand)
	AIChatStream(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions, callback common.ChatStreamFunc) error

	// GetUpdatableResult retrieves the current state of a result from the result cache.
	// Returns nil if the result is not found (no longer visible in UI).
	// Returns a pointer to UpdatableResult containing the current state if found.
	//
	// The returned UpdatableResult can be modified and passed to UpdateResult() to update the UI.
	//
	// Example - Toggle favorite state:
	//   Action: func(ctx context.Context, actionContext ActionContext) {
	//       // Get current result state
	//       updatableResult := api.GetUpdatableResult(ctx, actionContext.ResultId)
	//       if updatableResult == nil {
	//           return // Result no longer visible
	//       }
	//
	//       // Toggle favorite
	//       if isFavorite {
	//           removeFavorite()
	//           // Update action name and icon
	//           (*updatableResult.Actions)[actionIndex].Name = "Add to favorite"
	//           (*updatableResult.Actions)[actionIndex].Icon = AddToFavIcon
	//           // Remove favorite tail
	//           *updatableResult.Tails = removeMatchingTail(*updatableResult.Tails, favoriteTail)
	//       } else {
	//           addFavorite()
	//           // Update action name and icon
	//           (*updatableResult.Actions)[actionIndex].Name = "Remove from favorite"
	//           (*updatableResult.Actions)[actionIndex].Icon = RemoveFromFavIcon
	//           // Add favorite tail
	//           *updatableResult.Tails = append(*updatableResult.Tails, favoriteTail)
	//       }
	//
	//       // Update the result
	//       api.UpdateResult(ctx, *updatableResult)
	//   }
	GetUpdatableResult(ctx context.Context, resultId string) *UpdatableResult

	// UpdateResult updates a query result that is currently displayed in the UI.
	//
	// This method is designed for showing real-time progress updates during long-running operations,
	// such as file downloads, plugin installations, or API calls. It directly pushes updates to the UI
	// without polling, making it ideal for one-time or event-driven updates.
	//
	// Returns:
	//   - true: The result was successfully updated (still visible in the UI)
	//   - false: The result is no longer visible in the UI (caller should stop updating)
	//
	// When to use UpdateResult:
	//   - Progress updates during Action execution (e.g., "Downloading... 50%")
	//   - One-time status updates (e.g., "Installation complete")
	//   - Event-driven updates with clear start/end (e.g., file change notifications)
	//   - Periodic updates (e.g., CPU/memory monitoring) - start a timer in Init() and track result IDs
	//
	// Best practices:
	//   - Set PreventHideAfterAction: true in your action to keep the result visible
	//   - Only call this within Action handlers or background goroutines spawned by actions
	//   - Check the return value - if false, stop updating to avoid resource leaks
	//   - Only update fields that have changed (use nil for fields you don't want to update)
	//
	// Example:
	//   Action: func(ctx context.Context, actionContext ActionContext) {
	//       title := "Installing..."
	//       api.UpdateResult(ctx, UpdatableResult{Id: actionContext.ResultId, Title: &title})
	//
	//       go func() {
	//           title := "Downloading..."
	//           if !api.UpdateResult(ctx, UpdatableResult{Id: actionContext.ResultId, Title: &title}) {
	//               return // Result no longer visible, stop updating
	//           }
	//           // ... perform download ...
	//           title = "Installation complete"
	//           api.UpdateResult(ctx, UpdatableResult{Id: actionContext.ResultId, Title: &title})
	//       }()
	//   }
	UpdateResult(ctx context.Context, result UpdatableResult) bool

	// PushResults pushes additional query results to UI for the given query.
	// Returns true if results were accepted by UI (query still active), false otherwise.
	PushResults(ctx context.Context, query Query, results []QueryResult) bool

	// IsVisible returns true if the Wox window is currently visible.
	// This is useful for plugins that perform periodic updates (e.g., CPU/memory monitoring)
	// to avoid wasting resources when the window is hidden.
	//
	// Example:
	//   func (p *Plugin) refreshData(ctx context.Context) {
	//       if !p.api.IsVisible(ctx) {
	//           return // Window is hidden, skip update
	//       }
	//       // ... update data ...
	//   }
	IsVisible(ctx context.Context) bool

	// RefreshQuery re-executes the current query with the existing query text.
	// This is useful when plugin data changes and you want to update the displayed results.
	//
	// Parameters:
	//   - ctx: Context
	//   - param: RefreshQueryParam to control refresh behavior
	//
	// Example - Refresh after marking item as favorite:
	//   Action: func(ctx context.Context, actionContext ActionContext) {
	//       markAsFavorite(item)
	//       // Refresh query and preserve user's current selection
	//       api.RefreshQuery(ctx, RefreshQueryParam{PreserveSelectedIndex: true})
	//   }
	//
	// Example - Refresh after deleting item:
	//   Action: func(ctx context.Context, actionContext ActionContext) {
	//       deleteItem(item)
	//       // Refresh query and reset to first item
	//       api.RefreshQuery(ctx, RefreshQueryParam{PreserveSelectedIndex: false})
	//   }
	RefreshQuery(ctx context.Context, param RefreshQueryParam)

	// RefreshGlance asks Wox UI to pull the latest Global Glance data for this plugin.
	// It deliberately does not push UI content so user slot settings remain authoritative.
	RefreshGlance(ctx context.Context, ids []string)

	// Copy copies the given content to the system clipboard.
	// Supports text, image, or both simultaneously.
	Copy(ctx context.Context, params CopyParams)

	// Screenshot captures a user-selected screen area and returns the saved PNG path.
	Screenshot(ctx context.Context, option ScreenshotOption) ScreenshotResult
}

type CopyParams struct {
	Type     CopyType
	Text     string
	WoxImage *common.WoxImage
}

// ScreenshotOption controls optional screenshot behavior.
type ScreenshotOption struct {
	// HideAnnotationToolbar keeps plugin capture flows focused on raw image selection when callers,
	// such as OCR plugins, do not need Wox's markup tools. Cancel and confirm remain visible so the
	// user still has an explicit escape/finish path when AutoConfirm is not requested.
	HideAnnotationToolbar bool `json:"hideAnnotationToolbar"`
	// AutoConfirm completes the screenshot as soon as the user finishes drawing the selection.
	// The previous API always required a manual confirm click, which is unnecessary for callers that
	// only need the selected PNG path and do their own processing after capture.
	AutoConfirm bool `json:"autoConfirm"`
}

// ScreenshotResult reports the screenshot capture outcome and saved PNG path.
type ScreenshotResult struct {
	Success        bool
	ScreenshotPath string
	ErrMsg         string
}

// APIImpl is the concrete API implementation bound to one plugin instance.
type APIImpl struct {
	pluginInstance       *Instance
	logger               *util.Log
	toolCallStartTimeMap *util.HashMap[string, int64] // store the start time of tool calls
}

func (a *APIImpl) ChangeQuery(ctx context.Context, query common.PlainQuery) {
	GetPluginManager().GetUI().ChangeQuery(ctx, query)
}

func (a *APIImpl) HideApp(ctx context.Context) {
	GetPluginManager().GetUI().HideApp(ctx)
}

func (a *APIImpl) ShowApp(ctx context.Context) {
	GetPluginManager().GetUI().ShowApp(ctx, common.ShowContext{
		SelectAll: true,
	})
}

func (a *APIImpl) Notify(ctx context.Context, message string) {
	icon := a.pluginInstance.Metadata.Icon
	if parsedIcon, err := common.ParseWoxImage(icon); err == nil {
		convertedIcon := common.ConvertIcon(ctx, parsedIcon, a.pluginInstance.PluginDirectory)
		icon = convertedIcon.String()
	}

	GetPluginManager().GetUI().Notify(ctx, common.NotifyMsg{
		PluginId:       a.pluginInstance.Metadata.Id,
		Text:           a.GetTranslation(ctx, message),
		Icon:           icon,
		DisplaySeconds: 5,
	})
}

// PushAttention persists a plugin-owned item and refreshes the launcher unread badge.
func (a *APIImpl) PushAttention(ctx context.Context, request PushAttentionRequest) {
	if a.pluginInstance == nil {
		return
	}

	request.Title = a.GetTranslation(ctx, request.Title)
	if request.Description != "" {
		request.Description = a.GetTranslation(ctx, request.Description)
	}

	defaultIcon := a.pluginInstance.Metadata.GetIconOrDefault(a.pluginInstance.PluginDirectory, common.WoxIcon)
	_, err := GetAttentionManager().Push(ctx, AttentionPluginSource{
		PluginID:        a.pluginInstance.Metadata.Id,
		PluginDirectory: a.pluginInstance.PluginDirectory,
		DefaultIcon:     defaultIcon,
	}, request)
	if err != nil {
		a.Log(ctx, LogLevelWarning, fmt.Sprintf("failed to push attention item: %v", err))
		return
	}

	PublishAttentionUnreadCount(ctx)
}

// PublishAttentionUnreadCount pushes the current unread attention count to the UI.
func PublishAttentionUnreadCount(ctx context.Context) {
	count, err := GetAttentionManager().UnreadCount(ctx)
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to count unread attention items: %v", err))
		return
	}

	GetPluginManager().GetUI().UpdateAttentionUnreadCount(ctx, int(count))
}

func (a *APIImpl) Log(ctx context.Context, level LogLevel, msg string) {
	logCtx := util.WithComponentContext(ctx, a.pluginInstance.GetName(ctx))
	if level == LogLevelError {
		a.logger.Error(logCtx, msg)
		logger.Error(logCtx, msg)
		return
	}

	if level == LogLevelInfo {
		a.logger.Info(logCtx, msg)
		logger.Info(logCtx, msg)
		return
	}

	if level == LogLevelDebug {
		a.logger.Debug(logCtx, msg)
		logger.Debug(logCtx, msg)
		return
	}

	if level == LogLevelWarning {
		a.logger.Warn(logCtx, msg)
		logger.Warn(logCtx, msg)
		return
	}
}

func (a *APIImpl) GetTranslation(ctx context.Context, key string) string {
	return a.pluginInstance.Metadata.translate(ctx, common.I18nString(key))
}

func (a *APIImpl) GetSetting(ctx context.Context, key string) string {
	// try to get platform specific setting first
	platformSpecificKey := key + "@" + util.GetCurrentPlatform()
	v, exist := a.pluginInstance.Setting.Get(platformSpecificKey)
	if exist {
		return v
	}

	v, exist = a.pluginInstance.Setting.Get(key)
	if exist {
		return v
	}
	return ""
}

func (a *APIImpl) SaveSetting(ctx context.Context, key string, value string, isPlatformSpecific bool) {
	finalKey := key
	if isPlatformSpecific {
		finalKey = key + "@" + util.GetCurrentPlatform()
	} else {
		// if not platform specific, remove platform specific setting, otherwise it will be loaded first
		a.pluginInstance.Setting.Delete(key + "@" + util.GetCurrentPlatform())
	}

	existValue, exist := a.pluginInstance.Setting.Get(finalKey)
	a.pluginInstance.Setting.Set(finalKey, value)
	if !exist || (existValue != value) {
		for _, callback := range a.pluginInstance.SettingChangeCallbacks {
			util.Go(ctx, "plugin setting change callback", func() {
				callback(ctx, key, value)
			})
		}
	}
}

func (a *APIImpl) OnSettingChanged(ctx context.Context, callback func(ctx context.Context, key string, value string)) {
	a.pluginInstance.SettingChangeCallbacks = append(a.pluginInstance.SettingChangeCallbacks, callback)
}

func (a *APIImpl) OnGetDynamicSetting(
	ctx context.Context,
	callback func(ctx context.Context, key string) definition.PluginSettingDefinitionItem,
) {
	a.pluginInstance.DynamicSettingCallbacks = append(a.pluginInstance.DynamicSettingCallbacks, callback)
}

func (a *APIImpl) OnDeepLink(ctx context.Context, callback func(ctx context.Context, arguments map[string]string)) {
	if !a.pluginInstance.Metadata.IsSupportFeature(MetadataFeatureDeepLink) {
		a.Log(ctx, LogLevelError, "plugin has no access to deep link feature")
		return
	}

	a.pluginInstance.DeepLinkCallbacks = append(a.pluginInstance.DeepLinkCallbacks, callback)
}

func (a *APIImpl) OnUnload(ctx context.Context, callback func(ctx context.Context)) {
	a.pluginInstance.UnloadCallbacks = append(a.pluginInstance.UnloadCallbacks, callback)
}

func (a *APIImpl) ShowToolbarMsg(ctx context.Context, msg ToolbarMsg) {
	GetPluginManager().ShowToolbarMsg(ctx, a.pluginInstance, msg)
}

func (a *APIImpl) ClearToolbarMsg(ctx context.Context, toolbarMsgId string) {
	GetPluginManager().ClearToolbarMsg(ctx, a.pluginInstance, toolbarMsgId)
}

func (a *APIImpl) OnEnterPluginQuery(ctx context.Context, callback func(ctx context.Context)) {
	a.pluginInstance.EnterPluginQueryCallbacks = append(a.pluginInstance.EnterPluginQueryCallbacks, callback)
}

func (a *APIImpl) OnLeavePluginQuery(ctx context.Context, callback func(ctx context.Context)) {
	a.pluginInstance.LeavePluginQueryCallbacks = append(a.pluginInstance.LeavePluginQueryCallbacks, callback)
}

func (a *APIImpl) RegisterQueryCommands(ctx context.Context, commands []MetadataCommand) {
	a.pluginInstance.RuntimeQueryCommands = append([]MetadataCommand(nil), commands...)
}

func (a *APIImpl) AIChatStream(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions, callback common.ChatStreamFunc) error {
	//check if plugin has the feature permission
	if !a.pluginInstance.Metadata.IsSupportFeature(MetadataFeatureAI) {
		return fmt.Errorf("plugin has no access to ai feature")
	}

	provider, providerErr := GetPluginManager().GetAIProvider(ctx, model.Provider, model.ProviderAlias)
	if providerErr != nil {
		return providerErr
	}

	// // resize images in the conversation
	// for i, conversation := range conversations {
	// 	for j, image := range conversation.Images {
	// 		image.Resize(600)
	// 		resizeImage(ctx, image, 600)

	// 		// resize image if it's too large
	// 		maxWidth := 600
	// 		if image.Bounds().Dx() > maxWidth {
	// 			start := util.GetSystemTimestamp()
	// 			conversations[i].Images[j] = imaging.Resize(image, maxWidth, 0, imaging.Lanczos)
	// 			a.Log(ctx, LogLevelDebug, fmt.Sprintf("resizing image (%d -> %d) in ai chat, cost %d ms", image.Bounds().Dx(), maxWidth, util.GetSystemTimestamp()-start))
	// 		}
	// 	}
	// }

	stream, err := provider.ChatStream(ctx, model, conversations, options)
	if err != nil {
		return err
	}

	if callback != nil {
		util.Go(ctx, "ai chat stream", func() {
			for {
				streamResult, streamErr := stream.Receive(ctx)
				if streamErr != nil {
					// may be for loop too fast
					if streamErr == ai.ChatStreamNoContentErr {
						time.Sleep(time.Millisecond * 200)
						continue
					}

					util.GetLogger().Info(ctx, fmt.Sprintf("AI: failed to read stream from ai provider: %s", streamErr.Error()))
					callback(common.ChatStreamData{
						Status:    common.ChatStreamStatusError,
						Data:      streamErr.Error(),
						ToolCalls: []common.ToolCallInfo{},
					})
					return
				}

				util.GetLogger().Debug(ctx, fmt.Sprintf("AI: Received stream from ai provider: status=%s, data=%s, tool calls=%d", streamResult.Status, streamResult.Data, len(streamResult.ToolCalls)))

				a.applyStartTimeIfAbsent(&streamResult)

				if streamResult.Status == common.ChatStreamStatusStreaming {
					callback(streamResult)
					continue
				}

				if streamResult.Status == common.ChatStreamStatusStreamed {
					// execute tool calls
					// we execute tool calls asynchronously, but wait for all tool calls to finish before sending the final result
					var sw = sync.WaitGroup{}

					for toolCallIndex, toolCall := range streamResult.ToolCalls {
						util.GetLogger().Info(ctx, fmt.Sprintf("AI: Tool call is pending to execute, name: %s, args: %v", toolCall.Name, toolCall.Arguments))

						for _, tool := range options.Tools {
							if tool.Name == toolCall.Name {
								sw.Add(1)

								util.GetLogger().Info(ctx, fmt.Sprintf("AI: Executing tool: %s with args: %v, toolcall id: %s, toolcall status: %s", tool.Name, toolCall.Arguments, toolCall.Id, toolCall.Status))

								// update tool call status to running and sync to caller
								streamResult.Status = common.ChatStreamStatusRunningToolCall
								streamResult.ToolCalls[toolCallIndex].Status = common.ToolCallStatusRunning

								util.Go(ctx, "ai tool call execution", func() {
									toolResponse, toolErr := tool.Callback(ctx, toolCall.Arguments)
									if toolErr != nil {
										util.GetLogger().Error(ctx, fmt.Sprintf("AI: tool execution failed: %s", toolErr.Error()))
										streamResult.ToolCalls[toolCallIndex].Status = common.ToolCallStatusFailed
										streamResult.ToolCalls[toolCallIndex].Response = toolErr.Error()
									} else {
										streamResult.ToolCalls[toolCallIndex].Status = common.ToolCallStatusSucceeded
										streamResult.ToolCalls[toolCallIndex].Response = toolResponse.Text
										streamResult.ToolCalls[toolCallIndex].EndTimestamp = util.GetSystemTimestamp()
									}

									callback(streamResult)
									sw.Done()
								}, func() {
									util.GetLogger().Error(ctx, fmt.Sprintf("AI: tool execution failed with panic, name: %s", tool.Name))
									streamResult.ToolCalls[toolCallIndex].Status = common.ToolCallStatusFailed
									streamResult.ToolCalls[toolCallIndex].Response = "tool execution failed with panic"

									callback(streamResult)
									sw.Done()
								})
							}
						}
					}

					sw.Wait()

					anyToolCallFailed := lo.SomeBy(streamResult.ToolCalls, func(toolCall common.ToolCallInfo) bool {
						return toolCall.Status == common.ToolCallStatusFailed
					})
					if anyToolCallFailed {
						streamResult.Status = common.ChatStreamStatusError
						callback(streamResult)
					} else {
						streamResult.Status = common.ChatStreamStatusFinished
						callback(streamResult)
					}
					return
				}
			}
		})
	}

	return nil
}

func (a *APIImpl) applyStartTimeIfAbsent(streamResult *common.ChatStreamData) {
	for toolCallIndex, toolCall := range streamResult.ToolCalls {
		startTime := util.GetSystemTimestamp()
		if v, ok := a.toolCallStartTimeMap.Load(toolCall.Id); ok {
			startTime = v
		} else {
			a.toolCallStartTimeMap.Store(toolCall.Id, startTime)
		}
		streamResult.ToolCalls[toolCallIndex].StartTimestamp = startTime
	}
}

func (a *APIImpl) OnMRURestore(ctx context.Context, callback func(ctx context.Context, mruData MRUData) (*QueryResult, error)) {
	if !a.pluginInstance.Metadata.IsSupportFeature(MetadataFeatureMRU) {
		a.Log(ctx, LogLevelError, "plugin has no access to MRU feature")
		return
	}

	a.pluginInstance.MRURestoreCallbacks = append(a.pluginInstance.MRURestoreCallbacks, callback)
}

func (a *APIImpl) UpdateResult(ctx context.Context, result UpdatableResult) bool {
	if sessionId, queryId := GetPluginManager().GetQueryInfoByResultId(result.Id); sessionId != "" {
		ctx = util.WithQueryIdContext(util.WithSessionContext(ctx, sessionId), queryId)
	}
	polishedResult := GetPluginManager().PolishUpdatableResult(ctx, a.pluginInstance, result)
	success := GetPluginManager().GetUI().UpdateResult(ctx, polishedResult)
	return success
}

func (a *APIImpl) PushResults(ctx context.Context, query Query, results []QueryResult) bool {
	if query.Id == "" {
		a.Log(ctx, LogLevelWarning, "PushResults ignored: query id is empty")
		return false
	}
	if query.SessionId == "" {
		a.Log(ctx, LogLevelWarning, "PushResults ignored: session id is empty")
		return false
	}
	if util.GetContextSessionId(ctx) == "" {
		ctx = util.WithQueryIdContext(util.WithSessionContext(ctx, query.SessionId), query.Id)
	}

	// Bug fix: core no longer owns "current query" state because backend query
	// pipelines are concurrent. Push by query id and let Flutter accept or reject
	// the payload against the visible query, matching normal Query responses.
	layout := GetPluginManager().getCachedLayoutForPluginQuery(ctx, a.pluginInstance, query)
	for i := range results {
		results[i] = GetPluginManager().PolishResult(ctx, a.pluginInstance, query, layout, results[i])
	}

	polishedResults := GetPluginManager().BuildQueryResultsSnapshot(query.SessionId, query.Id)
	payload := PushResultsPayload{
		QueryId: query.Id,
		Results: polishedResults,
	}
	return GetPluginManager().GetUI().PushResults(ctx, payload)
}

func (a *APIImpl) GetUpdatableResult(ctx context.Context, resultId string) *UpdatableResult {
	return GetPluginManager().GetUpdatableResult(ctx, resultId)
}

func (a *APIImpl) IsVisible(ctx context.Context) bool {
	return GetPluginManager().GetUI().IsVisible(ctx)
}

func (a *APIImpl) RefreshQuery(ctx context.Context, param RefreshQueryParam) {
	GetPluginManager().GetUI().RefreshQuery(ctx, param.PreserveSelectedIndex)
}

func (a *APIImpl) RefreshGlance(ctx context.Context, ids []string) {
	if a.pluginInstance == nil {
		return
	}
	GetPluginManager().GetUI().RefreshGlance(ctx, a.pluginInstance.Metadata.Id, ids)
}

func (a *APIImpl) Copy(ctx context.Context, params CopyParams) {
	if params.Type == CopyTypePlainText {
		err := clipboard.WriteText(params.Text)
		if err != nil {
			a.Log(ctx, LogLevelError, fmt.Sprintf("failed to copy text to clipboard: %v", err))
		}
		return
	}

	if params.Type == CopyTypeImage {
		img, err := params.WoxImage.ToImage()
		if err != nil {
			a.Log(ctx, LogLevelError, fmt.Sprintf("failed to convert woximage to image: %v", err))
			return
		}
		err = clipboard.Write(&clipboard.ImageData{
			Image: img,
		})
		if err != nil {
			a.Log(ctx, LogLevelError, fmt.Sprintf("failed to copy image to clipboard: %v", err))
		}
		return
	}
}

func (a *APIImpl) Screenshot(ctx context.Context, option ScreenshotOption) ScreenshotResult {
	request := common.DefaultCaptureScreenshotRequest()
	// Plugin screenshots return a saved file path and leave clipboard handling to the caller.
	request.Output = "file"
	// Screenshot API options are translated in core where the plugin caller is known. Keeping Flutter
	// on a request-only contract avoids making the UI infer SDK defaults from plugin runtime details.
	request.HideAnnotationToolbar = option.HideAnnotationToolbar
	request.AutoConfirm = option.AutoConfirm
	if !a.pluginInstance.IsSystemPlugin {
		// Third-party screenshot callers need a visible identity marker in the floating toolbox.
		// The UI cannot reliably infer the plugin from the generic CaptureScreenshot websocket method,
		// so core resolves the metadata icon here and sends only the render-ready WoxImage.
		callerIcon := a.pluginInstance.Metadata.GetIconOrDefault(a.pluginInstance.PluginDirectory, common.WoxIcon)
		request.CallerIcon = &callerIcon
	}

	result, err := GetPluginManager().GetUI().CaptureScreenshot(ctx, request)
	if err != nil {
		return ScreenshotResult{
			Success: false,
			ErrMsg:  err.Error(),
		}
	}

	switch result.Status {
	case common.CaptureScreenshotStatusCompleted:
		if result.ScreenshotPath == "" {
			return ScreenshotResult{
				Success: false,
				ErrMsg:  "screenshot completed without an export path",
			}
		}

		return ScreenshotResult{
			Success:        true,
			ScreenshotPath: result.ScreenshotPath,
			ErrMsg:         result.ClipboardWarningMessage,
		}
	case common.CaptureScreenshotStatusCancelled:
		return ScreenshotResult{
			Success: false,
			ErrMsg:  "cancelled",
		}
	case common.CaptureScreenshotStatusFailed:
		errMsg := result.ErrorMessage
		if errMsg == "" {
			errMsg = "screenshot session failed"
		}
		return ScreenshotResult{
			Success: false,
			ErrMsg:  errMsg,
		}
	default:
		return ScreenshotResult{
			Success: false,
			ErrMsg:  fmt.Sprintf("unexpected screenshot status: %s", result.Status),
		}
	}
}

func NewAPI(instance *Instance) API {
	apiImpl := &APIImpl{pluginInstance: instance}
	logFolder := path.Join(util.GetLocation().GetLogPluginDirectory(), instance.Metadata.Id)
	apiImpl.logger = util.CreateLogger(logFolder)
	apiImpl.toolCallStartTimeMap = util.NewHashMap[string, int64]()
	return apiImpl
}
