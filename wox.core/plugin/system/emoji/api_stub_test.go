package emoji

import (
	"context"
	"wox/common"
	"wox/plugin"
	"wox/setting/definition"
)

type stubAPI struct {
	settings map[string]string
}

func (s *stubAPI) setting(key string) string {
	if s.settings == nil {
		return ""
	}
	return s.settings[key]
}

func (s *stubAPI) OnGetDynamicSetting(
	context.Context,
	func(context.Context, string) definition.PluginSettingDefinitionItem,
) {
}

func (s *stubAPI) ChangeQuery(ctx context.Context, query common.PlainQuery) {}

func (s *stubAPI) HideApp(ctx context.Context) {}

func (s *stubAPI) ShowApp(ctx context.Context) {}

func (s *stubAPI) Notify(ctx context.Context, message string) {}

func (s *stubAPI) PushAttention(ctx context.Context, request plugin.PushAttentionRequest) {}

func (s *stubAPI) Log(ctx context.Context, level plugin.LogLevel, msg string) {}

func (s *stubAPI) GetTranslation(ctx context.Context, key string) string {
	return key
}

func (s *stubAPI) GetSetting(ctx context.Context, key string) string {
	return s.setting(key)
}

func (s *stubAPI) SaveSetting(ctx context.Context, key string, value string, isPlatformSpecific bool) {
}

func (s *stubAPI) SetSetting(ctx context.Context, option plugin.SetSettingOption) plugin.SetSettingResult {
	return plugin.SetSettingResult{Success: true}
}

func (s *stubAPI) OnSettingChanged(ctx context.Context, callback func(context.Context, string, string)) {
}

func (s *stubAPI) OnDeepLink(ctx context.Context, callback func(context.Context, map[string]string)) {
}

func (s *stubAPI) OnUnload(ctx context.Context, callback func(context.Context)) {}

func (s *stubAPI) ShowToolbarMsg(ctx context.Context, msg plugin.ToolbarMsg) {}

func (s *stubAPI) ClearToolbarMsg(ctx context.Context, toolbarMsgId string) {}

func (s *stubAPI) OnEnterPluginQuery(ctx context.Context, callback func(context.Context)) {}

func (s *stubAPI) OnLeavePluginQuery(ctx context.Context, callback func(context.Context)) {}

func (s *stubAPI) RegisterQueryCommands(ctx context.Context, commands []plugin.MetadataCommand) {}

func (s *stubAPI) AIChatStream(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions, callback common.ChatStreamFunc) error {
	return nil
}

func (s *stubAPI) OnMRURestore(ctx context.Context, callback func(context.Context, plugin.MRUData) (*plugin.QueryResult, error)) {
}

func (s *stubAPI) OnHandlePluginCommand(ctx context.Context, handler plugin.PluginCommandHandler) {}

func (s *stubAPI) InvokePluginCommand(ctx context.Context, request plugin.PluginCommandRequest) (plugin.PluginCommandResult, error) {
	return plugin.PluginCommandResult{}, nil
}

func (s *stubAPI) UpdateResult(ctx context.Context, result plugin.UpdatableResult) bool {
	return false
}

func (s *stubAPI) PushResults(ctx context.Context, query plugin.Query, results []plugin.QueryResult) bool {
	return false
}

func (s *stubAPI) GetUpdatableResult(ctx context.Context, resultId string) *plugin.UpdatableResult {
	return nil
}

func (s *stubAPI) IsVisible(ctx context.Context) bool {
	return false
}

func (s *stubAPI) RefreshQuery(ctx context.Context, params plugin.RefreshQueryParam) {}

func (s *stubAPI) RefreshGlance(ctx context.Context, ids []string) {}

func (s *stubAPI) Copy(ctx context.Context, params plugin.CopyParams) {}

func (s *stubAPI) Screenshot(ctx context.Context, option plugin.ScreenshotOption) plugin.ScreenshotResult {
	return plugin.ScreenshotResult{}
}

func (s *stubAPI) GetCacheFolder(ctx context.Context) string {
	return ""
}
