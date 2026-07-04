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

	// OnHandlePluginCommand registers a handler for commands addressed to this plugin.
	// Command names and payload keys are owned by the target plugin and should be treated
	// as documented constants rather than dynamically registered capabilities.
	OnHandlePluginCommand(ctx context.Context, handler PluginCommandHandler)

	// InvokePluginCommand sends a command request to another loaded plugin by plugin id.
	// It is intended for built-in plugin coordination
	InvokePluginCommand(ctx context.Context, request PluginCommandRequest) (PluginCommandResult, error)

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

// OnHandlePluginCommand registers a handler for commands addressed to this plugin.
func (a *APIImpl) OnHandlePluginCommand(ctx context.Context, handler PluginCommandHandler) {
	a.pluginInstance.PluginCommandHandlers = append(a.pluginInstance.PluginCommandHandlers, handler)
}

// InvokePluginCommand sends a command request to another loaded plugin by plugin id.
func (a *APIImpl) InvokePluginCommand(ctx context.Context, request PluginCommandRequest) (PluginCommandResult, error) {
	return GetPluginManager().InvokePluginCommand(ctx, a.pluginInstance, request)
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

	policy := options.LoopPolicy
	if policy.MaxIterations == 0 {
		policy.MaxIterations = 25
	}
	if policy.MaxRetries == 0 {
		policy.MaxRetries = 3
	}

	if callback != nil {
		util.Go(ctx, "ai chat stream", func() {
			a.runChatLoop(ctx, model, conversations, options, policy, provider, callback)
		})
	}

	return nil
}

