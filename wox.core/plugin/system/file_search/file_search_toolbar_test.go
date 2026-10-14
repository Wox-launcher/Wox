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

type fullProgressToolbarAPI struct {
	fileSearchToolbarTestAPI
	messages []plugin.ToolbarMsg
}

func (a *fullProgressToolbarAPI) ShowToolbarMsg(_ context.Context, msg plugin.ToolbarMsg) {
	a.messages = append(a.messages, msg)
}

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
	case "plugin_file_status_full_indexing_progress":
		return "Full indexing... %s files (%s/s) · %s"
	case "plugin_file_status_full_indexing_saving":
		return "Full indexing... %s files · %s · saving index"
	case "plugin_file_status_incremental_indexing_elapsed":
		return "Incremental indexing %s %s"
	case "plugin_file_status_index_duration_seconds":
		return "%ds"
	case "plugin_file_status_index_duration_minutes":
		return "%dm %02ds"
	case "plugin_file_status_index_duration_hours":
		return "%dh %02dm"
	case "plugin_file_setting_index_stats_unavailable":
		return "-"
	case "plugin_file_setting_index_stats_duration_ms":
		return "%dms"
	case "plugin_file_setting_index_stats_duration_seconds_ms":
		return "%ds %dms"
	default:
		return key
	}
}
func (a fileSearchToolbarTestAPI) GetSetting(ctx context.Context, key string) string { return "" }
func (a fileSearchToolbarTestAPI) SaveSetting(ctx context.Context, key string, value string, isPlatformSpecific bool) {
}
func (a fileSearchToolbarTestAPI) SetSetting(ctx context.Context, option plugin.SetSettingOption) plugin.SetSettingResult {
	return plugin.SetSettingResult{Success: true}
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
func (a fileSearchToolbarTestAPI) RegisterPluginTool(context.Context, plugin.RegisterPluginToolOption) plugin.RegisterPluginToolResult {
	return plugin.RegisterPluginToolResult{}
}
func (a fileSearchToolbarTestAPI) UnregisterPluginTool(context.Context, plugin.UnregisterPluginToolOption) plugin.UnregisterPluginToolResult {
	return plugin.UnregisterPluginToolResult{}
}
func (a fileSearchToolbarTestAPI) ListPluginTools(context.Context, plugin.ListPluginToolsOption) plugin.ListPluginToolsResult {
	return plugin.ListPluginToolsResult{}
}
func (a fileSearchToolbarTestAPI) InvokePluginTool(context.Context, plugin.InvokePluginToolOption) plugin.InvokePluginToolResult {
	return plugin.InvokePluginToolResult{}
}
func (a fileSearchToolbarTestAPI) ShowToolbarMsg(ctx context.Context, msg plugin.ToolbarMsg) {}
func (a fileSearchToolbarTestAPI) ClearToolbarMsg(ctx context.Context, toolbarMsgId string)  {}
func (a fileSearchToolbarTestAPI) OnEnterPluginQuery(ctx context.Context, callback func(ctx context.Context)) {
}
func (a fileSearchToolbarTestAPI) OnLeavePluginQuery(ctx context.Context, callback func(ctx context.Context)) {
}
func (a fileSearchToolbarTestAPI) OnDragOut(ctx context.Context, option plugin.DragOutListenOption) plugin.DragOutListenResult {
	return plugin.DragOutListenResult{}
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
func (a fileSearchToolbarTestAPI) GetCacheFolder(ctx context.Context) string { return "" }
func (a fileSearchToolbarTestAPI) GetThemeColors(ctx context.Context, option plugin.GetThemeColorsOption) plugin.GetThemeColorsResult {
	return plugin.GetThemeColorsResult{}
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

// TestFullIndexToolbarKeepsElapsedDuringSaving checks progress before the first
// file, during scanning, and while counts stop changing during publication.
func TestFullIndexToolbarKeepsElapsedDuringSaving(t *testing.T) {
	c := &FileSearchPlugin{api: fileSearchToolbarTestAPI{}}
	status := filesearch.StatusSnapshot{IsIndexing: true, ActiveRunKind: filesearch.RunKindFull, ActiveRunStatus: filesearch.RunStatusPlanning}
	msg, found := c.buildToolbarMsgFromStatus(context.Background(), status, false)
	if !found || !msg.Indeterminate || !strings.Contains(msg.Title, "0 files") || !strings.Contains(msg.Title, "0s") {
		t.Fatalf("initial progress: %+v", msg)
	}
	status.ActiveRunFileCount, status.ActiveRunElapsedMs = 12345, 5000
	status.ActiveRunStatus = filesearch.RunStatusExecuting
	scanning := c.buildIndexingToolbarTitle(context.Background(), status)
	if !strings.Contains(scanning, "12,345 files") || !strings.Contains(scanning, "5s") {
		t.Fatalf("scan progress: %s", scanning)
	}
	status.ActiveRunStatus = filesearch.RunStatusFinalizing
	saving := c.buildIndexingToolbarTitle(context.Background(), status)
	status.ActiveRunElapsedMs = 6000
	later := c.buildIndexingToolbarTitle(context.Background(), status)
	if !strings.Contains(saving, "saving index") || !strings.Contains(saving, "12,345 files") || !strings.Contains(later, "6s") || saving == later {
		t.Fatalf("saving progress froze: %q -> %q", saving, later)
	}
}

// TestNewFullIndexReplacesCompletionHold preserves the hold for incidental
// incremental work, while a deliberate full rebuild shows progress immediately.
func TestNewFullIndexReplacesCompletionHold(t *testing.T) {
	api := &fullProgressToolbarAPI{}
	c := &FileSearchPlugin{api: api}
	c.armCompletionToolbarHold()
	status := filesearch.StatusSnapshot{IsIndexing: true, ActiveRunKind: filesearch.RunKindIncremental, ActiveRunElapsedMs: 2000}
	c.syncToolbarMsgWithStatus(context.Background(), status, false)
	if len(api.messages) != 0 {
		t.Fatal("incidental incremental work replaced the completion summary")
	}
	status.ActiveRunKind = filesearch.RunKindFull
	c.syncToolbarMsgWithStatus(context.Background(), status, false)
	if len(api.messages) != 1 || c.shouldHoldCompletionToolbar() {
		t.Fatal("new full build remained hidden behind the previous completion hold")
	}
}

func TestFileSearchResultActionsIncludeNotesAndShell(t *testing.T) {
	plugin := &FileSearchPlugin{api: fileSearchToolbarTestAPI{}}
	actions := plugin.buildFileSearchResultActions(context.Background(), filesearch.SearchResult{
		Path:  `/Users/demo/main.go`,
		IsDir: false,
	}, false)

	want := map[string]bool{
		"i18n:plugin_file_copy_path":            false,
		"i18n:plugin_file_copy_name":            false,
		"i18n:plugin_file_execute_command_here": false,
		"i18n:plugin_notes_action_save":         false,
	}
	for _, action := range actions {
		if _, ok := want[action.Name]; ok {
			want[action.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Fatalf("missing file search action %s in %#v", name, actions)
		}
	}
}

func TestFileSearchCopyActionsWorkForFilesAndFolders(t *testing.T) {
	api := &fileSearchStatusCopyAPI{}
	fileSearchPlugin := &FileSearchPlugin{api: api}

	fileItem := filesearch.SearchResult{Path: `/Users/demo/main.go`, Name: "main.go", IsDir: false}
	folderItem := filesearch.SearchResult{Path: `C:\Users\qianl\Droppy`, Name: "Droppy", IsDir: true}

	assertFileSearchCopyAction(t, api, fileSearchPlugin.buildFileSearchResultActions(context.Background(), fileItem, false), "i18n:plugin_file_copy_path", fileItem.Path)
	assertFileSearchCopyAction(t, api, fileSearchPlugin.buildFileSearchResultActions(context.Background(), fileItem, false), "i18n:plugin_file_copy_name", "main.go")
	assertFileSearchCopyAction(t, api, fileSearchPlugin.buildFileSearchResultActions(context.Background(), folderItem, false), "i18n:plugin_file_copy_path", folderItem.Path)
	assertFileSearchCopyAction(t, api, fileSearchPlugin.buildFileSearchResultActions(context.Background(), folderItem, false), "i18n:plugin_file_copy_name", "Droppy")
}

func assertFileSearchCopyAction(t *testing.T, api *fileSearchStatusCopyAPI, actions []plugin.QueryResultAction, name, want string) {
	t.Helper()
	for _, action := range actions {
		if action.Name != name {
			continue
		}
		api.copiedText = ""
		action.Action(context.Background(), plugin.ActionContext{})
		if api.copiedText != want {
			t.Fatalf("%s copied %q, want %q", name, api.copiedText, want)
		}
		return
	}
	t.Fatalf("missing action %s in %#v", name, actions)
}

func (a fileSearchToolbarTestAPI) RegisterTriggerKeyword(context.Context, plugin.RegisterTriggerKeywordOption) plugin.RegisterTriggerKeywordResult {
	return plugin.RegisterTriggerKeywordResult{Success: true}
}
func (a fileSearchToolbarTestAPI) UnregisterTriggerKeyword(context.Context, plugin.UnregisterTriggerKeywordOption) plugin.UnregisterTriggerKeywordResult {
	return plugin.UnregisterTriggerKeywordResult{Success: true}
}
