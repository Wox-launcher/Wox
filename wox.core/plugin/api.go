package plugin

import (
	"context"
	"fmt"
	"path"
	"time"
	"wox/ai"
	"wox/common"
	"wox/i18n"
	"wox/setting"
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
	OnGetDynamicSetting(ctx context.Context, callback func(key string) string)
	OnDeepLink(ctx context.Context, callback func(arguments map[string]string))
	OnUnload(ctx context.Context, callback func())
	RegisterQueryCommands(ctx context.Context, commands []MetadataCommand)
	AIChatStream(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions, callback common.ChatStreamFunc) error
}

type APIImpl struct {
	pluginInstance   *Instance
	logger           *util.Log
	toolCallStartMap *util.HashMap[string, int64]
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
		DisplaySeconds: 3,
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
		return i18n.GetI18nManager().TranslatePlugin(ctx, key, a.pluginInstance.PluginDirectory)
	}
}

func (a *APIImpl) GetSetting(ctx context.Context, key string) string {
	// try to get platform specific setting first
	platformSpecificKey := key + "@" + util.GetCurrentPlatform()
	v, exist := a.pluginInstance.Setting.GetSetting(platformSpecificKey)
	if exist {
		return v
	}

	v, exist = a.pluginInstance.Setting.GetSetting(key)
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
		a.pluginInstance.Setting.Settings.Delete(key + "@" + util.GetCurrentPlatform())
	}

	existValue, exist := a.pluginInstance.Setting.Settings.Load(finalKey)
	a.pluginInstance.Setting.Settings.Store(finalKey, value)
	saveErr := a.pluginInstance.SaveSetting(ctx)
	if saveErr != nil {
		a.logger.Error(ctx, fmt.Sprintf("failed to save setting: %s", saveErr.Error()))
		return
	}

	if !exist || (existValue != value) {
		for _, callback := range a.pluginInstance.SettingChangeCallbacks {
			callback(key, value)
		}
	}
}

func (a *APIImpl) OnSettingChanged(ctx context.Context, callback func(key string, value string)) {
	a.pluginInstance.SettingChangeCallbacks = append(a.pluginInstance.SettingChangeCallbacks, callback)
}

