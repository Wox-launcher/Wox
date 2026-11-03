package system

import (
	"context"
	"testing"
	"wox/common"
	"wox/database"
	"wox/plugin"
	"wox/plugin/plugintest"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type attentionActionTestAPI struct {
	plugintest.StubAPI

	changedQuery string
}

func (a *attentionActionTestAPI) ChangeQuery(ctx context.Context, query common.PlainQuery) {
	a.changedQuery = query.QueryText
}

func (a *attentionActionTestAPI) GetTranslation(ctx context.Context, key string) string {
	return key
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

// TestAttentionSettingsActionIsDefault verifies the persisted action reaches the inbox as its primary action.
func TestAttentionSettingsActionIsDefault(t *testing.T) {
	ctx := context.Background()
	manager := newSystemAttentionTestManager(t)
	item, err := manager.Push(ctx, plugin.AttentionPluginSource{PluginID: "file-search"}, plugin.PushAttentionRequest{
		Key: "update", Title: "Update available",
		Action: &plugin.AttentionAction{Type: plugin.AttentionActionTypeOpenPluginSettings},
	})
	if err != nil {
		t.Fatal(err)
	}
	actions := (&AttentionPlugin{manager: manager}).buildItemActions(ctx, item)
	if len(actions) != 2 || actions[0].Id != attentionOpenActionID || !actions[0].IsDefault || actions[1].IsDefault {
		t.Fatalf("settings action must precede mark-read as the default: %+v", actions)
	}
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
	if response.Results[0].Actions[0].Id != attentionOpenActionID || response.Results[0].Actions[1].Id != attentionMarkReadActionID {
		t.Fatalf("attention action ids = %q, %q", response.Results[0].Actions[0].Id, response.Results[0].Actions[1].Id)
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

func TestAttentionActionMarksItemUnread(t *testing.T) {
	ctx := context.Background()
	manager := newSystemAttentionTestManager(t)

	item, err := manager.Push(ctx, plugin.AttentionPluginSource{
		PluginID:    "github",
		DefaultIcon: common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1"><rect width="1" height="1"/></svg>`),
	}, plugin.PushAttentionRequest{
		Key:   "notifications",
		Title: "1 unread notification",
		Action: &plugin.AttentionAction{
			Type:  plugin.AttentionActionTypeChangeQuery,
			Query: "gh notifications",
		},
	})
	if err != nil {
		t.Fatalf("push item: %v", err)
	}
	if err := manager.MarkRead(ctx, item.IdentityKey); err != nil {
		t.Fatalf("mark read: %v", err)
	}

	attentionPlugin := &AttentionPlugin{api: &attentionActionTestAPI{}, manager: manager}
	response := attentionPlugin.Query(ctx, plugin.Query{Type: plugin.QueryTypeInput})
	if len(response.Results) != 1 {
		t.Fatalf("expected one attention result, got %d", len(response.Results))
	}
	if len(response.Results[0].Actions) < 2 || response.Results[0].Actions[1].Id != attentionMarkUnreadActionID {
		t.Fatalf("read item actions = %#v, want mark unread", response.Results[0].Actions)
	}

	response.Results[0].Actions[1].Action(ctx, plugin.ActionContext{})

	items, err := manager.List(ctx)
	if err != nil {
		t.Fatalf("list items: %v", err)
	}
	if len(items.Unread) != 1 || len(items.Read) != 0 {
		t.Fatalf("expected action to mark item unread, got unread=%d read=%d", len(items.Unread), len(items.Read))
	}
}
