package plugintest

import (
	"context"
	"wox/common"
	"wox/plugin"
	"wox/setting/definition"
)

// StubAPI supplies default plugin API behavior for tests.
type StubAPI struct{}

var _ plugin.API = StubAPI{}

func (e StubAPI) OnGetDynamicSetting(
	context.Context,
	func(context.Context, string) definition.PluginSettingDefinitionItem,
) {
}

func (e StubAPI) ChangeQuery(ctx context.Context, query common.PlainQuery) {
}

func (e StubAPI) HideApp(ctx context.Context) {
}

func (e StubAPI) ShowApp(ctx context.Context) {
}

func (e StubAPI) PushAttention(ctx context.Context, request plugin.PushAttentionRequest) {
}

func (e StubAPI) Log(ctx context.Context, level plugin.LogLevel, msg string) {
}

func (e StubAPI) GetTranslation(ctx context.Context, key string) string {
	return ""
}

func (e StubAPI) GetSetting(ctx context.Context, key string) string {
	return ""
}

func (e StubAPI) SaveSetting(ctx context.Context, key string, value string, isPlatformSpecific bool) {
}

func (e StubAPI) SetSetting(ctx context.Context, option plugin.SetSettingOption) plugin.SetSettingResult {
	return plugin.SetSettingResult{Success: true}
}

func (e StubAPI) OnSettingChanged(ctx context.Context, callback func(context.Context, string, string)) {
}

func (e StubAPI) OnDeepLink(ctx context.Context, callback func(context.Context, map[string]string)) {
}

func (e StubAPI) OnUnload(ctx context.Context, callback func(context.Context)) {
}

func (e StubAPI) ShowToolbarMsg(ctx context.Context, msg plugin.ToolbarMsg) {
}

func (e StubAPI) ClearToolbarMsg(ctx context.Context, toolbarMsgId string) {
}

func (e StubAPI) OnEnterPluginQuery(ctx context.Context, callback func(context.Context)) {
}

func (e StubAPI) OnLeavePluginQuery(ctx context.Context, callback func(context.Context)) {
}

func (e StubAPI) OnDragOut(ctx context.Context, option plugin.DragOutListenOption) plugin.DragOutListenResult {
	return plugin.DragOutListenResult{}
}

func (e StubAPI) RegisterQueryCommands(ctx context.Context, commands []plugin.MetadataCommand) {
}

func (e StubAPI) GetTriggerKeywords(context.Context, plugin.GetTriggerKeywordsOption) plugin.GetTriggerKeywordsResult {
	return plugin.GetTriggerKeywordsResult{TriggerKeywords: []string{}}
}

func (e StubAPI) GetPluginInfo(context.Context, plugin.GetPluginInfoOption) plugin.GetPluginInfoResult {
	return plugin.GetPluginInfoResult{TriggerKeywords: []string{}}
}

func (e StubAPI) AIChatStream(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions, callback common.ChatStreamFunc) error {
	return nil
}

func (e StubAPI) OnMRURestore(ctx context.Context, callback func(context.Context, plugin.MRUData) (*plugin.QueryResult, error)) {
}

func (e StubAPI) RegisterPluginTool(context.Context, plugin.RegisterPluginToolOption) plugin.RegisterPluginToolResult {
	return plugin.RegisterPluginToolResult{}
}
func (e StubAPI) UnregisterPluginTool(context.Context, plugin.UnregisterPluginToolOption) plugin.UnregisterPluginToolResult {
	return plugin.UnregisterPluginToolResult{}
}
func (e StubAPI) ListPluginTools(context.Context, plugin.ListPluginToolsOption) plugin.ListPluginToolsResult {
	return plugin.ListPluginToolsResult{}
}
func (e StubAPI) InvokePluginTool(context.Context, plugin.InvokePluginToolOption) plugin.InvokePluginToolResult {
	return plugin.InvokePluginToolResult{}
}

func (e StubAPI) UpdateResult(ctx context.Context, result plugin.UpdatableResult) bool {
	return false
}

func (e StubAPI) UpdatePluginPreview(context.Context, plugin.UpdatePluginPreviewOption) plugin.UpdatePluginPreviewResult {
	return plugin.UpdatePluginPreviewResult{}
}

func (e StubAPI) UpdatePluginPreviewData(context.Context, plugin.UpdatePluginPreviewDataOption) plugin.UpdatePluginPreviewDataResult {
	return plugin.UpdatePluginPreviewDataResult{}
}

func (e StubAPI) RemoveResult(ctx context.Context, option plugin.RemoveResultOption) plugin.RemoveResultResult {
	return plugin.RemoveResultResult{}
}

func (e StubAPI) PushResults(ctx context.Context, query plugin.Query, results []plugin.QueryResult) bool {
	return false
}

func (e StubAPI) GetUpdatableResult(ctx context.Context, resultId string) *plugin.UpdatableResult {
	return nil
}

func (e StubAPI) IsVisible(ctx context.Context) bool {
	return false
}

func (e StubAPI) RefreshQuery(ctx context.Context, params plugin.RefreshQueryParam) {
}

func (e StubAPI) RefreshGlance(ctx context.Context, ids []string) {
}

func (e StubAPI) Copy(ctx context.Context, params plugin.CopyParams) {
}

func (e StubAPI) Screenshot(ctx context.Context, option plugin.ScreenshotOption) plugin.ScreenshotResult {
	return plugin.ScreenshotResult{}
}

func (e StubAPI) GetCacheFolder(ctx context.Context) string {
	return ""
}

func (e StubAPI) GetThemeColors(ctx context.Context, option plugin.GetThemeColorsOption) plugin.GetThemeColorsResult {
	return plugin.GetThemeColorsResult{}
}

func (e StubAPI) GetOSPermissionStatus(ctx context.Context, option plugin.OSPermissionOption) plugin.OSPermissionResult {
	return plugin.OSPermissionResult{}
}

func (e StubAPI) RequestOSPermission(ctx context.Context, option plugin.OSPermissionOption) plugin.OSPermissionResult {
	return plugin.OSPermissionResult{}
}

func (e StubAPI) RegisterTriggerKeyword(context.Context, plugin.RegisterTriggerKeywordOption) plugin.RegisterTriggerKeywordResult {
	return plugin.RegisterTriggerKeywordResult{Success: true}
}
func (e StubAPI) UnregisterTriggerKeyword(context.Context, plugin.UnregisterTriggerKeywordOption) plugin.UnregisterTriggerKeywordResult {
	return plugin.UnregisterTriggerKeywordResult{Success: true}
}

func (api StubAPI) ShowToastMsg(ctx context.Context, option plugin.ShowToastMsgOption) plugin.ShowToastMsgResult {
	return plugin.ShowToastMsgResult{Success: true}
}

func (api StubAPI) CreateWindow(ctx context.Context, option plugin.CreateWindowOption) plugin.CreateWindowResult {
	return plugin.CreateWindowResult{}
}

func (e StubAPI) OnHotkey(ctx context.Context, option plugin.OnHotkeyOption) plugin.OnHotkeyResult {
	return plugin.OnHotkeyResult{Success: true}
}
