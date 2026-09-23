package migration

import (
	"context"
	"errors"
	"testing"
	"wox/cloudsync"
	"wox/common"
	"wox/database"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSplitAIChatsMigrationSplitsArrayAndTombstonesLegacyKey(t *testing.T) {
	db := openSplitAIChatsMigrationDB(t)
	legacy := `[{"Id":"a","Title":"First","UpdatedAt":10,"Conversations":[{"Id":"c1","Role":"user","Text":"hi"}]},{"Id":"b","Title":"Second","UpdatedAt":20},{"Title":"no id"}]`
	if err := db.Create(&database.PluginSetting{PluginID: common.AIChatPluginID, Key: legacyAIChatsSettingKey, Value: legacy}).Error; err != nil {
		t.Fatal(err)
	}
	// A newer copy of chat "b" already arrived from another migrated device; it must win.
	newerB := `{"Id":"b","Title":"Second edited elsewhere","UpdatedAt":30}`
	if err := db.Create(&database.PluginSetting{PluginID: common.AIChatPluginID, Key: aiChatSettingKeyPrefix + "b", Value: newerB}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&database.Oplog{EntityType: cloudsync.EntityPluginSetting, EntityID: common.AIChatPluginID, Operation: cloudsync.OpUpsert, Key: legacyAIChatsSettingKey, Value: legacy}).Error; err != nil {
		t.Fatal(err)
	}

	migration := &splitAIChatsMigration{}
	for range 2 {
		if err := db.Transaction(func(tx *gorm.DB) error { return migration.Up(context.Background(), tx) }); err != nil {
			t.Fatal(err)
		}
	}

	var legacyRow database.PluginSetting
	if err := db.Where("plugin_id = ? AND key = ?", common.AIChatPluginID, legacyAIChatsSettingKey).First(&legacyRow).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("legacy key still present: %#v err=%v", legacyRow, err)
	}
	var rows []database.PluginSetting
	if err := db.Where("plugin_id = ?", common.AIChatPluginID).Order("key asc").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Key != aiChatSettingKeyPrefix+"a" || rows[1].Key != aiChatSettingKeyPrefix+"b" {
		t.Fatalf("rows = %#v, want chat:a and chat:b", rows)
	}
	if rows[1].Value != newerB {
		t.Fatalf("newer synced copy of chat b was overwritten: %s", rows[1].Value)
	}

	var ops []database.Oplog
	if err := db.Order("id asc").Find(&ops).Error; err != nil {
		t.Fatal(err)
	}
	if len(ops) != 3 {
		t.Fatalf("oplogs = %d, want discarded legacy upsert + chat:a upsert + legacy delete", len(ops))
	}
	if !ops[0].CloudSyncDiscarded {
		t.Fatal("pending legacy array upload must be discarded")
	}
	if ops[1].Operation != cloudsync.OpUpsert || ops[1].Key != aiChatSettingKeyPrefix+"a" {
		t.Fatalf("expected upsert for chat:a, got %+v", ops[1])
	}
	if ops[2].Operation != cloudsync.OpDelete || ops[2].Key != legacyAIChatsSettingKey {
		t.Fatalf("expected delete tombstone for ai_chats, got %+v", ops[2])
	}
}

func TestSplitAIChatsMigrationKeepsLocalOnlyChatsLocal(t *testing.T) {
	db := openSplitAIChatsMigrationDB(t)
	if err := db.Create(&database.PluginSetting{PluginID: common.AIChatPluginID, Key: legacyAIChatsSettingKey, Value: `[{"Id":"a","UpdatedAt":1}]`, IsLocal: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := (&splitAIChatsMigration{}).Up(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	var row database.PluginSetting
	if err := db.Where("plugin_id = ? AND key = ?", common.AIChatPluginID, aiChatSettingKeyPrefix+"a").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if !row.IsLocal {
		t.Fatal("local-only history must stay local after the split")
	}
	var count int64
	if err := db.Model(&database.Oplog{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("local-only history must not produce oplogs, got %d", count)
	}
}

func openSplitAIChatsMigrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&database.PluginSetting{}, &database.Oplog{}); err != nil {
		t.Fatal(err)
	}
	return db
}
