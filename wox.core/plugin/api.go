package plugin

import (
	"context"
	"fmt"
	"image"
	"sync"
	"sync/atomic"
	"time"
	"wox/ai"
	"wox/common"
	"wox/common/icons"
	"wox/setting/definition"
	"wox/util"
	"wox/util/clipboard"
	"wox/util/notifier"

	"github.com/samber/lo"
)

type LogLevel = string

type RegisterTriggerKeywordOption struct {
	Keyword   string
	QueryHint *common.QueryHint
	// DebounceMs overrides plugin-wide debounce for input queries using this keyword.
	// Nil inherits the metadata trigger policy, then the plugin default; zero executes immediately.
	DebounceMs *int
	// QueryVariables is core-only context requested by built-in searches before launcher activation.
	QueryVariables []QueryVariable `json:"-"`
}

type RegisterTriggerKeywordResult struct {
	Success bool
}

type UnregisterTriggerKeywordOption struct {
	Keyword string
}

type UnregisterTriggerKeywordResult struct {
	Success bool
}

type DragOutStatus string

const (
	DragOutStatusSuccess        DragOutStatus = "success"
	DragOutStatusCancel         DragOutStatus = "cancel"
	DragOutStatusCancelInSource DragOutStatus = "cancel_in_source"
)

// DragOutEvent is a post-drag notification. It does not enable, disable, or cancel the drag.
type DragOutEvent struct {
	ResultId string        `json:"ResultId"`
	Files    []string      `json:"Files"`
	Status   DragOutStatus `json:"Status"`
}

type DragOutListenOption struct {
	Callback func(ctx context.Context, event DragOutEvent)
}

type DragOutListenResult struct {
	Success bool
}

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

