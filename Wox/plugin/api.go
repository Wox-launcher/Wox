package plugin

import (
	"context"
	"path"
	"wox/util"
)

type API interface {
	ChangeQuery(ctx context.Context, query string)
	HideApp(ctx context.Context)
	ShowApp(ctx context.Context)
	ShowMsg(ctx context.Context, title string, description string, icon string)
	Log(ctx context.Context, msg string)
	GetTranslation(ctx context.Context, key string) string
	GetSetting(ctx context.Context, key string) string
	SaveSetting(ctx context.Context, key string, value string)
	OnSettingChanged(ctx context.Context, callback func(key string, value string))
}

type APIImpl struct {
	pluginInstance         *Instance
	logger                 *util.Log
	settingChangeCallbacks []func(key string, value string)
}

func (a *APIImpl) ChangeQuery(ctx context.Context, query string) {
	GetPluginManager().GetUI().ChangeQuery(ctx, query)
}

func (a *APIImpl) HideApp(ctx context.Context) {
	GetPluginManager().GetUI().HideApp(ctx)
}

func (a *APIImpl) ShowApp(ctx context.Context) {
	GetPluginManager().GetUI().ShowApp(ctx)
}

func (a *APIImpl) ShowMsg(ctx context.Context, title string, description string, icon string) {
	GetPluginManager().GetUI().ShowMsg(ctx, title, description, icon)
}

func (a *APIImpl) Log(ctx context.Context, msg string) {
	a.logger.Info(ctx, msg)
}

func (a *APIImpl) GetTranslation(ctx context.Context, key string) string {
	return ""
}

func (a *APIImpl) GetSetting(ctx context.Context, key string) string {
	v, exist := a.pluginInstance.Setting.GetCustomizedSetting(key)
	if exist {
		return v
	}
	return ""
}

func (a *APIImpl) SaveSetting(ctx context.Context, key string, value string) {
	a.pluginInstance.Setting.CustomizedSettings.Store(key, value)
	a.pluginInstance.SaveSetting(ctx)
	for _, callback := range a.settingChangeCallbacks {
		callback(key, value)
	}
}

func (a *APIImpl) OnSettingChanged(ctx context.Context, callback func(key string, value string)) {
	a.settingChangeCallbacks = append(a.settingChangeCallbacks, callback)
}

func NewAPI(instance *Instance) API {
	apiImpl := &APIImpl{pluginInstance: instance}
	logFolder := path.Join(util.GetLocation().GetLogPluginDirectory(), instance.Metadata.Name)
	apiImpl.logger = util.CreateLogger(logFolder)
	return apiImpl
}
