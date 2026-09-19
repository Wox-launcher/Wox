package migration

import (
	"context"
	"errors"
	"testing"
	"wox/cloudsync"
	"wox/database"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestRenameQueryShortcutsMigrationMovesSettingAndPendingOplog(t *testing.T) {
	db := openRenameQueryShortcutsMigrationDB(t)
	value := `[{"Shortcut":"g","Query":"google {0}"}]`
	if err := db.Create(&database.WoxSetting{Key: legacyQueryShortcutsSettingKey, Value: value}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&database.Oplog{
		EntityType: cloudsync.EntityWoxSetting,
		EntityID:   legacyQueryShortcutsSettingKey,
		Operation:  cloudsync.OpUpsert,
		Key:        legacyQueryShortcutsSettingKey,
		Value:      value,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&database.Oplog{
		EntityType: cloudsync.EntityWoxSetting,
		EntityID:   "ThemeId",
		Operation:  cloudsync.OpUpsert,
		Key:        "ThemeId",
		Value:      `"theme"`,
	}).Error; err != nil {
		t.Fatal(err)
	}

	migration := &renameQueryShortcutsMigration{}
	for range 2 {
		if err := db.Transaction(func(tx *gorm.DB) error { return migration.Up(context.Background(), tx) }); err != nil {
			t.Fatal(err)
		}
	}

	var legacy database.WoxSetting
	if err := db.Where("key = ?", legacyQueryShortcutsSettingKey).First(&legacy).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("legacy key still present: %#v err=%v", legacy, err)
	}
	var current database.WoxSetting
	if err := db.Where("key = ?", queryAliasesSettingKey).First(&current).Error; err != nil {
		t.Fatal(err)
	}
	if current.Value != value {
		t.Fatalf("migrated value = %q, want %q", current.Value, value)
	}

	var ops []database.Oplog
	if err := db.Find(&ops).Error; err != nil {
		t.Fatal(err)
	}
	if len(ops) != 2 {
		t.Fatalf("oplogs = %d, want 2", len(ops))
	}
	renamed := false
	for _, op := range ops {
		if op.Key == "ThemeId" {
			continue
		}
		if op.Key != queryAliasesSettingKey || op.EntityID != queryAliasesSettingKey {
			t.Fatalf("oplog was not renamed: %+v", op)
		}
		renamed = true
	}
	if !renamed {
		t.Fatal("query alias oplog was dropped")
	}
}

func TestRenameQueryShortcutsMigrationKeepsExistingQueryAliases(t *testing.T) {
	db := openRenameQueryShortcutsMigrationDB(t)
	if err := db.Create(&database.WoxSetting{Key: legacyQueryShortcutsSettingKey, Value: `[{"Shortcut":"old","Query":"legacy"}]`}).Error; err != nil {
		t.Fatal(err)
	}
	current := `[{"Shortcut":"new","Query":"current"}]`
	if err := db.Create(&database.WoxSetting{Key: queryAliasesSettingKey, Value: current}).Error; err != nil {
		t.Fatal(err)
	}

	if err := (&renameQueryShortcutsMigration{}).Up(context.Background(), db); err != nil {
		t.Fatal(err)
	}

	var legacy database.WoxSetting
	if err := db.Where("key = ?", legacyQueryShortcutsSettingKey).First(&legacy).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("legacy key should be removed when QueryAliases already exists")
	}
	var row database.WoxSetting
	if err := db.Where("key = ?", queryAliasesSettingKey).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Value != current {
		t.Fatalf("existing QueryAliases overwritten: %q", row.Value)
	}
}

func openRenameQueryShortcutsMigrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&database.WoxSetting{}, &database.Oplog{}); err != nil {
		t.Fatal(err)
	}
	return db
}