// API exposes the runtime services available to a plugin instance.
// Published plugin and SDK entry points must remain backward compatible.
type API interface {
	// Launcher and query lifecycle.

	// ChangeQuery changes the launcher query. Only input and selection query types are accepted.
	ChangeQuery(ctx context.Context, query common.PlainQuery)

	// ShowApp shows the launcher and selects its query text.
	ShowApp(ctx context.Context)

	// HideApp hides the launcher.
	HideApp(ctx context.Context)

	// IsVisible reports visibility for the context's session when known; otherwise it reports the
	// primary launcher, without counting secondary panels.
	IsVisible(ctx context.Context) bool

	// OnEnterPluginQuery registers a callback that fires once when a session enters this plugin's query context.
	OnEnterPluginQuery(ctx context.Context, callback func(ctx context.Context))

	// OnLeavePluginQuery registers a callback that fires once when a session leaves this plugin's query context.
	OnLeavePluginQuery(ctx context.Context, callback func(ctx context.Context))

	// RegisterQueryCommands replaces runtime query commands. Invalid debounce values leave the
	// previous commands intact; metadata commands take precedence over duplicates.
	RegisterQueryCommands(ctx context.Context, commands []MetadataCommand)

	// RegisterTriggerKeyword registers a runtime trigger. It fails for invalid keywords or keywords
	// owned by another enabled plugin; re-registering this plugin's keyword succeeds.
	RegisterTriggerKeyword(ctx context.Context, option RegisterTriggerKeywordOption) RegisterTriggerKeywordResult

	// UnregisterTriggerKeyword removes this plugin's runtime trigger registration. An absent
	// registration also succeeds.
	UnregisterTriggerKeyword(ctx context.Context, option UnregisterTriggerKeywordOption) UnregisterTriggerKeywordResult

	// RefreshQuery re-executes the current query. Selection behavior is controlled by param;
	// SelectedResultId requires the rebuilt result to retain its ID.
	RefreshQuery(ctx context.Context, param RefreshQueryParam)

	// PushResults adds results for the supplied query. The query must include its ID and session ID;
	// false means the UI did not accept the update.
	PushResults(ctx context.Context, query Query, results []QueryResult) bool

	// GetUpdatableResult returns a snapshot of a cached result, or nil if it is not found. Modify the
	// snapshot and pass it to UpdateResult to apply changes.
	GetUpdatableResult(ctx context.Context, resultId string) *UpdatableResult

	// UpdateResult applies a patch to a query result in the UI. Nil fields are left unchanged. Returns
	// false if the update is rejected, including when the result is no longer available. Stop periodic
	// updates when it returns false; use PreventHideAfterAction if an action needs the result to stay visible.
	UpdateResult(ctx context.Context, result UpdatableResult) bool

	// OnMRURestore registers a callback that reconstructs a result from its MRU data. The plugin must
	// declare the MRU feature.
	OnMRURestore(ctx context.Context, callback func(ctx context.Context, mruData MRUData) (*QueryResult, error))

	// OnDragOut registers a post-drag notification for draggable results. It cannot cancel a drag;
	// native drags are copy-only, so remove source files after a successful drag if needed.
	OnDragOut(ctx context.Context, option DragOutListenOption) DragOutListenResult

	// Presentation and notifications.

	// CreateWindow creates a hidden plugin window and installs its event callback before content
	// loads.
	CreateWindow(ctx context.Context, option CreateWindowOption) CreateWindowResult

	// ShowToastMsg displays a transient toast message.
	// Requires Wox >= 2.5.0.
	ShowToastMsg(ctx context.Context, option ShowToastMsgOption) ShowToastMsgResult

	// PushAttention persists a plugin-owned attention item and refreshes the unread badge.
	PushAttention(ctx context.Context, request PushAttentionRequest)

	// ShowToolbarMsg creates or updates a toolbar message while this plugin is active in the current
	// query session. It is cleared when the session leaves the plugin's query context.
	ShowToolbarMsg(ctx context.Context, msg ToolbarMsg)

	// ClearToolbarMsg removes a toolbar message previously shown by this plugin by ID.
	ClearToolbarMsg(ctx context.Context, toolbarMsgId string)

	// RefreshGlance asks the UI to fetch the latest Global Glance data for this plugin and the given
	// IDs.
	RefreshGlance(ctx context.Context, ids []string)

	// GetThemeColors returns the current launcher palette as opaque #RRGGBB colors for plugin HTML and
	// webviews. Requires Wox >= 2.4.5.
	GetThemeColors(ctx context.Context, option GetThemeColorsOption) GetThemeColorsResult

	// Settings and lifecycle.

	// GetSetting returns a plugin setting, preferring the current platform's value. Returns an empty
	// string if absent.
	GetSetting(ctx context.Context, key string) string

	// SaveSetting persists a synchronized plugin setting.
	//
	// Deprecated: use SetSetting.
	SaveSetting(ctx context.Context, key string, value string, isPlatformSpecific bool)

	// SetSetting persists a plugin setting with explicit platform and device-local behavior. Available
	// since Wox v2.4.0.
	SetSetting(ctx context.Context, option SetSettingOption) SetSettingResult

	// OnSettingChanged registers a callback for plugin setting changes.
	OnSettingChanged(ctx context.Context, callback func(ctx context.Context, key string, value string))

	// OnGetDynamicSetting registers a callback that supplies a dynamic setting definition for a key.
	OnGetDynamicSetting(ctx context.Context, callback func(ctx context.Context, key string) definition.PluginSettingDefinitionItem)

	// OnHotkey subscribes to plugin hotkey events once per initialization cycle. Bindings become
	// active after successful initialization.
	OnHotkey(ctx context.Context, option OnHotkeyOption) OnHotkeyResult

	// OnDeepLink registers a callback for deep link arguments. The plugin must declare the deep link
	// feature.
	OnDeepLink(ctx context.Context, callback func(ctx context.Context, arguments map[string]string))

	// OnUnload registers a callback invoked when the plugin is unloaded.
	OnUnload(ctx context.Context, callback func(ctx context.Context))

	// Plugin tools and AI.

	// RegisterPluginTool publishes a callable tool owned by this plugin.
	// Requires Wox >= 2.4.5.
	RegisterPluginTool(ctx context.Context, option RegisterPluginToolOption) RegisterPluginToolResult

	// UnregisterPluginTool removes a tool registered by this plugin. Repeating the operation succeeds.
	// Requires Wox >= 2.4.5.
	UnregisterPluginTool(ctx context.Context, option UnregisterPluginToolOption) UnregisterPluginToolResult

	// ListPluginTools returns callable tools from initialized, enabled plugins.
	// Requires Wox >= 2.4.5.
	ListPluginTools(ctx context.Context, option ListPluginToolsOption) ListPluginToolsResult

	// InvokePluginTool invokes another plugin's registered tool after schema validation. Requires Wox
	// Requires Wox >= 2.4.5.
	InvokePluginTool(ctx context.Context, option InvokePluginToolOption) InvokePluginToolResult

	// AIChatStream starts an asynchronous AI chat stream and delivers updates to a non-nil callback.
	// The plugin must declare the AI feature; setup errors are returned immediately. A nil callback
	// does not start a stream.
	AIChatStream(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions, callback common.ChatStreamFunc) error

	// Utilities.

	// Log writes a message to the Wox log under this plugin name.
	Log(ctx context.Context, level LogLevel, msg string)

	// GetTranslation resolves a translation using the plugin and Wox translations for the current
	// language.
	GetTranslation(ctx context.Context, key string) string

	// GetCacheFolder returns this plugin's machine-local cache directory, creating it if needed, or an
	// empty string on failure. Wox deletes it on uninstall; store settings through GetSetting and
	// SetSetting instead.
	GetCacheFolder(ctx context.Context) string

	// Copy copies text or an image to the system clipboard according to params.Type.
	Copy(ctx context.Context, params CopyParams)

	// Screenshot captures a user-selected screen area and returns the saved image path.
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
	// only need the selected image path and do their own processing after capture.
	AutoConfirm bool `json:"autoConfirm"`
}

// ScreenshotResult reports the screenshot capture outcome and saved image path.
type ScreenshotResult struct {
	Success        bool
	ScreenshotPath string
	ErrMsg         string
}

// GetThemeColorsOption is reserved so later filters can be added without a new API.
// Requires Wox >= 2.4.5.
type GetThemeColorsOption struct{}

// GetThemeColorsResult is the opaque launcher palette for plugin-authored HTML.
// Requires Wox >= 2.4.5.
type GetThemeColorsResult struct {
	Background    string `json:"Background"`
	Text          string `json:"Text"`
	SecondaryText string `json:"SecondaryText"`
	Border        string `json:"Border"`
	Accent        string `json:"Accent"`
	AccentText    string `json:"AccentText"`
	Selection     string `json:"Selection"`
	Dark          bool   `json:"Dark"`
}

