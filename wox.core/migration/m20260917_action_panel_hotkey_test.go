package migration

import (
	"context"
	"testing"
	"wox/cloudsync"
	"wox/database"
	"wox/setting"
	"wox/util"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestActionPanelHotkeyMigrationSkipsFreshInstall(t *testing.T) {
	db := openActionPanelHotkeyMigrationDB(t)
	if err := db.Save(&database.WoxSetting{Key: "ThemeId", Value: `"theme"`}).Error; err != nil {
		t.Fatal(err)
	}

	migration := &actionPanelHotkeyMigration{}
	needed, err := migration.IsNeeded(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if needed {
		t.Fatal("fresh installs should keep the new primary+K default")
	}
}

func TestActionPanelHotkeyMigrationPreservesExistingUsers(t *testing.T) {
	db := openActionPanelHotkeyMigrationDB(t)
	if err := db.Save(&database.WoxSetting{Key: setting.PlatformSettingKey("MainHotkey", util.GetCurrentPlatform()), Value: `"alt+space"`}).Error; err != nil {
		t.Fatal(err)
	}

	migration := &actionPanelHotkeyMigration{}
	needed, err := migration.IsNeeded(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if !needed {
		t.Fatal("existing users should receive the legacy primary+J shortcut")
	}
	for range 2 {
		if err := db.Transaction(func(tx *gorm.DB) error { return migration.Up(context.Background(), tx) }); err != nil {
			t.Fatal(err)
		}
	}

	for _, platform := range []string{util.PlatformWindows, util.PlatformMacOS, util.PlatformLinux} {
		key := setting.PlatformSettingKey("ActionPanelHotkey", platform)
		var row database.WoxSetting
		if err := db.Where("key = ?", key).First(&row).Error; err != nil {
			t.Fatalf("missing %s: %v", key, err)
		}
		want, err := setting.SerializeValue(setting.LegacyActionPanelHotkeyForPlatform(platform))
		if err != nil {
			t.Fatal(err)
		}
		if row.Value != want {
			t.Fatalf("%s = %q, want %q", key, row.Value, want)
		}
	}

	var ops []database.Oplog
	if err := db.Find(&ops).Error; err != nil {
		t.Fatal(err)
	}
	if len(ops) != 3 {
		t.Fatalf("oplogs = %d, want 3 platform upserts", len(ops))
	}
	for _, op := range ops {
		if op.EntityType != cloudsync.EntityWoxSetting || op.Operation != cloudsync.OpUpsert {
			t.Fatalf("unexpected oplog: %+v", op)
		}
	}

	needed, err = migration.IsNeeded(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if needed {
		t.Fatal("migration should be complete after writing the legacy shortcuts")
	}
}

func TestActionPanelHotkeyMigrationKeepsCustomValue(t *testing.T) {
	db := openActionPanelHotkeyMigrationDB(t)
	key := setting.PlatformSettingKey("ActionPanelHotkey", util.GetCurrentPlatform())
	if err := db.Save(&database.WoxSetting{Key: setting.PlatformSettingKey("MainHotkey", util.GetCurrentPlatform()), Value: `"alt+space"`}).Error; err != nil {
		t.Fatal(err)
	}
	custom, err := setting.SerializeValue("ctrl+m")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Save(&database.WoxSetting{Key: key, Value: custom}).Error; err != nil {
		t.Fatal(err)
	}

	if err := (&actionPanelHotkeyMigration{}).Up(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	var row database.WoxSetting
	if err := db.Where("key = ?", key).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Value != custom {
		t.Fatalf("custom hotkey overwritten: %q", row.Value)
	}
}

func openActionPanelHotkeyMigrationDB(t *testing.T) *gorm.DB {
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