func (a *APIImpl) OnGetDynamicSetting(ctx context.Context, callback func(key string) string) {
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
	a.pluginInstance.Setting.QueryCommands = lo.Map(commands, func(command MetadataCommand, _ int) setting.PluginQueryCommand {
		return setting.PluginQueryCommand{
			Command:     command.Command,
			Description: command.Description,
		}
	})
	a.pluginInstance.SaveSetting(ctx)
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
						Type:     common.ChatStreamTypeError,
						Data:     streamErr.Error(),
						ToolCall: common.ToolCallInfo{},
					})
					return
				}

				util.GetLogger().Debug(ctx, fmt.Sprintf("AI: Received stream from ai provider: type=%s, response=%s", streamResult.Type, streamResult.Data))

				if streamResult.Type == common.ChatStreamTypeFinished {
					util.GetLogger().Info(ctx, "AI: read stream from ai provider completed")
					callback(common.ChatStreamData{
						Type:     common.ChatStreamTypeFinished,
						Data:     "",
						ToolCall: common.ToolCallInfo{},
					})
					return
				}

				if streamResult.Type == common.ChatStreamTypeToolCall {
					startTime := util.GetSystemTimestamp()
					if v, ok := a.toolCallStartMap.Load(streamResult.ToolCall.Id); ok {
						startTime = v
					} else {
						a.toolCallStartMap.Store(streamResult.ToolCall.Id, startTime)
					}
					streamResult.ToolCall.StartTimestamp = startTime

					if streamResult.ToolCall.Status == common.ToolCallStatusStreaming {
						util.GetLogger().Info(ctx, fmt.Sprintf("AI: Tool call is streaming, delta: %s", streamResult.ToolCall.Delta))
						time.Sleep(time.Millisecond * 100)
						callback(streamResult)
						continue
					}

					if streamResult.ToolCall.Status == common.ToolCallStatusPending {
						util.GetLogger().Info(ctx, fmt.Sprintf("AI: Tool call is pending to execute, name: %s, args: %v", streamResult.ToolCall.Name, streamResult.ToolCall.Arguments))

						for _, tool := range options.Tools {
							if tool.Name == streamResult.ToolCall.Name {
								util.GetLogger().Info(ctx, fmt.Sprintf("AI: Executing tool: %s with args: %v, toolcall id: %s, toolcall status: %s", tool.Name, streamResult.ToolCall.Arguments, streamResult.ToolCall.Id, streamResult.ToolCall.Status))

								callback(common.ChatStreamData{
									Type: common.ChatStreamTypeToolCall,
									Data: "",
									ToolCall: common.ToolCallInfo{
										Id:             streamResult.ToolCall.Id,
										Name:           streamResult.ToolCall.Name,
										Arguments:      streamResult.ToolCall.Arguments,
										Response:       "",
										Status:         common.ToolCallStatusRunning,
										StartTimestamp: startTime,
									},
								})

								// execute tool call in a new goroutine, and send callback continuously to refresh the status
								execCtx, cancelExec := context.WithCancel(ctx)
								var toolResponse common.Conversation
								var toolErr error

								util.Go(ctx, "ai tool call execution", func() {
									toolResponse, toolErr = tool.Callback(ctx, streamResult.ToolCall.Arguments)
									cancelExec()
								}, func() {
									toolErr = fmt.Errorf("tool execution failed with panic")
									cancelExec()
								})

								util.Go(ctx, "ai tool call status update", func() {
									for {
										select {
										case <-execCtx.Done():
											return
										case <-time.After(time.Millisecond * 200):
											callback(common.ChatStreamData{
												Type: common.ChatStreamTypeToolCall,
												Data: "",
												ToolCall: common.ToolCallInfo{
													Id:             streamResult.ToolCall.Id,
													Name:           streamResult.ToolCall.Name,
													Arguments:      streamResult.ToolCall.Arguments,
													Response:       "",
													Status:         common.ToolCallStatusRunning,
													StartTimestamp: startTime,
												},
											})
										}
									}
								})

								<-execCtx.Done()

								endTime := util.GetSystemTimestamp()
								duration := endTime - startTime

								if toolErr != nil {
									util.GetLogger().Error(ctx, fmt.Sprintf("AI: tool execution failed: %s", toolErr.Error()))
									callback(common.ChatStreamData{
										Type: common.ChatStreamTypeToolCall,
										Data: "",
										ToolCall: common.ToolCallInfo{
											Id:             streamResult.ToolCall.Id,
											Name:           streamResult.ToolCall.Name,
											Arguments:      streamResult.ToolCall.Arguments,
											Response:       toolErr.Error(),
											Status:         common.ToolCallStatusFailed,
											StartTimestamp: startTime,
											EndTimestamp:   endTime,
										},
									})
									break
								}

								util.GetLogger().Info(ctx, fmt.Sprintf("AI: Tool execution completed - name: %s, toolCallID: %s, duration: %dms", tool.Name, streamResult.ToolCall.Id, duration))
								util.GetLogger().Info(ctx, fmt.Sprintf("AI: Tool response text: %s", toolResponse.Text))

								callback(common.ChatStreamData{
									Type: common.ChatStreamTypeToolCall,
									Data: "",
									ToolCall: common.ToolCallInfo{
										Id:             streamResult.ToolCall.Id,
										Name:           streamResult.ToolCall.Name,
										Arguments:      streamResult.ToolCall.Arguments,
										Response:       toolResponse.Text,
										Status:         common.ToolCallStatusSucceeded,
										StartTimestamp: startTime,
										EndTimestamp:   endTime,
									},
								})
								break
							}
						}

						// if tool call executed, break the loop
						break
					}
				}

				if streamResult.Type == common.ChatStreamTypeStreaming {
					callback(streamResult)
				}
			}
		})
	}

	return nil
}

func NewAPI(instance *Instance) API {
	apiImpl := &APIImpl{pluginInstance: instance}
	logFolder := path.Join(util.GetLocation().GetLogPluginDirectory(), instance.Metadata.Name)
	apiImpl.logger = util.CreateLogger(logFolder)
	apiImpl.toolCallStartMap = util.NewHashMap[string, int64]()
	return apiImpl
}
