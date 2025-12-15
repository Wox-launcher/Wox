package test

import (
	"path/filepath"
	"testing"
	"wox/database"
	"wox/setting"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPluginSettingStore_DeleteAll(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "plugin_setting_test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get underlying sql db: %v", err)
	}
	defer sqlDB.Close()

	if err := db.AutoMigrate(&database.PluginSetting{}); err != nil {
		t.Fatalf("failed to migrate schema: %v", err)
	}

	storeA := setting.NewPluginSettingStore(db, "pluginA")
	storeB := setting.NewPluginSettingStore(db, "pluginB")

	if err := storeA.Set("k1", "v1"); err != nil {
		t.Fatalf("failed to set pluginA k1: %v", err)
	}
	if err := storeA.Set("k2", "v2"); err != nil {
		t.Fatalf("failed to set pluginA k2: %v", err)
	}
	if err := storeB.Set("k1", "v1"); err != nil {
		t.Fatalf("failed to set pluginB k1: %v", err)
	}

	if err := storeA.DeleteAll(); err != nil {
		t.Fatalf("failed to delete pluginA settings: %v", err)
	}

	var countA int64
	if err := db.Model(&database.PluginSetting{}).Where("plugin_id = ?", "pluginA").Count(&countA).Error; err != nil {
		t.Fatalf("failed to count pluginA settings: %v", err)
	}
	if countA != 0 {
		t.Fatalf("expected pluginA settings deleted, got %d rows", countA)
	}

	var countB int64
	if err := db.Model(&database.PluginSetting{}).Where("plugin_id = ?", "pluginB").Count(&countB).Error; err != nil {
		t.Fatalf("failed to count pluginB settings: %v", err)
	}
	if countB != 1 {
		t.Fatalf("expected pluginB settings preserved, got %d rows", countB)
	}
}
