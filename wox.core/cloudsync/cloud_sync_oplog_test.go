package cloudsync

import (
	"context"
	"testing"

	"wox/database"
	"wox/util"
)

// TestPendingOplogsUploadLatestState covers identity, timing and retirement across small batches.
func TestPendingOplogsUploadLatestState(t *testing.T) {
	initCloudSyncTestDatabase(t)
	ctx := context.Background()
	rows := []database.Oplog{
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "watchlist", Operation: OpUpsert, Timestamp: 10, Value: "old"},
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "watchlist", Operation: OpUpsert, Timestamp: 20, Value: "new"},
		{EntityType: EntityPluginSetting, EntityID: "other", Key: "watchlist", Operation: OpUpsert, Timestamp: 10},
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "deleted", Operation: OpUpsert, Timestamp: 10},
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "deleted", Operation: OpDelete, Timestamp: 20},
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "recreated", Operation: OpDelete, Timestamp: 10},
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "recreated", Operation: OpUpsert, Timestamp: 20},
		{EntityType: EntityWoxSetting, EntityID: "stock", Key: "watchlist", Operation: OpUpsert, Timestamp: 10},
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "delayed", Operation: OpUpsert, Timestamp: 10},
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "delayed", Operation: OpUpsert, Timestamp: 20, SyncAfter: util.GetSystemTimestamp() + 60000},
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "tie", Operation: OpUpsert, Timestamp: 20, Value: "old"},
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "tie", Operation: OpUpsert, Timestamp: 20, Value: "new"},
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "clock", Operation: OpUpsert, Timestamp: 30},
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "clock", Operation: OpUpsert, Timestamp: 20},
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "discarded", Operation: OpUpsert, Timestamp: 10},
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "discarded", Operation: OpUpsert, Timestamp: 20, CloudSyncDiscarded: true},
	}
	if err := database.GetDB().Create(&rows).Error; err != nil {
		t.Fatal(err)
	}

	store := NewDefaultOplogStore()
	if count, err := store.CountPending(ctx); err != nil || count != 7 {
		t.Fatalf("pending count = %d, err = %v, want 7", count, err)
	}
	for _, index := range []int{1, 2, 4, 6, 7, 11, 12} {
		pending, err := store.LoadPending(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(pending) != 1 || pending[0].ID != rows[index].ID {
			t.Fatalf("pending = %+v, want row %+v", pending, rows[index])
		}
		if err := store.MarkSynced(ctx, []uint{pending[0].ID}); err != nil {
			t.Fatal(err)
		}
	}
	if count, err := store.CountPending(ctx); err != nil || count != 0 {
		t.Fatalf("remaining count = %d, err = %v", count, err)
	}
	if pending, err := store.LoadPending(ctx, 0); err != nil || len(pending) != 0 {
		t.Fatalf("retired values resurfaced: %+v, err = %v", pending, err)
	}
	if err := database.GetDB().Model(&database.Oplog{}).Where("id = ?", rows[9].ID).Update("sync_after", 0).Error; err != nil {
		t.Fatal(err)
	}
	if pending, err := store.LoadPending(ctx, 0); err != nil || len(pending) != 1 || pending[0].ID != rows[9].ID {
		t.Fatalf("due delayed state = %+v, err = %v", pending, err)
	}
}