// SetSettingOption controls how a plugin setting is persisted.
// Available since Wox v2.4.0.
type SetSettingOption struct {
	Key              string `json:"key"`
	Value            string `json:"value"`
	PlatformSpecific bool   `json:"platformSpecific,omitempty"`
	IsLocal          bool   `json:"isLocal,omitempty"`
}

// SetSettingResult reports whether a plugin setting was persisted.
// Available since Wox v2.4.0.
type SetSettingResult struct {
	Success bool
	ErrMsg  string
}

// APIImpl is the concrete API implementation bound to one plugin instance.
type APIImpl struct {
	hotkeyMu             sync.Mutex
	hotkeys              *hotkeySubscription
	hotkeysStopped       bool
	windowEpoch          atomic.Uint64
	windowSuspended      atomic.Bool
	windowOnce           sync.Once
	windows              *util.HashMap[string, *PluginWindow]
	pluginInstance       *Instance
	toolCallStartTimeMap *util.HashMap[string, int64] // store the start time of tool calls
}

func (a *APIImpl) ChangeQuery(ctx context.Context, query common.PlainQuery) {
	if query.QueryType != QueryTypeInput && query.QueryType != QueryTypeSelection {
		util.GetLogger().Error(ctx, "ChangeQuery requires QueryType input or selection")
		return
	}
	query.QueryHint = query.QueryHint.NormalizeForQuery(query.QueryType, query.QueryText)
	if query.QueryHint != nil {
		query.QueryHint = query.QueryHint.Clone()
		for i := range query.QueryHint.Elements {
			query.QueryHint.Elements[i].Placeholder = common.I18nString(a.pluginInstance.TranslateMetadataText(ctx, query.QueryHint.Elements[i].Placeholder))
		}
	}
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

// ShowToastMsg displays an independent transient window owned by this plugin.
func (a *APIImpl) ShowToastMsg(ctx context.Context, option ShowToastMsgOption) ShowToastMsgResult {
	if option.Message == "" {
		return ShowToastMsgResult{ErrMsg: "message is required"}
	}
	if option.Action != nil && (option.Action.Name == "" || option.Action.Action == nil) {
		return ShowToastMsgResult{ErrMsg: "action requires a name and callback"}
	}
	icon := a.pluginInstance.Metadata.Icon
	var imageIcon image.Image
	if parsed, err := common.ParseWoxImage(icon); err == nil {
		converted := a.pluginInstance.ConvertIcon(ctx, parsed)
		imageIcon, _ = converted.ToImageWithoutRemoteFetch()
	}
	epoch := a.pluginInstance.toastEpoch.Load()
	toast := notifier.Toast{PluginID: a.pluginInstance.Metadata.Id, Icon: imageIcon, Title: a.GetTranslation(ctx, option.Title), Message: a.GetTranslation(ctx, option.Message), IsValid: func() bool { return a.pluginInstance.toastEpoch.Load() == epoch }}
	if option.Action != nil {
		action := *option.Action
		action.ContextData = make(common.ContextData, len(option.Action.ContextData))
		for key, value := range option.Action.ContextData {
			action.ContextData[key] = value
		}
		toast.ActionLabel = a.GetTranslation(ctx, action.Name)
		toast.OnDone = func(clicked bool) {
			if clicked && toast.IsValid() {
				action.Action(context.WithoutCancel(ctx), ToastMsgActionContext{ContextData: action.ContextData})
			} else if action.Release != nil {
				action.Release()
			}
		}
	}
	if !notifier.ShowToast(toast) {
		return ShowToastMsgResult{ErrMsg: "could not show notification"}
	}
	return ShowToastMsgResult{Success: true}
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

	defaultIcon := a.pluginInstance.Metadata.GetIconOrDefault(a.pluginInstance.PluginDirectory, icons.Get(icons.BrandWox))
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
	count := 0
	if !IsAttentionPluginDisabled() {
		unread, err := GetAttentionManager().UnreadCount(ctx)
		if err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("failed to count unread attention items: %v", err))
			return
		}
		count = int(unread)
	}

	ui := GetPluginManager().GetUI()
	if ui == nil {
		return
	}
	ui.UpdateAttentionUnreadCount(ctx, count)
}

// Log writes plugin output into the shared Wox log tagged with the plugin name.
// Plugins used to get a private logger and log file each; that duplicated every
// line already written here and cost a goroutine, a pipe, and an open file per plugin.
func (a *APIImpl) Log(ctx context.Context, level LogLevel, msg string) {
	logCtx := util.WithComponentContext(ctx, a.pluginInstance.GetName(ctx))
	switch level {
	case LogLevelError:
		logger.Error(logCtx, msg)
	case LogLevelInfo:
		logger.Info(logCtx, msg)
	case LogLevelDebug:
		logger.Debug(logCtx, msg)
	case LogLevelWarning:
		logger.Warn(logCtx, msg)
	}
}

func (a *APIImpl) GetTranslation(ctx context.Context, key string) string {
	return a.pluginInstance.Metadata.translate(ctx, common.I18nString(key))
}

