package plugin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"wox/entity"
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
	ChangeQuery(ctx context.Context, query entity.PlainQuery)
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
	AIChatStream(ctx context.Context, model entity.Model, conversations []entity.Conversation, callback entity.ChatStreamFunc) error
}

type APIImpl struct {
	pluginInstance *Instance
	logger         *util.Log
}

func (a *APIImpl) ChangeQuery(ctx context.Context, query entity.PlainQuery) {
	GetPluginManager().GetUI().ChangeQuery(ctx, query)
}

func (a *APIImpl) HideApp(ctx context.Context) {
	GetPluginManager().GetUI().HideApp(ctx)
}

func (a *APIImpl) ShowApp(ctx context.Context) {
	GetPluginManager().GetUI().ShowApp(ctx, entity.ShowContext{
		SelectAll: true,
	})
}

func (a *APIImpl) Notify(ctx context.Context, message string) {
	GetPluginManager().GetUI().Notify(ctx, entity.NotifyMsg{
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

func (a *APIImpl) AIChatStream(ctx context.Context, model entity.Model, conversations []entity.Conversation, callback entity.ChatStreamFunc) error {
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

	stream, err := provider.ChatStream(ctx, model, conversations)
	if err != nil {
		return err
	}

	if callback != nil {
		util.Go(ctx, "ai chat stream", func() {
			for {
				util.GetLogger().Info(ctx, fmt.Sprintf("reading chat stream"))
				response, streamErr := stream.Receive(ctx)
				if errors.Is(streamErr, io.EOF) {
					util.GetLogger().Info(ctx, "read stream completed")
					callback(entity.ChatStreamTypeFinished, "")
					return
				}

				if streamErr != nil {
					util.GetLogger().Info(ctx, fmt.Sprintf("failed to read stream: %s", streamErr.Error()))
					callback(entity.ChatStreamTypeError, streamErr.Error())
					return
				}

				callback(entity.ChatStreamTypeStreaming, response)
			}
		})
	}

	return nil
}

func NewAPI(instance *Instance) API {
	apiImpl := &APIImpl{pluginInstance: instance}
	logFolder := path.Join(util.GetLocation().GetLogPluginDirectory(), instance.Metadata.Name)
	apiImpl.logger = util.CreateLogger(logFolder)
	return apiImpl
}
