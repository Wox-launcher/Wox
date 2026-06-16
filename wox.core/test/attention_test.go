package test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
	"wox/common"
	"wox/database"
	"wox/plugin"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newAttentionTestManager(t *testing.T) *plugin.AttentionManager {
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

func testAttentionSource() plugin.AttentionPluginSource {
	return plugin.AttentionPluginSource{
		PluginID:    "github",
		DefaultIcon: common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1"><rect width="1" height="1"/></svg>`),
	}
}

func TestAttentionPushLifecycle(t *testing.T) {
	ctx := context.Background()
	manager := newAttentionTestManager(t)

	created, err := manager.Push(ctx, testAttentionSource(), plugin.PushAttentionRequest{
		Key:         "notifications",
		Title:       "3 unread notifications",
		Description: "Review pending GitHub notifications",
	})
	if err != nil {
		t.Fatalf("push new item: %v", err)
	}
	if created.IsRead {
		t.Fatalf("new attention item should be unread")
	}

	updated, err := manager.Push(ctx, testAttentionSource(), plugin.PushAttentionRequest{
		Key:         "notifications",
		Title:       "5 unread notifications",
		Description: "Review pending GitHub notifications",
	})
	if err != nil {
		t.Fatalf("push unread update: %v", err)
	}
	if updated.IdentityKey != created.IdentityKey {
		t.Fatalf("same plugin key should update existing item")
	}
	if updated.IsRead {
		t.Fatalf("unread item should stay unread when updated")
	}
	if updated.Title != "5 unread notifications" {
		t.Fatalf("unread update should replace title, got %q", updated.Title)
	}

	unreadCount, err := manager.UnreadCount(ctx)
	if err != nil {
		t.Fatalf("count unread: %v", err)
	}
	if unreadCount != 1 {
		t.Fatalf("same key updates should keep one unread item, got %d", unreadCount)
	}
}

func TestAttentionReadItemStaysReadWhenContentFingerprintUnchanged(t *testing.T) {
	ctx := context.Background()
	manager := newAttentionTestManager(t)

	item, err := manager.Push(ctx, testAttentionSource(), plugin.PushAttentionRequest{
		Key:         "notifications",
		Title:       "3 unread notifications",
		Description: "Review pending GitHub notifications",
	})
	if err != nil {
		t.Fatalf("push item: %v", err)
	}
	if err := manager.MarkRead(ctx, item.IdentityKey); err != nil {
		t.Fatalf("mark read: %v", err)
	}

	updated, err := manager.Push(ctx, testAttentionSource(), plugin.PushAttentionRequest{
		Key:         "notifications",
		Title:       "3 unread notifications",
		Description: "Review pending GitHub notifications",
		Action: &plugin.AttentionAction{
			Type:  plugin.AttentionActionTypeChangeQuery,
			Query: "gh notifications",
		},
	})
	if err != nil {
		t.Fatalf("push unchanged read item: %v", err)
	}
	if !updated.IsRead {
		t.Fatalf("read item should stay read when title and description are unchanged")
	}
}

func TestAttentionReadItemBecomesUnreadWhenContentFingerprintChanges(t *testing.T) {
	ctx := context.Background()
	manager := newAttentionTestManager(t)

	item, err := manager.Push(ctx, testAttentionSource(), plugin.PushAttentionRequest{
		Key:         "notifications",
		Title:       "3 unread notifications",
		Description: "Review pending GitHub notifications",
	})
	if err != nil {
		t.Fatalf("push item: %v", err)
	}
	if err := manager.MarkRead(ctx, item.IdentityKey); err != nil {
		t.Fatalf("mark read: %v", err)
	}

	updated, err := manager.Push(ctx, testAttentionSource(), plugin.PushAttentionRequest{
		Key:         "notifications",
		Title:       "3 new notifications",
		Description: "Review pending GitHub notifications",
	})
	if err != nil {
		t.Fatalf("push changed read item: %v", err)
	}
	if updated.IsRead {
		t.Fatalf("read item should become unread when title or description changes")
	}
}

func TestPushAttentionRequestUnmarshalsObjectParameter(t *testing.T) {
	raw := `{"key":"notifications","title":"3 unread","description":"Review GitHub","action":{"type":"change_query","query":"gh notifications"}}`

	var request plugin.PushAttentionRequest
	if err := json.Unmarshal([]byte(raw), &request); err != nil {
		t.Fatalf("unmarshal push attention request: %v", err)
	}

	if request.Key != "notifications" || request.Title != "3 unread" || request.Description != "Review GitHub" {
		t.Fatalf("unexpected request fields: %+v", request)
	}
	if request.Action == nil || request.Action.Type != plugin.AttentionActionTypeChangeQuery || request.Action.Query != "gh notifications" {
		t.Fatalf("unexpected action: %+v", request.Action)
	}
}

func TestAttentionPushRetriesWhenDatabaseIsTemporarilyLocked(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "wox.db")

	lockDB, err := gorm.Open(sqlite.Open(dbPath+"?_busy_timeout=1"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open lock db: %v", err)
	}
	lockSQLDB, err := lockDB.DB()
	if err != nil {
		t.Fatalf("get lock sql db: %v", err)
	}
	t.Cleanup(func() {
		_ = lockSQLDB.Close()
	})
	if err := lockDB.AutoMigrate(&database.AttentionItem{}); err != nil {
		t.Fatalf("migrate attention item: %v", err)
	}

	managerDB, err := gorm.Open(sqlite.Open(dbPath+"?_busy_timeout=1"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open manager db: %v", err)
	}
	managerSQLDB, err := managerDB.DB()
	if err != nil {
		t.Fatalf("get manager sql db: %v", err)
	}
	t.Cleanup(func() {
		_ = managerSQLDB.Close()
	})
	manager := plugin.NewAttentionManager(managerDB)

	tx := lockDB.Begin()
	if err := tx.Create(&database.AttentionItem{
		IdentityKey:        "lock:holder",
		PluginID:           "lock",
		Key:                "holder",
		Title:              "Lock holder",
		ContentFingerprint: "holder",
		CreatedTimestamp:   1,
		UpdatedTimestamp:   1,
	}).Error; err != nil {
		t.Fatalf("hold sqlite write lock: %v", err)
	}

	commitErr := make(chan error, 1)
	go func() {
		time.Sleep(100 * time.Millisecond)
		commitErr <- tx.Commit().Error
	}()

	item, pushErr := manager.Push(ctx, testAttentionSource(), plugin.PushAttentionRequest{
		Key:   "notifications",
		Title: "3 unread notifications",
	})
	if err := <-commitErr; err != nil {
		t.Fatalf("release sqlite write lock: %v", err)
	}
	if pushErr != nil {
		t.Fatalf("push should retry after a temporary sqlite lock: %v", pushErr)
	}
	if item.IdentityKey == "" {
		t.Fatalf("expected pushed attention item")
	}
}