func (a *APIImpl) GetSetting(ctx context.Context, key string) string {
	// Wox-owned workflow settings can derive their displayed value from one atomic persisted state.
	if provider, ok := a.pluginInstance.Plugin.(interface {
		GetSettingOverride(context.Context, string) (string, bool)
	}); ok {
		if value, handled := provider.GetSettingOverride(ctx, key); handled {
			return value
		}
	}
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

// SaveSetting persists a synchronized plugin setting for compatibility with older plugins.
//
// Deprecated: use SetSetting.
func (a *APIImpl) SaveSetting(ctx context.Context, key string, value string, isPlatformSpecific bool) {
	result := a.SetSetting(ctx, SetSettingOption{
		Key:              key,
		Value:            value,
		PlatformSpecific: isPlatformSpecific,
	})
	if !result.Success {
		a.Log(ctx, LogLevelError, fmt.Sprintf("failed to save setting %q: %s", key, result.ErrMsg))
	}
}

// SetSetting persists a plugin setting according to the request's platform and local-only options.
func (a *APIImpl) SetSetting(ctx context.Context, option SetSettingOption) SetSettingResult {
	return a.setHotkeySetting(ctx, option)
}

// persistSetting performs storage and notifications after hotkey registration has been validated.
func (a *APIImpl) persistSetting(ctx context.Context, option SetSettingOption) SetSettingResult {
	if option.Key == "" {
		return SetSettingResult{ErrMsg: "setting key cannot be empty"}
	}

	finalKey := option.Key
	if option.PlatformSpecific {
		finalKey = option.Key + "@" + util.GetCurrentPlatform()
	} else {
		// if not platform specific, remove platform specific setting, otherwise it will be loaded first
		platformSpecificKey := option.Key + "@" + util.GetCurrentPlatform()
		var deleteErr error
		if option.IsLocal {
			deleteErr = a.pluginInstance.Setting.DeleteLocal(platformSpecificKey)
		} else {
			deleteErr = a.pluginInstance.Setting.Delete(platformSpecificKey)
		}
		if deleteErr != nil {
			return SetSettingResult{ErrMsg: fmt.Sprintf("failed to remove platform-specific setting: %s", deleteErr)}
		}
	}

	existValue, exist := a.pluginInstance.Setting.Get(finalKey)
	var saveErr error
	if option.IsLocal {
		saveErr = a.pluginInstance.Setting.SetLocal(finalKey, option.Value)
	} else {
		saveErr = a.pluginInstance.Setting.Set(finalKey, option.Value)
	}
	if saveErr != nil {
		return SetSettingResult{ErrMsg: fmt.Sprintf("failed to persist setting: %s", saveErr)}
	}

	if !exist || existValue != option.Value {
		for _, callback := range a.pluginInstance.SettingChangeCallbacks {
			util.Go(ctx, "plugin setting change callback", func() {
				callback(ctx, option.Key, option.Value)
			})
		}
	}
	return SetSettingResult{Success: true}
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

func (a *APIImpl) OnDragOut(ctx context.Context, option DragOutListenOption) DragOutListenResult {
	if option.Callback == nil {
		return DragOutListenResult{Success: false}
	}
	a.pluginInstance.DragOutCallbacks = append(a.pluginInstance.DragOutCallbacks, option.Callback)
	return DragOutListenResult{Success: true}
}

func (a *APIImpl) RegisterQueryCommands(ctx context.Context, commands []MetadataCommand) {
	if err := validateCommandDebounce(commands); err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("register query commands: %v", err))
		return
	}
	a.pluginInstance.setRuntimeQueryCommands(commands)
}

func (a *APIImpl) RegisterTriggerKeyword(ctx context.Context, option RegisterTriggerKeywordOption) RegisterTriggerKeywordResult {
	return RegisterTriggerKeywordResult{Success: GetPluginManager().registerTriggerKeyword(a.pluginInstance, option)}
}

