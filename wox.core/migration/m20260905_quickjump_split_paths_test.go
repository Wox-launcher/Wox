package migration

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"wox/database"
	"wox/setting"
	"wox/util"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestQuickJumpSplitPathsMovesCurrentPlatformAndKeepsOtherOS(t *testing.T) {
	db := openQuickJumpMigrationDB(t)
	macPath := `/Users/qianlifeng/Projects`
	winPath := `C:\Users\qianlifeng\Projects`
	if err := db.Save(&database.PluginSetting{
		PluginID: quickJumpPluginID,
		Key:      quickJumpPathsSettingKey,
		Value:    `[{"Path":"/Users/qianlifeng/Projects"},{"Path":"C:\\Users\\qianlifeng\\Projects"}]`,
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := (&quickjumpSplitPathsMigration{}).Up(context.Background(), db); err != nil {
		t.Fatal(err)
	}

	platformKey := setting.PlatformSettingKey(quickJumpPathsSettingKey, util.GetCurrentPlatform())
	platform := mustLoadQuickJumpSetting(t, db, platformKey)
	global := loadQuickJumpSetting(t, db, quickJumpPathsSettingKey)
	if util.IsWindows() {
		assertQuickJumpPaths(t, platform, filepath.Clean(winPath))
		assertQuickJumpPaths(t, global, macPath)
		return
	}
	assertQuickJumpPaths(t, platform, filepath.Clean(macPath))
	assertQuickJumpPaths(t, global, winPath)
}

func TestQuickJumpSplitPathsKeepsForeignGlobalWhenCurrentPlatformEmpty(t *testing.T) {
	db := openQuickJumpMigrationDB(t)
	foreign := `/Users/qianlifeng/Projects`
	if !util.IsWindows() {
		foreign = `C:\Users\qianlifeng\Projects`
	}
	payload, err := marshalQuickJumpPaths([]string{foreign})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Save(&database.PluginSetting{
		PluginID: quickJumpPluginID,
		Key:      quickJumpPathsSettingKey,
		Value:    payload,
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := (&quickjumpSplitPathsMigration{}).Up(context.Background(), db); err != nil {
		t.Fatal(err)
	}

	platformKey := setting.PlatformSettingKey(quickJumpPathsSettingKey, util.GetCurrentPlatform())
	assertQuickJumpPaths(t, mustLoadQuickJumpSetting(t, db, platformKey))
	assertQuickJumpPaths(t, mustLoadQuickJumpSetting(t, db, quickJumpPathsSettingKey), foreign)
}

func TestQuickJumpSplitPathsDeletesGlobalWhenOnlyCurrentPlatformRemains(t *testing.T) {
	db := openQuickJumpMigrationDB(t)
	current := `/Users/qianlifeng/Projects`
	if util.IsWindows() {
		current = `C:\Users\qianlifeng\Projects`
	}
	payload, err := marshalQuickJumpPaths([]string{current})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Save(&database.PluginSetting{
		PluginID: quickJumpPluginID,
		Key:      quickJumpPathsSettingKey,
		Value:    payload,
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := (&quickjumpSplitPathsMigration{}).Up(context.Background(), db); err != nil {
		t.Fatal(err)
	}

	platformKey := setting.PlatformSettingKey(quickJumpPathsSettingKey, util.GetCurrentPlatform())
	assertQuickJumpPaths(t, mustLoadQuickJumpSetting(t, db, platformKey), filepath.Clean(current))
	if row := loadQuickJumpSetting(t, db, quickJumpPathsSettingKey); row != nil {
		t.Fatalf("expected leftover global key to be deleted, got %q", row.Value)
	}
}

func TestQuickJumpSplitPathsRewritesMixedPlatformKey(t *testing.T) {
	db := openQuickJumpMigrationDB(t)
	foreign := `/Users/qianlifeng/Projects`
	if !util.IsWindows() {
		foreign = `C:\Users\qianlifeng\Projects`
	}
	payload, err := marshalQuickJumpPaths([]string{foreign})
	if err != nil {
		t.Fatal(err)
	}
	platformKey := setting.PlatformSettingKey(quickJumpPathsSettingKey, util.GetCurrentPlatform())
	if err := db.Save(&database.PluginSetting{
		PluginID: quickJumpPluginID,
		Key:      platformKey,
		Value:    payload,
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := (&quickjumpSplitPathsMigration{}).Up(context.Background(), db); err != nil {
		t.Fatal(err)
	}

	assertQuickJumpPaths(t, mustLoadQuickJumpSetting(t, db, platformKey))
	assertQuickJumpPaths(t, mustLoadQuickJumpSetting(t, db, quickJumpPathsSettingKey), foreign)
}

func TestQuickJumpSplitPathsNoopsWhenMissing(t *testing.T) {
	db := openQuickJumpMigrationDB(t)
	if err := (&quickjumpSplitPathsMigration{}).Up(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if row := loadQuickJumpSetting(t, db, quickJumpPathsSettingKey); row != nil {
		t.Fatalf("unexpected global row %q", row.Value)
	}
}

func openQuickJumpMigrationDB(t *testing.T) *gorm.DB {
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

func mustLoadQuickJumpSetting(t *testing.T, db *gorm.DB, key string) *database.PluginSetting {
	t.Helper()
	row := loadQuickJumpSetting(t, db, key)
	if row == nil {
		t.Fatalf("missing setting %q", key)
	}
	return row
}

func loadQuickJumpSetting(t *testing.T, db *gorm.DB, key string) *database.PluginSetting {
	t.Helper()
	row, err := loadQuickJumpPluginSetting(db, key)
	if err != nil {
		t.Fatal(err)
	}
	return row
}

func assertQuickJumpPaths(t *testing.T, row *database.PluginSetting, want ...string) {
	t.Helper()
	if row == nil {
		if len(want) == 0 {
			return
		}
		t.Fatal("missing setting row")
	}
	var entries []quickJumpPathSetting
	if err := json.Unmarshal([]byte(row.Value), &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(want) {
		t.Fatalf("paths = %s, want %v", row.Value, want)
	}
	for i, path := range want {
		if entries[i].Path != path {
			t.Fatalf("path[%d] = %q, want %q", i, entries[i].Path, path)
		}
	}
}