// runChatLoop is the explicit tool-enabled loop. Each iteration starts a new
// stream, drains it, executes any tool calls, and either finishes or feeds the
// tool results back for the next iteration. Bounded by MaxIterations and the
// loop context.
func (a *APIImpl) runChatLoop(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions, policy common.LoopPolicy, provider ai.Provider, callback common.ChatStreamFunc) {
	var loopCtx context.Context
	var cancel context.CancelFunc
	if policy.Timeout > 0 {
		loopCtx, cancel = context.WithTimeout(ctx, policy.Timeout)
	} else {
		loopCtx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	// localCopy keeps iterations from mutating the shared options while still
	// letting the loop update the conversations slice each round.
	opts := options

	for iteration := 0; policy.MaxIterations < 0 || iteration < policy.MaxIterations; iteration++ {
		if loopCtx.Err() != nil {
			callback(common.ChatStreamData{Status: common.ChatStreamStatusError, Data: "chat loop cancelled"})
			return
		}

		// Summarize old conversations when the context policy asks for it.
		if opts.OnSummarize != nil {
			conversations = opts.OnSummarize(loopCtx, conversations, opts.ContextPolicy)
		}

		stream, err := provider.ChatStream(loopCtx, model, conversations, opts)
		if err != nil {
			callback(common.ChatStreamData{Status: common.ChatStreamStatusError, Data: err.Error(), ToolCalls: []common.ToolCallInfo{}})
			return
		}

		streamedResult, drainErr := a.drainStream(loopCtx, stream, callback)
		if drainErr != nil {
			callback(common.ChatStreamData{Status: common.ChatStreamStatusError, Data: drainErr.Error(), ToolCalls: []common.ToolCallInfo{}})
			return
		}
		if streamedResult == nil {
			// drainStream already reported a terminal status; nothing more to do.
			return
		}

		// No tool calls means the model is done.
		if len(streamedResult.ToolCalls) == 0 {
			finalResult := *streamedResult
			finalResult.Status = common.ChatStreamStatusFinished
			callback(finalResult)
			return
		}

		allSucceeded := a.executeToolCalls(loopCtx, streamedResult, opts, callback)

		if !allSucceeded && !policy.RetryOnFailure {
			streamedResult.Status = common.ChatStreamStatusError
			callback(*streamedResult)
			return
		}

		// load_tools expands only this chat loop's visible tool set; the global
		// registry remains the discovery source and is not mutated.
		opts.Tools = ai.AppendRequestedTools(opts.Tools, streamedResult.ToolCalls)

		// Append tool results as tool-role conversations so the next iteration
		// shows the model what each tool returned.
		conversations = append(conversations, buildToolConversations(streamedResult)...)

		// Sync this iteration's tool results to the UI before looping.
		callback(*streamedResult)
	}

	callback(common.ChatStreamData{
		Status:    common.ChatStreamStatusError,
		Data:      fmt.Sprintf("chat loop exceeded max iterations (%d)", policy.MaxIterations),
		ToolCalls: []common.ToolCallInfo{},
	})
}

// drainStream consumes a provider stream until it either reaches the Streamed
// status (returning the aggregated result) or hits a terminal error. Streaming
// chunks are forwarded to the callback as they arrive.
func (a *APIImpl) drainStream(ctx context.Context, stream ai.ChatStream, callback common.ChatStreamFunc) (*common.ChatStreamData, error) {
	noContentCount := 0
	for {
		streamResult, streamErr := stream.Receive(ctx)
		if streamErr != nil {
			if streamErr == ai.ChatStreamNoContentErr {
				noContentCount++
				if noContentCount > 10 {
					return nil, fmt.Errorf("too many empty stream retries")
				}
				time.Sleep(200 * time.Millisecond)
				continue
			}
			return nil, streamErr
		}

		a.applyStartTimeIfAbsent(&streamResult)

		if streamResult.Status == common.ChatStreamStatusStreaming {
			callback(streamResult)
			continue
		}

		if streamResult.Status == common.ChatStreamStatusStreamed {
			return &streamResult, nil
		}
	}
}

// executeToolCalls runs every tool call in streamedResult concurrently and
// reports per-call status updates to the callback. Returns true when all calls
// succeeded. Honors LoopPolicy.MaxRetries for repeated same-name failures.
func (a *APIImpl) executeToolCalls(ctx context.Context, streamedResult *common.ChatStreamData, options common.ChatOptions, callback common.ChatStreamFunc) bool {
	streamedResult.Status = common.ChatStreamStatusRunningToolCall

	var sw sync.WaitGroup
	retryCounts := make(map[string]int)

	for toolCallIndex, toolCall := range streamedResult.ToolCalls {
		tool, ok := findVisibleTool(options.Tools, toolCall.Name)
		if !ok {
			util.GetLogger().Error(ctx, fmt.Sprintf("AI: tool not available in this chat step: %s", toolCall.Name))
			streamedResult.ToolCalls[toolCallIndex].Status = common.ToolCallStatusFailed
			streamedResult.ToolCalls[toolCallIndex].Response = fmt.Sprintf("tool %q is not loaded; call load_tools first", toolCall.Name)
			continue
		}

		sw.Add(1)
		util.Go(ctx, "ai tool call execution", func() {
			defer sw.Done()
			a.runSingleToolCall(ctx, tool, streamedResult, toolCallIndex, options, callback, retryCounts)
		}, func() {
			defer sw.Done()
			util.GetLogger().Error(ctx, fmt.Sprintf("AI: tool execution panicked, name: %s", tool.Name))
			streamedResult.ToolCalls[toolCallIndex].Status = common.ToolCallStatusFailed
			streamedResult.ToolCalls[toolCallIndex].Response = "tool execution failed with panic"
			callback(*streamedResult)
		})
	}

	sw.Wait()

	return !lo.SomeBy(streamedResult.ToolCalls, func(tc common.ToolCallInfo) bool {
		return tc.Status == common.ToolCallStatusFailed
	})
}

// findVisibleTool enforces the per-step tool boundary advertised to the model.
func findVisibleTool(tools []common.Tool, name string) (common.Tool, bool) {
	for _, tool := range tools {
		if tool.Name == name {
			return tool, true
		}
	}
	return common.Tool{}, false
}

// runSingleToolCall invokes one tool and records its result. Failures are
// recorded as the tool-call response so the model can see them when
// RetryOnFailure is on.
func (a *APIImpl) runSingleToolCall(ctx context.Context, tool common.Tool, streamedResult *common.ChatStreamData, toolCallIndex int, options common.ChatOptions, callback common.ChatStreamFunc, retryCounts map[string]int) {
	toolCall := streamedResult.ToolCalls[toolCallIndex]
	util.GetLogger().Info(ctx, fmt.Sprintf("AI: Executing tool: %s with args: %v, toolcall id: %s", tool.Name, toolCall.Arguments, toolCall.Id))

	streamedResult.ToolCalls[toolCallIndex].Status = common.ToolCallStatusRunning
	callback(*streamedResult)

	toolResponse, toolErr := tool.Callback(ctx, toolCall.Arguments)
	if toolErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("AI: tool execution failed: %s", toolErr.Error()))
		// Allow a few retries for transient failures within the same iteration.
		retryCounts[tool.Name]++
		if retryCounts[tool.Name] < options.LoopPolicy.MaxRetries {
			// Re-attempt once more before giving up; the model may still recover
			// via the error message we record when we give up below.
			toolResponse, toolErr = tool.Callback(ctx, toolCall.Arguments)
		}
	}
	if toolErr != nil {
		streamedResult.ToolCalls[toolCallIndex].Status = common.ToolCallStatusFailed
		streamedResult.ToolCalls[toolCallIndex].Response = toolErr.Error()
	} else {
		streamedResult.ToolCalls[toolCallIndex].Status = common.ToolCallStatusSucceeded
		streamedResult.ToolCalls[toolCallIndex].Response = toolResponse.Text
		streamedResult.ToolCalls[toolCallIndex].EndTimestamp = util.GetSystemTimestamp()
	}
	callback(*streamedResult)
}

// buildToolConversations turns a streamed result's tool calls into tool-role
// conversation entries so the next loop iteration presents them to the model.
func buildToolConversations(streamedResult *common.ChatStreamData) []common.Conversation {
	convs := make([]common.Conversation, 0, len(streamedResult.ToolCalls))
	for i, tc := range streamedResult.ToolCalls {
		reasoning := ""
		if i == 0 {
			reasoning = streamedResult.Reasoning
		}
		convs = append(convs, common.Conversation{
			Id:           tc.Id,
			Role:         common.ConversationRoleTool,
			Text:         tc.Delta,
			Reasoning:    reasoning,
			ToolCallInfo: tc,
			Timestamp:    tc.StartTimestamp,
		})
	}
	return convs
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