func (a *APIImpl) UnregisterTriggerKeyword(ctx context.Context, option UnregisterTriggerKeywordOption) UnregisterTriggerKeywordResult {
	a.pluginInstance.unregisterTriggerKeyword(option.Keyword)
	return UnregisterTriggerKeywordResult{Success: true}
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
	debugTraceName := opts.DebugTraceName
	if debugTraceName == "" {
		debugTraceName = "chat"
	}

	for iteration := 0; policy.MaxIterations < 0 || iteration < policy.MaxIterations; iteration++ {
		iterationNumber := iteration + 1
		modelCallId := fmt.Sprintf("%s-%d", debugTraceName, iterationNumber)
		if loopCtx.Err() != nil {
			opts.DebugTrace.AppendEvent(common.AIChatDebugEvent{
				Type:      common.AIChatDebugEventModelCallError,
				Name:      debugTraceName,
				Iteration: iterationNumber,
				CallId:    modelCallId,
				Model:     model,
				Status:    string(common.ChatStreamStatusError),
				Error:     "chat loop cancelled",
			})
			callback(common.ChatStreamData{Status: common.ChatStreamStatusError, Data: "chat loop cancelled"})
			return
		}

		opts.DebugTrace.AppendEvent(common.AIChatDebugEvent{
			Type:         common.AIChatDebugEventModelCallStarted,
			Name:         debugTraceName,
			Iteration:    iterationNumber,
			CallId:       modelCallId,
			Model:        model,
			Status:       "started",
			Request:      cloneDebugConversations(conversations),
			VisibleTools: debugToolSummaries(opts.Tools),
		})

		if model.Provider.IsInstalledCLI() {
			opts.ExecuteTool = func(toolCtx context.Context, option common.AgentToolExecutionOption) (result common.AgentToolExecutionResult) {
				state := common.ChatStreamData{ToolCalls: []common.ToolCallInfo{option.Call}}
				defer func() {
					if recovered := recover(); recovered != nil {
						util.GetLogger().Error(toolCtx, fmt.Sprintf("AI: installed tool execution panicked: %v", recovered))
						state.ToolCalls[0].Status = common.ToolCallStatusFailed
						state.ToolCalls[0].Response = "tool execution failed"
						option.OnUpdate(state.ToolCalls[0])
					}
					result.Call = state.ToolCalls[0]
				}()
				result.Result = a.runSingleToolCall(toolCtx, option.Tool, &state, 0, opts, func(update common.ChatStreamData) {
					option.OnUpdate(update.ToolCalls[0])
				}, map[string]int{}, iterationNumber, modelCallId)
				return result
			}
		}
		stream, err := provider.ChatStream(loopCtx, model, conversations, opts)
		if err != nil {
			opts.DebugTrace.AppendEvent(common.AIChatDebugEvent{
				Type:      common.AIChatDebugEventModelCallError,
				Name:      debugTraceName,
				Iteration: iterationNumber,
				CallId:    modelCallId,
				Model:     model,
				Status:    string(common.ChatStreamStatusError),
				Error:     err.Error(),
			})
			callback(common.ChatStreamData{Status: common.ChatStreamStatusError, Data: err.Error(), ToolCalls: []common.ToolCallInfo{}})
			return
		}

		streamedResult, drainErr := a.drainStream(loopCtx, stream, callback, opts.Tools)
		if drainErr != nil {
			opts.DebugTrace.AppendEvent(common.AIChatDebugEvent{
				Type:      common.AIChatDebugEventModelCallError,
				Name:      debugTraceName,
				Iteration: iterationNumber,
				CallId:    modelCallId,
				Model:     model,
				Status:    string(common.ChatStreamStatusError),
				Error:     drainErr.Error(),
			})
			callback(common.ChatStreamData{Status: common.ChatStreamStatusError, Data: drainErr.Error(), ToolCalls: []common.ToolCallInfo{}})
			return
		}
		if streamedResult == nil {
			opts.DebugTrace.AppendEvent(common.AIChatDebugEvent{
				Type:      common.AIChatDebugEventModelCallError,
				Name:      debugTraceName,
				Iteration: iterationNumber,
				CallId:    modelCallId,
				Model:     model,
				Status:    string(common.ChatStreamStatusError),
				Error:     "stream ended without a result",
			})
			// drainStream already reported a terminal status; nothing more to do.
			return
		}

		// No tool calls means the model is done.
		if len(streamedResult.ToolCalls) == 0 || (model.Provider.IsInstalledCLI() && !hasPendingToolCalls(streamedResult.ToolCalls)) {
			finalResult := *streamedResult
			finalResult.Status = common.ChatStreamStatusFinished
			opts.DebugTrace.AppendEvent(common.AIChatDebugEvent{
				Type:      common.AIChatDebugEventModelCallFinished,
				Name:      debugTraceName,
				Iteration: iterationNumber,
				CallId:    modelCallId,
				Model:     model,
				Status:    string(finalResult.Status),
				Response:  buildDebugResponseConversations(modelCallId, &finalResult),
			})
			callback(finalResult)
			return
		}

		opts.DebugTrace.AppendEvent(common.AIChatDebugEvent{
			Type:      common.AIChatDebugEventModelCallFinished,
			Name:      debugTraceName,
			Iteration: iterationNumber,
			CallId:    modelCallId,
			Model:     model,
			Status:    string(streamedResult.Status),
			Response:  buildDebugResponseConversations(modelCallId, streamedResult),
		})

		allSucceeded := a.executeToolCalls(loopCtx, streamedResult, opts, callback, iterationNumber, modelCallId)

		if !allSucceeded && !policy.RetryOnFailure {
			streamedResult.Status = common.ChatStreamStatusError
			callback(*streamedResult)
			return
		}

		// load_tools expands only this chat loop's visible tool set; the global
		// registry remains the discovery source and is not mutated.
		opts.Tools = ai.AppendRequestedTools(opts.Tools, streamedResult.ToolCalls)

		// Append the assistant message so the model sees its own response
		// in the next iteration. Without this the model may repeat itself
		// because it has no memory of what it already said.
		conversations = append(conversations, common.Conversation{
			Id:        fmt.Sprintf("%s-assistant", modelCallId),
			Role:      common.ConversationRoleAssistant,
			Text:      streamedResult.Data,
			Reasoning: streamedResult.Reasoning,
			Timestamp: util.GetSystemTimestamp(),
		})

		// Append tool results as tool-role conversations so the next iteration
		// shows the model what each tool returned.
		conversations = append(conversations, buildToolConversations(streamedResult)...)

		// Sync this iteration's tool results to the UI before looping.
		callback(*streamedResult)
	}

	opts.DebugTrace.AppendEvent(common.AIChatDebugEvent{
		Type:      common.AIChatDebugEventModelCallError,
		Name:      debugTraceName,
		Iteration: policy.MaxIterations,
		CallId:    fmt.Sprintf("%s-%d", debugTraceName, policy.MaxIterations),
		Model:     model,
		Status:    string(common.ChatStreamStatusError),
		Error:     fmt.Sprintf("chat loop exceeded max iterations (%d)", policy.MaxIterations),
	})
	callback(common.ChatStreamData{
		Status:    common.ChatStreamStatusError,
		Data:      fmt.Sprintf("chat loop exceeded max iterations (%d)", policy.MaxIterations),
		ToolCalls: []common.ToolCallInfo{},
	})
}

