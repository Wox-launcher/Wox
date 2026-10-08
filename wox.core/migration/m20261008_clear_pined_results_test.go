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

func TestClearPinedResultsMigrationDropsLegacyPins(t *testing.T) {
	db := openClearPinedResultsMigrationDB(t)
	legacy := `{"abc":true,"def":true}`
	if err := db.Create(&database.WoxSetting{Key: pinedResultsSettingKey, Value: legacy}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&database.Oplog{
		EntityType: cloudsync.EntityWoxSetting,
		EntityID:   pinedResultsSettingKey,
		Operation:  cloudsync.OpUpsert,
		Key:        pinedResultsSettingKey,
		Value:      legacy,
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

	migration := &clearPinedResultsMigration{}
	for range 2 {
		if err := migration.Up(context.Background(), db); err != nil {
			t.Fatal(err)
		}
	}

	var row database.WoxSetting
	if err := db.Where("key = ?", pinedResultsSettingKey).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Value != "{}" {
		t.Fatalf("pined results = %q, want {}", row.Value)
	}

	var ops []database.Oplog
	if err := db.Where("key = ?", pinedResultsSettingKey).Find(&ops).Error; err != nil {
		t.Fatal(err)
	}
	if len(ops) != 2 {
		t.Fatalf("pined result oplogs = %d, want the discarded legacy upload and one empty replacement", len(ops))
	}
	discardedLegacy := false
	emptyReplacement := false
	for _, op := range ops {
		if op.Value == legacy && op.CloudSyncDiscarded {
			discardedLegacy = true
		}
		if op.Value == "{}" && !op.CloudSyncDiscarded && !op.SyncedToCloud {
			emptyReplacement = true
		}
	}
	if !discardedLegacy || !emptyReplacement {
		t.Fatalf("oplogs = %+v", ops)
	}

	var theme database.Oplog
	if err := db.Where("key = ?", "ThemeId").First(&theme).Error; err != nil {
		t.Fatal(err)
	}
	if theme.CloudSyncDiscarded {
		t.Fatal("theme oplog was discarded")
	}
}

func TestClearPinedResultsMigrationKeepsQueryScopedPins(t *testing.T) {
	db := openClearPinedResultsMigrationDB(t)
	current := `{"abc":{"Queries":["b r"]}}`
	if err := db.Create(&database.WoxSetting{Key: pinedResultsSettingKey, Value: current}).Error; err != nil {
		t.Fatal(err)
	}

	if err := (&clearPinedResultsMigration{}).Up(context.Background(), db); err != nil {
		t.Fatal(err)
	}

	var row database.WoxSetting
	if err := db.Where("key = ?", pinedResultsSettingKey).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Value != current {
		t.Fatalf("query-scoped pins overwritten: %q", row.Value)
	}
}

func TestClearPinedResultsMigrationIgnoresMissingRow(t *testing.T) {
	db := openClearPinedResultsMigrationDB(t)
	if err := (&clearPinedResultsMigration{}).Up(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	var row database.WoxSetting
	if err := db.Where("key = ?", pinedResultsSettingKey).First(&row).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("missing pins were created: %#v err=%v", row, err)
	}
}

func openClearPinedResultsMigrationDB(t *testing.T) *gorm.DB {
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
