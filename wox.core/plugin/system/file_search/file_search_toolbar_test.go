package system

import (
	"context"
	"strings"
	"testing"
	"wox/common"
	"wox/plugin"
	"wox/setting/definition"
	"wox/util/filesearch"
)

type fileSearchToolbarTestAPI struct{}

func (a fileSearchToolbarTestAPI) ChangeQuery(ctx context.Context, query common.PlainQuery) {}
func (a fileSearchToolbarTestAPI) HideApp(ctx context.Context)                              {}
func (a fileSearchToolbarTestAPI) ShowApp(ctx context.Context)                              {}
func (a fileSearchToolbarTestAPI) Notify(ctx context.Context, description string)           {}
func (a fileSearchToolbarTestAPI) PushAttention(ctx context.Context, request plugin.PushAttentionRequest) {
}
func (a fileSearchToolbarTestAPI) Log(ctx context.Context, level plugin.LogLevel, msg string) {
}
func (a fileSearchToolbarTestAPI) GetTranslation(ctx context.Context, key string) string {
	switch key {
	case "plugin_file_status_incremental_indexing_elapsed":
		return "Incremental indexing %s %s"
	case "plugin_file_status_index_duration_seconds":
		return "%ds"
	default:
		return key
	}
}
func (a fileSearchToolbarTestAPI) GetSetting(ctx context.Context, key string) string { return "" }
func (a fileSearchToolbarTestAPI) SaveSetting(ctx context.Context, key string, value string, isPlatformSpecific bool) {
}
func (a fileSearchToolbarTestAPI) OnSettingChanged(ctx context.Context, callback func(ctx context.Context, key string, value string)) {
}
func (a fileSearchToolbarTestAPI) OnGetDynamicSetting(ctx context.Context, callback func(ctx context.Context, key string) definition.PluginSettingDefinitionItem) {
}
func (a fileSearchToolbarTestAPI) OnDeepLink(ctx context.Context, callback func(ctx context.Context, arguments map[string]string)) {
}
func (a fileSearchToolbarTestAPI) OnUnload(ctx context.Context, callback func(ctx context.Context)) {
}
func (a fileSearchToolbarTestAPI) OnMRURestore(ctx context.Context, callback func(ctx context.Context, mruData plugin.MRUData) (*plugin.QueryResult, error)) {
}
func (a fileSearchToolbarTestAPI) ShowToolbarMsg(ctx context.Context, msg plugin.ToolbarMsg) {}
func (a fileSearchToolbarTestAPI) ClearToolbarMsg(ctx context.Context, toolbarMsgId string)  {}
func (a fileSearchToolbarTestAPI) OnEnterPluginQuery(ctx context.Context, callback func(ctx context.Context)) {
}
func (a fileSearchToolbarTestAPI) OnLeavePluginQuery(ctx context.Context, callback func(ctx context.Context)) {
}
func (a fileSearchToolbarTestAPI) RegisterQueryCommands(ctx context.Context, commands []plugin.MetadataCommand) {
}
func (a fileSearchToolbarTestAPI) AIChatStream(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions, callback common.ChatStreamFunc) error {
	return nil
}
func (a fileSearchToolbarTestAPI) GetUpdatableResult(ctx context.Context, resultId string) *plugin.UpdatableResult {
	return nil
}
func (a fileSearchToolbarTestAPI) UpdateResult(ctx context.Context, result plugin.UpdatableResult) bool {
	return false
}
func (a fileSearchToolbarTestAPI) PushResults(ctx context.Context, query plugin.Query, results []plugin.QueryResult) bool {
	return false
}
func (a fileSearchToolbarTestAPI) IsVisible(ctx context.Context) bool { return false }
func (a fileSearchToolbarTestAPI) RefreshQuery(ctx context.Context, param plugin.RefreshQueryParam) {
}
func (a fileSearchToolbarTestAPI) RefreshGlance(ctx context.Context, ids []string) {}
func (a fileSearchToolbarTestAPI) Copy(ctx context.Context, params plugin.CopyParams) {
}
func (a fileSearchToolbarTestAPI) Screenshot(ctx context.Context, option plugin.ScreenshotOption) plugin.ScreenshotResult {
	return plugin.ScreenshotResult{}
}

func TestIncrementalToolbarMessageWaitsForMinimumVisibleDuration(t *testing.T) {
	plugin := &FileSearchPlugin{api: fileSearchToolbarTestAPI{}}

	status := filesearch.StatusSnapshot{
		IsIndexing:         true,
		ActiveRunKind:      filesearch.RunKindIncremental,
		ActiveRunElapsedMs: 999,
		ActiveRunFileCount: 3,
	}
	if _, found := plugin.buildToolbarMsgFromStatus(context.Background(), status, false); found {
		t.Fatal("expected sub-second incremental indexing to stay silent")
	}

	status.ActiveRunElapsedMs = 1000
	msg, found := plugin.buildToolbarMsgFromStatus(context.Background(), status, false)
	if !found {
		t.Fatal("expected one-second incremental indexing to show toolbar status")
	}
	if !msg.Indeterminate {
		t.Fatal("expected incremental indexing toolbar status to keep spinner")
	}
	if !strings.Contains(msg.Title, "Incremental indexing") {
		t.Fatalf("expected incremental indexing title, got %q", msg.Title)
	}
}
