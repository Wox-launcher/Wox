package plugin

import (
	"context"
	"errors"
	"fmt"
	"github.com/samber/lo"
	"io"
	"path"
	"wox/ai"
	"wox/i18n"
	"wox/setting"
	"wox/setting/definition"
	"wox/share"
	"wox/util"
)

type LogLevel = string

const (
	LogLevelInfo    LogLevel = "Info"
	LogLevelError   LogLevel = "Error"
	LogLevelDebug   LogLevel = "Debug"
	LogLevelWarning LogLevel = "Warning"
)

type API interface {
	ChangeQuery(ctx context.Context, query share.PlainQuery)
	HideApp(ctx context.Context)
	ShowApp(ctx context.Context)
	Notify(ctx context.Context, title string, description string)
	Log(ctx context.Context, level LogLevel, msg string)
	GetTranslation(ctx context.Context, key string) string
	GetSetting(ctx context.Context, key string) string
	SaveSetting(ctx context.Context, key string, value string, isPlatformSpecific bool)
	OnSettingChanged(ctx context.Context, callback func(key string, value string))
	OnGetDynamicSetting(ctx context.Context, callback func(key string) definition.PluginSettingDefinitionItem)
	RegisterQueryCommands(ctx context.Context, commands []MetadataCommand)
	AIChatStream(ctx context.Context, model ai.Model, conversations []ai.Conversation, callback ai.ChatStreamFunc) error
}

type APIImpl struct {
	pluginInstance          *Instance
	logger                  *util.Log
	settingChangeCallbacks  []func(key string, value string)
	dynamicSettingCallbacks []func(key string) definition.PluginSettingDefinitionItem
}

func (a *APIImpl) ChangeQuery(ctx context.Context, query share.PlainQuery) {
	GetPluginManager().GetUI().ChangeQuery(ctx, query)
}

func (a *APIImpl) HideApp(ctx context.Context) {
	GetPluginManager().GetUI().HideApp(ctx)
}

func (a *APIImpl) ShowApp(ctx context.Context) {
	GetPluginManager().GetUI().ShowApp(ctx, share.ShowContext{
		SelectAll: true,
	})
}

func (a *APIImpl) Notify(ctx context.Context, title string, description string) {
	GetPluginManager().GetUI().Notify(ctx, title, description)
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
		for _, callback := range a.settingChangeCallbacks {
			callback(key, value)
		}
	}
}

func (a *APIImpl) OnSettingChanged(ctx context.Context, callback func(key string, value string)) {
	a.settingChangeCallbacks = append(a.settingChangeCallbacks, callback)
}

func (a *APIImpl) OnGetDynamicSetting(ctx context.Context, callback func(key string) definition.PluginSettingDefinitionItem) {
	a.pluginInstance.DynamicSettingCallbacks = append(a.pluginInstance.DynamicSettingCallbacks, callback)
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

func (a *APIImpl) AIChatStream(ctx context.Context, model ai.Model, conversations []ai.Conversation, callback ai.ChatStreamFunc) error {
	//check if plugin has the feature permission
	if !a.pluginInstance.Metadata.IsSupportFeature(MetadataFeatureAI) {
		return fmt.Errorf("plugin has no access to ai feature")
	}

	provider, providerErr := GetPluginManager().GetAIProvider(ctx, model.Provider)
	if providerErr != nil {
		return providerErr
	}

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
					callback(ai.ChatStreamTypeFinished, "")
					return
				}

				if streamErr != nil {
					util.GetLogger().Info(ctx, fmt.Sprintf("failed to read stream: %s", streamErr.Error()))
					callback(ai.ChatStreamTypeError, streamErr.Error())
					return
				}

				callback(ai.ChatStreamTypeStreaming, response)
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
