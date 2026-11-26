package migration

import (
	"context"
	"encoding/json"
	"testing"
	"wox/cloudsync"
	"wox/common"
	"wox/database"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMoveMemoryGlanceMigration(t *testing.T) {
	for _, test := range []struct {
		name, provider, glance string
		moved                  bool
	}{
		{"memory provider", common.WoxMemoryPluginID, "wox_memory", true},
		{"original provider", common.GlancePluginID, "wox_memory", true},
		{"clock", common.GlancePluginID, "time", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			if err := db.AutoMigrate(&database.WoxSetting{}, &database.Oplog{}); err != nil {
				t.Fatal(err)
			}
			const key = "PrimaryGlance"
			value, err := json.Marshal(map[string]string{
				"pluginId": test.provider,
				"GlanceId": test.glance,
				"Future":   "keep",
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&database.WoxSetting{Key: key, Value: string(value)}).Error; err != nil {
				t.Fatal(err)
			}
			migration := &moveMemoryGlanceMigration{}
			for range 2 {
				if err := db.Transaction(func(tx *gorm.DB) error { return migration.Up(ctx, tx) }); err != nil {
					t.Fatal(err)
				}
			}
			var row database.WoxSetting
			if err := db.First(&row, "key = ?", key).Error; err != nil {
				t.Fatal(err)
			}
			var ops []database.Oplog
			if err := db.Find(&ops).Error; err != nil {
				t.Fatal(err)
			}
			if !test.moved {
				if row.Value != string(value) || len(ops) != 0 {
					t.Fatalf("unrelated selection changed: %s, oplogs = %+v", row.Value, ops)
				}
				return
			}
			var fields map[string]string
			if err := json.Unmarshal([]byte(row.Value), &fields); err != nil {
				t.Fatal(err)
			}
			if fields["PluginId"] != common.DoctorPluginID || fields["GlanceId"] != test.glance || fields["Future"] != "keep" {
				t.Fatalf("migrated Glance = %s", row.Value)
			}
			if len(ops) != 1 || ops[0].EntityType != cloudsync.EntityWoxSetting || ops[0].Operation != cloudsync.OpUpsert || ops[0].Key != key || ops[0].Value != row.Value {
				t.Fatalf("expected one synchronized update, got %+v", ops)
			}
		})
	}
}
