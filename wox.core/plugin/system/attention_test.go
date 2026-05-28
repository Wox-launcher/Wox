package system

import (
	"context"
	"testing"
	"wox/common"
	"wox/database"
	"wox/plugin"
	"wox/setting/definition"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type attentionActionTestAPI struct {
	changedQuery string
}

func (a *attentionActionTestAPI) ChangeQuery(ctx context.Context, query common.PlainQuery) {
	a.changedQuery = query.QueryText
}

func (a *attentionActionTestAPI) HideApp(ctx context.Context)                    {}
func (a *attentionActionTestAPI) ShowApp(ctx context.Context)                    {}
func (a *attentionActionTestAPI) Notify(ctx context.Context, description string) {}
func (a *attentionActionTestAPI) PushAttention(ctx context.Context, request plugin.PushAttentionRequest) {
}
func (a *attentionActionTestAPI) Log(ctx context.Context, level plugin.LogLevel, msg string) {}
func (a *attentionActionTestAPI) GetTranslation(ctx context.Context, key string) string {
	return key
}
func (a *attentionActionTestAPI) GetSetting(ctx context.Context, key string) string {
	return ""
}
func (a *attentionActionTestAPI) SaveSetting(ctx context.Context, key string, value string, isPlatformSpecific bool) {
}
func (a *attentionActionTestAPI) OnSettingChanged(ctx context.Context, callback func(ctx context.Context, key string, value string)) {
}
func (a *attentionActionTestAPI) OnGetDynamicSetting(ctx context.Context, callback func(ctx context.Context, key string) definition.PluginSettingDefinitionItem) {
}
func (a *attentionActionTestAPI) OnDeepLink(ctx context.Context, callback func(ctx context.Context, arguments map[string]string)) {
}
func (a *attentionActionTestAPI) OnUnload(ctx context.Context, callback func(ctx context.Context)) {
}
func (a *attentionActionTestAPI) OnMRURestore(ctx context.Context, callback func(ctx context.Context, mruData plugin.MRUData) (*plugin.QueryResult, error)) {
}
func (a *attentionActionTestAPI) ShowToolbarMsg(ctx context.Context, msg plugin.ToolbarMsg) {}
func (a *attentionActionTestAPI) ClearToolbarMsg(ctx context.Context, toolbarMsgId string)  {}
func (a *attentionActionTestAPI) OnEnterPluginQuery(ctx context.Context, callback func(ctx context.Context)) {
}
func (a *attentionActionTestAPI) OnLeavePluginQuery(ctx context.Context, callback func(ctx context.Context)) {
}
func (a *attentionActionTestAPI) RegisterQueryCommands(ctx context.Context, commands []plugin.MetadataCommand) {
}
func (a *attentionActionTestAPI) AIChatStream(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions, callback common.ChatStreamFunc) error {
	return nil
}
func (a *attentionActionTestAPI) GetUpdatableResult(ctx context.Context, resultId string) *plugin.UpdatableResult {
	return nil
}
func (a *attentionActionTestAPI) UpdateResult(ctx context.Context, result plugin.UpdatableResult) bool {
	return false
}
func (a *attentionActionTestAPI) PushResults(ctx context.Context, query plugin.Query, results []plugin.QueryResult) bool {
	return false
}
func (a *attentionActionTestAPI) IsVisible(ctx context.Context) bool {
	return false
}
func (a *attentionActionTestAPI) RefreshQuery(ctx context.Context, param plugin.RefreshQueryParam) {
}
func (a *attentionActionTestAPI) RefreshGlance(ctx context.Context, ids []string) {
}
func (a *attentionActionTestAPI) Copy(ctx context.Context, params plugin.CopyParams) {
}
func (a *attentionActionTestAPI) Screenshot(ctx context.Context, option plugin.ScreenshotOption) plugin.ScreenshotResult {
	return plugin.ScreenshotResult{}
}

func newSystemAttentionTestManager(t *testing.T) *plugin.AttentionManager {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&database.AttentionItem{}); err != nil {
		t.Fatalf("migrate attention item: %v", err)
	}

	return plugin.NewAttentionManager(db)
}

func TestAttentionActionMarksItemRead(t *testing.T) {
	ctx := context.Background()
	manager := newSystemAttentionTestManager(t)

	_, err := manager.Push(ctx, plugin.AttentionPluginSource{
		PluginID:    "github",
		DefaultIcon: common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1"><rect width="1" height="1"/></svg>`),
	}, plugin.PushAttentionRequest{
		Key:         "notifications",
		Title:       "3 unread notifications",
		Description: "Review pending GitHub notifications",
		Action: &plugin.AttentionAction{
			Type:  plugin.AttentionActionTypeChangeQuery,
			Query: "gh notifications",
		},
	})
	if err != nil {
		t.Fatalf("push item: %v", err)
	}

	api := &attentionActionTestAPI{}
	attentionPlugin := &AttentionPlugin{api: api, manager: manager}
	response := attentionPlugin.Query(ctx, plugin.Query{Type: plugin.QueryTypeInput})
	if len(response.Results) != 1 {
		t.Fatalf("expected one attention result, got %d", len(response.Results))
	}
	if len(response.Results[0].Actions) == 0 {
		t.Fatalf("expected attention result action")
	}

	response.Results[0].Actions[0].Action(ctx, plugin.ActionContext{})

	items, err := manager.List(ctx)
	if err != nil {
		t.Fatalf("list items: %v", err)
	}
	if len(items.Unread) != 0 || len(items.Read) != 1 {
		t.Fatalf("expected action to mark item read, got unread=%d read=%d", len(items.Unread), len(items.Read))
	}
	if api.changedQuery != "gh notifications" {
		t.Fatalf("expected action to change query, got %q", api.changedQuery)
	}
}
