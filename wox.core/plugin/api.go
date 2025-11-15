package plugin

import (
	"context"
	"fmt"
	"path"
	"sync"
	"time"
	"wox/ai"
	"wox/common"
	"wox/i18n"
	"wox/setting"
	"wox/setting/definition"
	"wox/util"

	"github.com/samber/lo"
)

type LogLevel = string

const (
	LogLevelInfo    LogLevel = "Info"
	LogLevelError   LogLevel = "Error"
	LogLevelDebug   LogLevel = "Debug"
	LogLevelWarning LogLevel = "Warning"
)

type API interface {
	ChangeQuery(ctx context.Context, query common.PlainQuery)
	HideApp(ctx context.Context)
	ShowApp(ctx context.Context)
	Notify(ctx context.Context, description string)
	Log(ctx context.Context, level LogLevel, msg string)
	GetTranslation(ctx context.Context, key string) string
	GetSetting(ctx context.Context, key string) string
	SaveSetting(ctx context.Context, key string, value string, isPlatformSpecific bool)
	OnSettingChanged(ctx context.Context, callback func(key string, value string))
	OnGetDynamicSetting(ctx context.Context, callback func(key string) definition.PluginSettingDefinitionItem)
	OnDeepLink(ctx context.Context, callback func(arguments map[string]string))
	OnUnload(ctx context.Context, callback func())
	OnMRURestore(ctx context.Context, callback func(mruData MRUData) (*QueryResult, error))
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
}

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
	GetPluginManager().GetUI().Notify(ctx, common.NotifyMsg{
		PluginId:       a.pluginInstance.Metadata.Id,
		Text:           a.GetTranslation(ctx, message),
		DisplaySeconds: 5,
	})
}

func (a *APIImpl) Log(ctx context.Context, level LogLevel, msg string) {
	logCtx := util.NewComponentContext(ctx, a.pluginInstance.Metadata.Name)
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
	if a.pluginInstance.IsSystemPlugin {
		return i18n.GetI18nManager().TranslateWox(ctx, key)
	} else {
		// Try plugin translation first
		translated := i18n.GetI18nManager().TranslatePlugin(ctx, key, a.pluginInstance.PluginDirectory)
		// If translation failed, fallback to system translation
		// This handles cases where third-party plugins use system i18n keys (like notification messages)
		if key == translated {
			translated = i18n.GetI18nManager().TranslateWox(ctx, key)
		}
		return translated
	}
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
			callback(key, value)
		}
	}
}

func (a *APIImpl) OnSettingChanged(ctx context.Context, callback func(key string, value string)) {
	a.pluginInstance.SettingChangeCallbacks = append(a.pluginInstance.SettingChangeCallbacks, callback)
}

func (a *APIImpl) OnGetDynamicSetting(ctx context.Context, callback func(key string) definition.PluginSettingDefinitionItem) {
	a.pluginInstance.DynamicSettingCallbacks = append(a.pluginInstance.DynamicSettingCallbacks, callback)
}

func (a *APIImpl) OnDeepLink(ctx context.Context, callback func(arguments map[string]string)) {
	if !a.pluginInstance.Metadata.IsSupportFeature(MetadataFeatureDeepLink) {
		a.Log(ctx, LogLevelError, "plugin has no access to deep link feature")
		return
	}

	a.pluginInstance.DeepLinkCallbacks = append(a.pluginInstance.DeepLinkCallbacks, callback)
}

func (a *APIImpl) OnUnload(ctx context.Context, callback func()) {
	a.pluginInstance.UnloadCallbacks = append(a.pluginInstance.UnloadCallbacks, callback)
}

func (a *APIImpl) RegisterQueryCommands(ctx context.Context, commands []MetadataCommand) {
	a.pluginInstance.Setting.QueryCommands.Set(lo.Map(commands, func(command MetadataCommand, _ int) setting.PluginQueryCommand {
		return setting.PluginQueryCommand{
			Command:     command.Command,
			Description: command.Description,
		}
	}))
}

func (a *APIImpl) AIChatStream(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions, callback common.ChatStreamFunc) error {
	//check if plugin has the feature permission
	if !a.pluginInstance.Metadata.IsSupportFeature(MetadataFeatureAI) {
		return fmt.Errorf("plugin has no access to ai feature")
	}

	provider, providerErr := GetPluginManager().GetAIProvider(ctx, model.Provider)
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

func (a *APIImpl) OnMRURestore(ctx context.Context, callback func(mruData MRUData) (*QueryResult, error)) {
	if !a.pluginInstance.Metadata.IsSupportFeature(MetadataFeatureMRU) {
		a.Log(ctx, LogLevelError, "plugin has no access to MRU feature")
		return
	}

	a.pluginInstance.MRURestoreCallbacks = append(a.pluginInstance.MRURestoreCallbacks, callback)
}

func (a *APIImpl) UpdateResult(ctx context.Context, result UpdatableResult) bool {
	polishedResult := GetPluginManager().PolishUpdatableResult(ctx, a.pluginInstance, result)
	success := GetPluginManager().GetUI().UpdateResult(ctx, polishedResult)
	return success
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

func NewAPI(instance *Instance) API {
	apiImpl := &APIImpl{pluginInstance: instance}
	logFolder := path.Join(util.GetLocation().GetLogPluginDirectory(), instance.Metadata.Name)
	apiImpl.logger = util.CreateLogger(logFolder)
	apiImpl.toolCallStartTimeMap = util.NewHashMap[string, int64]()
	return apiImpl
}
