package migration

import (
	"context"
	"testing"
	"wox/cloudsync"
	"wox/database"
	"wox/setting"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestLocalLastWindowPositionMigration(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&database.WoxSetting{}, &database.Oplog{}); err != nil {
		t.Fatal(err)
	}
	for _, row := range []database.WoxSetting{{Key: "LastWindowX", Value: "-1920"}, {Key: "LastWindowY", Value: "40"}} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&database.Oplog{EntityType: cloudsync.EntityWoxSetting, EntityID: "LastWindowX", Key: "LastWindowX", Operation: cloudsync.OpUpsert}).Error; err != nil {
		t.Fatal(err)
	}
	m := &localLastWindowPositionMigration{}
	for range 2 {
		if err := db.Transaction(func(tx *gorm.DB) error { return m.Up(context.Background(), tx) }); err != nil {
			t.Fatal(err)
		}
	}
	store := setting.NewWoxSettingStore(db)
	var got setting.SavedWindowPosition
	if err := store.Get("LastWindowPosition", &got); err != nil || got != (setting.SavedWindowPosition{X: -1920, Y: 40, Valid: true}) {
		t.Fatalf("local position = %+v, err = %v", got, err)
	}
	var legacyCount int64
	if err := db.Model(&database.WoxSetting{}).Where("key IN ?", []string{"LastWindowX", "LastWindowY"}).Count(&legacyCount).Error; err != nil || legacyCount != 0 {
		t.Fatalf("legacy rows = %d, err = %v", legacyCount, err)
	}
	var ops []database.Oplog
	if err := db.Order("id").Find(&ops).Error; err != nil {
		t.Fatal(err)
	}
	if len(ops) != 3 || !ops[0].SyncedToCloud || ops[1].Operation != cloudsync.OpDelete || ops[2].Operation != cloudsync.OpDelete {
		t.Fatalf("oplogs = %+v, want superseded upsert and one delete per legacy key", ops)
	}
}
