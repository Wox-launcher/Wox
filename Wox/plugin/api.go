package plugin

import (
	"context"
	"fmt"
	"github.com/samber/lo"
	"path"
	"wox/i18n"
	"wox/setting"
	"wox/share"
	"wox/util"
)

type API interface {
	ChangeQuery(ctx context.Context, query share.ChangedQuery)
	HideApp(ctx context.Context)
	ShowApp(ctx context.Context)
	Notify(ctx context.Context, title string, description string)
	Log(ctx context.Context, msg string)
	GetTranslation(ctx context.Context, key string) string
	GetSetting(ctx context.Context, key string) string
	SaveSetting(ctx context.Context, key string, value string, isPlatformSpecific bool)
	OnSettingChanged(ctx context.Context, callback func(key string, value string))
	RegisterQueryCommands(ctx context.Context, commands []MetadataCommand)
}

type APIImpl struct {
	pluginInstance         *Instance
	logger                 *util.Log
	settingChangeCallbacks []func(key string, value string)
}

func (a *APIImpl) ChangeQuery(ctx context.Context, query share.ChangedQuery) {
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

func (a *APIImpl) Log(ctx context.Context, msg string) {
	a.logger.Info(ctx, msg)
	logger.Info(ctx, fmt.Sprintf("[%s] %s", a.pluginInstance.Metadata.Name, msg))
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
	if isPlatformSpecific {
		key = key + "@" + util.GetCurrentPlatform()
	}

	existValue, exist := a.pluginInstance.Setting.Settings.Load(key)
	a.pluginInstance.Setting.Settings.Store(key, value)
	a.pluginInstance.SaveSetting(ctx)

	if !exist || (exist && existValue != value) {
		for _, callback := range a.settingChangeCallbacks {
			callback(key, value)
		}
	}
}

func (a *APIImpl) OnSettingChanged(ctx context.Context, callback func(key string, value string)) {
	a.settingChangeCallbacks = append(a.settingChangeCallbacks, callback)
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

func NewAPI(instance *Instance) API {
	apiImpl := &APIImpl{pluginInstance: instance}
	logFolder := path.Join(util.GetLocation().GetLogPluginDirectory(), instance.Metadata.Name)
	apiImpl.logger = util.CreateLogger(logFolder)
	return apiImpl
}
