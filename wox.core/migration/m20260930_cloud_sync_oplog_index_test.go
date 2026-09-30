package migration

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"wox/database"

	"gorm.io/gorm"
)

func TestCloudSyncOplogIndexMigration(t *testing.T) {
	db := openQuickJumpMigrationDB(t)
	rows := []database.Oplog{
		{EntityType: "plugin_setting", EntityID: "plugin", Key: "key", Timestamp: 10, Value: "old"},
		{EntityType: "plugin_setting", EntityID: "plugin", Key: "key", Timestamp: 20, Value: "new", SyncedToCloud: true},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	migration := &cloudSyncOplogIndexMigration{}
	for range 2 {
		if err := db.Transaction(func(tx *gorm.DB) error { return migration.Up(context.Background(), tx) }); err != nil {
			t.Fatal(err)
		}
	}
	var got []database.Oplog
	if err := db.Order("id").Find(&got).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rows, got) {
		t.Fatal("index migration changed oplog data")
	}
	var plan []struct{ Detail string }
	if err := db.Raw(`EXPLAIN QUERY PLAN SELECT id, ROW_NUMBER() OVER (
		PARTITION BY entity_type, entity_id, key ORDER BY timestamp DESC, id DESC
	) AS sync_rank FROM oplogs`).Scan(&plan).Error; err != nil {
		t.Fatal(err)
	}
	usesIndex := false
	for _, step := range plan {
		usesIndex = usesIndex || strings.Contains(step.Detail, "COVERING INDEX idx_oplogs_cloud_sync_latest")
		if strings.Contains(step.Detail, "TEMP B-TREE") {
			t.Fatalf("latest-state ranking still sorts all oplogs: %+v", plan)
		}
	}
	if !usesIndex {
		t.Fatalf("latest-state ranking does not use the covering index: %+v", plan)
	}
}
