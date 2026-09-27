package migration

import (
	"context"
	"testing"
	"wox/database"

	"gorm.io/gorm"
)

func TestClipboardShowFavoritesMigration(t *testing.T) {
	for _, tc := range []struct {
		name       string
		existing   bool
		favorites  bool
		legacy     string
		local      bool
		want       string
		wantOps    int
		wantNeeded bool
	}{
		{name: "fresh install"},
		{name: "existing default", existing: true, want: "true", wantOps: 1, wantNeeded: true},
		{name: "existing favorites", favorites: true, want: "true", wantOps: 1, wantNeeded: true},
		{name: "old unchecked", legacy: "false", want: "true", wantOps: 2, wantNeeded: true},
		{name: "old checked", legacy: "true", want: "false", wantOps: 2, wantNeeded: true},
		{name: "local old checked", legacy: "true", local: true, want: "false", wantNeeded: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := openQuickJumpMigrationDB(t)
			if err := db.AutoMigrate(&database.WoxSetting{}); err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&database.WoxSetting{Key: "ThemeId", Value: `"theme"`}).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&database.PluginSetting{PluginID: fileSearchPluginID, Key: "roots", Value: "[]"}).Error; err != nil {
				t.Fatal(err)
			}
			if tc.existing {
				if err := db.Create(&database.WoxSetting{Key: "LangCode", Value: `"en_US"`}).Error; err != nil {
					t.Fatal(err)
				}
			}
			if tc.favorites {
				if err := db.Create(&database.PluginSetting{PluginID: clipboardPluginID, Key: "favorites", Value: "[]"}).Error; err != nil {
					t.Fatal(err)
				}
			}
			if tc.legacy != "" {
				if err := db.Create(&database.PluginSetting{PluginID: clipboardPluginID, Key: legacyHideFavoritesSettingKey, Value: tc.legacy, IsLocal: tc.local}).Error; err != nil {
					t.Fatal(err)
				}
			}
			migration := &clipboardShowFavoritesMigration{}
			needed, err := migration.IsNeeded(context.Background(), db)
			if err != nil || needed != tc.wantNeeded {
				t.Fatalf("IsNeeded = %v, %v; want %v", needed, err, tc.wantNeeded)
			}
			if !needed {
				return
			}
			for range 2 {
				if err := db.Transaction(func(tx *gorm.DB) error { return migration.Up(context.Background(), tx) }); err != nil {
					t.Fatal(err)
				}
			}
			var current database.PluginSetting
			if err := db.Where("plugin_id = ? AND key = ?", clipboardPluginID, showFavoritesByDefaultSettingKey).First(&current).Error; err != nil {
				t.Fatal(err)
			}
			if current.Value != tc.want || current.IsLocal != tc.local {
				t.Fatalf("new setting = %+v, want %q local=%v", current, tc.want, tc.local)
			}
			var oldCount int64
			if err := db.Model(&database.PluginSetting{}).Where("plugin_id = ? AND key = ?", clipboardPluginID, legacyHideFavoritesSettingKey).Count(&oldCount).Error; err != nil || oldCount != 0 {
				t.Fatalf("old setting count = %d, %v", oldCount, err)
			}
			var ops []database.Oplog
			if err := db.Find(&ops).Error; err != nil || len(ops) != tc.wantOps {
				t.Fatalf("oplogs = %v, %v; want %d", ops, err, tc.wantOps)
			}
		})
	}
}