// drainStream consumes a provider stream until it either reaches the Streamed
// status (returning the aggregated result) or hits a terminal error. Streaming
// chunks are forwarded to the callback as they arrive.
func (a *APIImpl) drainStream(ctx context.Context, stream ai.ChatStream, callback common.ChatStreamFunc, tools []common.Tool) (*common.ChatStreamData, error) {
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
		ai.AnnotateToolCalls(streamResult.ToolCalls, tools)

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
func (a *APIImpl) executeToolCalls(ctx context.Context, streamedResult *common.ChatStreamData, options common.ChatOptions, callback common.ChatStreamFunc, iteration int, parentCallId string) bool {
	streamedResult.Status = common.ChatStreamStatusRunningToolCall
	ai.AnnotateToolCalls(streamedResult.ToolCalls, options.Tools)

	var sw sync.WaitGroup
	retryCounts := make(map[string]int)

	for toolCallIndex, toolCall := range streamedResult.ToolCalls {
		tool, ok := findVisibleTool(options.Tools, toolCall.Name)
		if !ok {
			options.DebugTrace.AppendEvent(common.AIChatDebugEvent{
				Type:         common.AIChatDebugEventToolCallStarted,
				Name:         "tool_call",
				Iteration:    iteration,
				CallId:       toolCall.Id,
				ParentCallId: parentCallId,
				Status:       string(toolCall.Status),
				ToolCallInfo: &toolCall,
			})
			util.GetLogger().Error(ctx, fmt.Sprintf("AI: tool not available in this chat step: %s", toolCall.Name))
			streamedResult.ToolCalls[toolCallIndex].Status = common.ToolCallStatusSucceeded
			streamedResult.ToolCalls[toolCallIndex].Response = fmt.Sprintf("tool %q is not loaded; call load_tools first", toolCall.Name)
			streamedResult.ToolCalls[toolCallIndex].EndTimestamp = util.GetSystemTimestamp()
			failedToolCall := streamedResult.ToolCalls[toolCallIndex]
			options.DebugTrace.AppendEvent(common.AIChatDebugEvent{
				Type:         common.AIChatDebugEventToolCallFinished,
				Name:         "tool_call",
				Iteration:    iteration,
				CallId:       failedToolCall.Id,
				ParentCallId: parentCallId,
				Status:       string(failedToolCall.Status),
				ToolCallInfo: &failedToolCall,
			})
			continue
		}

		sw.Add(1)
		util.Go(ctx, "ai tool call execution", func() {
			defer sw.Done()
			a.runSingleToolCall(ctx, tool, streamedResult, toolCallIndex, options, callback, retryCounts, iteration, parentCallId)
		}, func() {
			defer sw.Done()
			util.GetLogger().Error(ctx, fmt.Sprintf("AI: tool execution panicked, name: %s", tool.Name))
			streamedResult.ToolCalls[toolCallIndex].Status = common.ToolCallStatusFailed
			streamedResult.ToolCalls[toolCallIndex].Response = "tool execution failed with panic"
			streamedResult.ToolCalls[toolCallIndex].EndTimestamp = util.GetSystemTimestamp()
			failedToolCall := streamedResult.ToolCalls[toolCallIndex]
			options.DebugTrace.AppendEvent(common.AIChatDebugEvent{
				Type:         common.AIChatDebugEventToolCallFinished,
				Name:         "tool_call",
				Iteration:    iteration,
				CallId:       failedToolCall.Id,
				ParentCallId: parentCallId,
				Status:       string(failedToolCall.Status),
				ToolCallInfo: &failedToolCall,
			})
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
func (a *APIImpl) runSingleToolCall(ctx context.Context, tool common.Tool, streamedResult *common.ChatStreamData, toolCallIndex int, options common.ChatOptions, callback common.ChatStreamFunc, retryCounts map[string]int, iteration int, parentCallId string) common.ToolResult {
	ai.ApplyToolOrigin(&streamedResult.ToolCalls[toolCallIndex], tool)
	toolCall := streamedResult.ToolCalls[toolCallIndex]
	util.GetLogger().Info(ctx, fmt.Sprintf("AI: Executing tool: %s with args: %v, toolcall id: %s", tool.Name, toolCall.Arguments, toolCall.Id))

	streamedResult.ToolCalls[toolCallIndex].Status = common.ToolCallStatusRunning
	runningToolCall := streamedResult.ToolCalls[toolCallIndex]
	options.DebugTrace.AppendEvent(common.AIChatDebugEvent{
		Type:         common.AIChatDebugEventToolCallStarted,
		Name:         "tool_call",
		Iteration:    iteration,
		CallId:       runningToolCall.Id,
		ParentCallId: parentCallId,
		Status:       string(runningToolCall.Status),
		ToolCallInfo: &runningToolCall,
	})
	callback(*streamedResult)

	toolResponse, toolErr := tool.Callback(ctx, toolCall.Arguments)
	if toolErr != nil {
		util.GetLogger().Info(ctx, fmt.Sprintf("AI: tool returned error (still counts as a result): %s", toolErr.Error()))
		// Allow a few retries for transient failures within the same iteration.
		retryCounts[tool.Name]++
		if retryCounts[tool.Name] < options.LoopPolicy.MaxRetries {
			toolResponse, toolErr = tool.Callback(ctx, toolCall.Arguments)
		}
	}
	// Any response from the tool — even an error message like "file not found"
	// or "permission denied" — is a valid result. The AI can read the error
	// and self-correct. Only mark as failed when the tool crashed without
	// producing any usable output (handled by the panic recovery above).
	if toolErr != nil {
		streamedResult.ToolCalls[toolCallIndex].Status = common.ToolCallStatusSucceeded
		streamedResult.ToolCalls[toolCallIndex].Response = toolErr.Error()
	} else {
		streamedResult.ToolCalls[toolCallIndex].Status = common.ToolCallStatusSucceeded
		streamedResult.ToolCalls[toolCallIndex].Response = toolResponse.Text
	}
	streamedResult.ToolCalls[toolCallIndex].EndTimestamp = util.GetSystemTimestamp()
	finishedToolCall := streamedResult.ToolCalls[toolCallIndex]
	options.DebugTrace.AppendEvent(common.AIChatDebugEvent{
		Type:         common.AIChatDebugEventToolCallFinished,
		Name:         "tool_call",
		Iteration:    iteration,
		CallId:       finishedToolCall.Id,
		ParentCallId: parentCallId,
		Status:       string(finishedToolCall.Status),
		ToolCallInfo: &finishedToolCall,
	})
	callback(*streamedResult)
	return toolResponse
}

// hasPendingToolCalls prevents an installed agent's completed tool calls from executing twice.
func hasPendingToolCalls(calls []common.ToolCallInfo) bool {
	for _, call := range calls {
		if call.Status != common.ToolCallStatusSucceeded && call.Status != common.ToolCallStatusFailed {
			return true
		}
	}
	return false
}

// buildToolConversations turns a streamed result's tool calls into tool-role
// conversation entries so the next loop iteration presents them to the model.
func buildToolConversations(streamedResult *common.ChatStreamData) []common.Conversation {
	convs := make([]common.Conversation, 0, len(streamedResult.ToolCalls))
	for _, tc := range streamedResult.ToolCalls {
		convs = append(convs, common.Conversation{
			Id:           tc.Id,
			Role:         common.ConversationRoleTool,
			Text:         tc.Delta,
			Reasoning:    "",
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

// debugToolSummaries strips executable callbacks from tool definitions for trace output.
func debugToolSummaries(tools []common.Tool) []common.AIChatDebugTool {
	summaries := make([]common.AIChatDebugTool, 0, len(tools))
	for _, tool := range tools {
		server := ""
		if tool.ServerConfig != nil {
			server = tool.ServerConfig.Name
		}
		if tool.PluginName != "" {
			server = tool.PluginName
		}
		summaries = append(summaries, common.AIChatDebugTool{
			Name:        tool.Name,
			Description: tool.Description,
			Source:      tool.Source,
			Server:      server,
		})
	}
	return summaries
}

// buildDebugResponseConversations converts a streamed response into trace-friendly messages.
func buildDebugResponseConversations(parentCallId string, streamedResult *common.ChatStreamData) []common.Conversation {
	if len(streamedResult.Conversations) > 0 {
		return cloneDebugConversations(streamedResult.Conversations)
	}
	conversations := []common.Conversation{}
	if streamedResult.Data != "" || streamedResult.Reasoning != "" {
		conversations = append(conversations, common.Conversation{
			Id:        parentCallId + "-assistant",
			Role:      common.ConversationRoleAssistant,
			Text:      streamedResult.Data,
			Reasoning: streamedResult.Reasoning,
			Timestamp: util.GetSystemTimestamp(),
		})
	}
	conversations = append(conversations, buildToolConversations(streamedResult)...)
	return cloneDebugConversations(conversations)
}

// cloneDebugConversations detaches trace snapshots from mutable stream state.
func cloneDebugConversations(conversations []common.Conversation) []common.Conversation {
	cloned := make([]common.Conversation, len(conversations))
	for i, conversation := range conversations {
		cloned[i] = conversation
		cloned[i].Attachments = append([]common.AIChatAttachment(nil), conversation.Attachments...)
		cloned[i].Images = append([]common.WoxImage(nil), conversation.Images...)
		cloned[i].SkillRefs = append([]common.AISkillRef(nil), conversation.SkillRefs...)
		cloned[i].Mentions = append([]common.AIMentionRef(nil), conversation.Mentions...)
		if conversation.ToolCallInfo.Arguments != nil {
			cloned[i].ToolCallInfo.Arguments = map[string]any{}
			for key, value := range conversation.ToolCallInfo.Arguments {
				cloned[i].ToolCallInfo.Arguments[key] = value
			}
		}
	}
	return cloned
}

func (a *APIImpl) OnMRURestore(ctx context.Context, callback func(ctx context.Context, mruData MRUData) (*QueryResult, error)) {
	if !a.pluginInstance.Metadata.IsSupportFeature(MetadataFeatureMRU) {
		a.Log(ctx, LogLevelError, "plugin has no access to MRU feature")
		return
	}

	a.pluginInstance.MRURestoreCallbacks = append(a.pluginInstance.MRURestoreCallbacks, callback)
}

func (a *APIImpl) UpdateResult(ctx context.Context, result UpdatableResult) bool {
	// Preserve an explicit query scope: stable result ids can occur in several
	// cached queries, and id-only lookup can select an obsolete action snapshot.
	if util.GetContextSessionId(ctx) == "" || util.GetContextQueryId(ctx) == "" {
		if sessionId, queryId := GetPluginManager().GetQueryInfoByResultId(result.Id); sessionId != "" {
			ctx = util.WithQueryIdContext(util.WithSessionContext(ctx, sessionId), queryId)
		}
	}
	if util.GetContextSessionId(ctx) != "" && util.GetContextQueryId(ctx) != "" {
		if _, found := GetPluginManager().resultStore.findWithContext(ctx, result.Id); !found {
			return false
		}
	}
	if _, command, err := WebViewScriptUpdate(result.Preview); command {
		cache, found := GetPluginManager().resultStore.findWithContext(ctx, result.Id)
		if err != nil || !found || cache.PluginInstance != a.pluginInstance || !IsInlineWebViewPreview(cache.Result.Preview) {
			return false
		}
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
	// pipelines are concurrent. Push by query id and let UI accept or reject
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
	GetPluginManager().GetUI().RefreshQuery(ctx, common.RefreshQueryOptions{
		PreserveSelectedIndex: param.PreserveSelectedIndex,
		SelectedResultId:      param.SelectedResultId,
	})
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
		if params.WoxImage.IsAnimatedGif() {
			path, cleanup, err := params.WoxImage.ResolveAnimatedGIFPath(ctx)
			if err != nil {
				a.Log(ctx, LogLevelError, fmt.Sprintf("failed to resolve animated gif for clipboard: %v", err))
				return
			}
			defer cleanup()
			if err := clipboard.WriteAnimatedGIF(path); err != nil {
				a.Log(ctx, LogLevelError, fmt.Sprintf("failed to copy animated gif to clipboard: %v", err))
			}
			return
		}

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
	// Screenshot API options are translated in core where the plugin caller is known. Keeping UI
	// on a request-only contract avoids making the UI infer SDK defaults from plugin runtime details.
	request.HideAnnotationToolbar = option.HideAnnotationToolbar
	request.AutoConfirm = option.AutoConfirm
	if !a.pluginInstance.IsSystemPlugin {
		// Third-party screenshot callers need a visible identity marker in the floating toolbox.
		// The UI cannot reliably infer the plugin from the generic CaptureScreenshot method,
		// so core resolves the metadata icon here and sends only the render-ready WoxImage.
		callerIcon := a.pluginInstance.Metadata.GetIconOrDefault(a.pluginInstance.PluginDirectory, icons.Get(icons.BrandWox))
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

// GetThemeColors maps the active Wox theme to opaque HTML-safe colors.
func (a *APIImpl) GetThemeColors(ctx context.Context, _ GetThemeColorsOption) GetThemeColorsResult {
	ui := GetPluginManager().GetUI()
	if ui == nil {
		return themeColorsResult(common.Theme{}.PluginColors())
	}
	theme := ui.GetCurrentTheme(ctx)
	result := themeColorsResult(theme.PluginColors())
	a.Log(ctx, LogLevelDebug, fmt.Sprintf("theme colors themeId=%s dark=%t background=%s text=%s", theme.ThemeId, result.Dark, result.Background, result.Text))
	return result
}

func themeColorsResult(colors common.ThemePluginColors) GetThemeColorsResult {
	return GetThemeColorsResult{
		Background:    colors.Background,
		Text:          colors.Text,
		SecondaryText: colors.SecondaryText,
		Border:        colors.Border,
		Accent:        colors.Accent,
		AccentText:    colors.AccentText,
		Selection:     colors.Selection,
		Dark:          colors.Dark,
	}
}

// GetCacheFolder returns this plugin's cache directory, creating it if needed.
func (a *APIImpl) GetCacheFolder(ctx context.Context) string {
	if a.pluginInstance == nil {
		return ""
	}

	folder, err := util.GetLocation().EnsurePluginCacheDirectory(a.pluginInstance.Metadata.Id)
	if err != nil {
		a.Log(ctx, LogLevelError, fmt.Sprintf("failed to create plugin cache folder: %v", err))
		return ""
	}
	return folder
}

func NewAPI(instance *Instance) API {
	return &APIImpl{
		pluginInstance:       instance,
		toolCallStartTimeMap: util.NewHashMap[string, int64](),
	}
}
