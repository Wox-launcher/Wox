package cloudsync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"wox/database"
	"wox/util"
)

// TestOplogMaintenanceWaitsForLocalTwoAM covers calendar boundaries and DST transitions.
func TestOplogMaintenanceWaitsForLocalTwoAM(t *testing.T) {
	local := time.FixedZone("CST", 8*60*60)
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		now  time.Time
		wait time.Duration
	}{
		{"evening", time.Date(2026, 10, 10, 23, 50, 0, 0, local), 2*time.Hour + 10*time.Minute},
		{"midnight", time.Date(2026, 10, 10, 0, 0, 0, 0, local), 2 * time.Hour},
		{"before two AM", time.Date(2026, 10, 10, 1, 50, 0, 0, local), 10 * time.Minute},
		{"at two AM", time.Date(2026, 10, 10, 2, 0, 0, 0, local), 24 * time.Hour},
		{"after two AM", time.Date(2026, 10, 10, 2, 10, 0, 0, local), 23*time.Hour + 50*time.Minute},
		{"month boundary", time.Date(2026, 2, 28, 23, 50, 0, 0, local), 2*time.Hour + 10*time.Minute},
		{"year boundary", time.Date(2026, 12, 31, 23, 50, 0, 0, local), 2*time.Hour + 10*time.Minute},
		{"spring DST", time.Date(2026, 3, 8, 0, 0, 0, 0, newYork), 2 * time.Hour},
		{"before spring DST jump", time.Date(2026, 3, 8, 1, 30, 0, 0, newYork), 30 * time.Minute},
		{"after spring DST jump", time.Date(2026, 3, 8, 3, 0, 0, 0, newYork), 23 * time.Hour},
		{"autumn DST", time.Date(2026, 11, 1, 0, 0, 0, 0, newYork), 3 * time.Hour},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := durationUntilOplogMaintenance(test.now); got != test.wait {
				t.Fatalf("wait = %s, want %s", got, test.wait)
			}
		})
	}
}

// TestStartOplogMaintenanceDefersCleanup guards against synchronous work returning to startup.
func TestStartOplogMaintenanceDefersCleanup(t *testing.T) {
	initCloudSyncTestDatabase(t)
	db := database.GetDB()
	rows := []database.Oplog{
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "watch", Timestamp: 10, SyncedToCloud: true},
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "watch", Timestamp: 20, SyncedToCloud: true},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	StartOplogMaintenance(ctx)
	var count int64
	if err := db.Model(&database.Oplog{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("oplogs after scheduling = %d, want cleanup deferred until 02:00", count)
	}
}

func TestPruneRetiredOplogsKeepsSyncWatermarks(t *testing.T) {
	initCloudSyncTestDatabase(t)
	ctx := context.Background()
	db := database.GetDB()

	rows := []database.Oplog{
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "watch", Operation: OpUpsert, Timestamp: 10, Value: "old-synced", SyncedToCloud: true},
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "watch", Operation: OpUpsert, Timestamp: 20, Value: "discarded-history", SyncedToCloud: true, CloudSyncDiscarded: true},
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "watch", Operation: OpUpsert, Timestamp: 30, Value: "live", SyncedToCloud: true},
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "blocked", Operation: OpUpsert, Timestamp: 10, Value: "older-pending"},
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "blocked", Operation: OpUpsert, Timestamp: 20, Value: "tombstone", CloudSyncDiscarded: true},
		{EntityType: EntityPluginSetting, EntityID: "other", Key: "watch", Operation: OpUpsert, Timestamp: 10, Value: "other-live"},
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "tie", Operation: OpUpsert, Timestamp: 40, Value: "tie-old", SyncedToCloud: true},
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "tie", Operation: OpUpsert, Timestamp: 40, Value: "tie-new", SyncedToCloud: true},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}

	deleted, cleared, err := pruneRetiredOplogs(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 3 {
		t.Fatalf("deleted = %d, want 3", deleted)
	}
	if cleared != 1 {
		t.Fatalf("cleared = %d, want 1", cleared)
	}

	var remaining []database.Oplog
	if err := db.Order("id asc").Find(&remaining).Error; err != nil {
		t.Fatal(err)
	}
	got := map[string]database.Oplog{}
	for _, row := range remaining {
		got[row.Key+"/"+row.Value] = row
	}
	if _, ok := got["watch/live"]; !ok {
		t.Fatalf("latest synced snapshot missing: %+v", remaining)
	}
	if _, ok := got["watch/old-synced"]; ok {
		t.Fatal("older synced snapshot was kept")
	}
	if _, ok := got["watch/discarded-history"]; ok {
		t.Fatal("discarded history was kept")
	}
	if row, ok := got["blocked/older-pending"]; !ok || row.CloudSyncDiscarded {
		t.Fatalf("older pending row = %+v, want it retained", row)
	}
	tombstone, ok := got["blocked/"]
	if !ok || !tombstone.CloudSyncDiscarded {
		t.Fatalf("discarded tombstone = %+v, want an empty value", remaining)
	}
	if _, ok := got["watch/other-live"]; !ok {
		t.Fatal("other identity was pruned")
	}
	if _, ok := got["tie/tie-new"]; !ok {
		t.Fatal("newer tie row was pruned")
	}
	if _, ok := got["tie/tie-old"]; ok {
		t.Fatal("older tie row was kept")
	}

	store := NewDefaultOplogStore()
	pending, err := store.LoadPending(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].EntityID != "other" || pending[0].Value != "other-live" {
		t.Fatalf("pending after prune = %+v, want only the other identity", pending)
	}
	localTs := latestLocalChangeTimestamp(CloudSyncRecord{EntityType: EntityPluginSetting, PluginID: "stock", Key: "blocked"})
	if localTs != 10 {
		t.Fatalf("local watermark = %d, want 10", localTs)
	}
	liveTs := latestLocalChangeTimestamp(CloudSyncRecord{EntityType: EntityPluginSetting, PluginID: "stock", Key: "watch"})
	if liveTs != 30 {
		t.Fatalf("synced watermark = %d, want 30", liveTs)
	}
}

// TestPruneRetiredOplogsRemovesSupersededPending covers obsolete rows that never receive upload acknowledgements.
func TestPruneRetiredOplogsRemovesSupersededPending(t *testing.T) {
	initCloudSyncTestDatabase(t)
	ctx := context.Background()
	db := database.GetDB()
	rows := []database.Oplog{
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "pending", Operation: OpUpsert, Timestamp: 10, Value: "obsolete"},
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "pending", Operation: OpUpsert, Timestamp: 20, Value: "latest", SyncAfter: time.Now().Add(time.Hour).UnixMilli()},
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "synced", Operation: OpUpsert, Timestamp: 10, Value: "obsolete"},
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "synced", Operation: OpUpsert, Timestamp: 20, Value: "latest"},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	store := NewDefaultOplogStore()
	pending, err := store.LoadPending(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].ID != rows[3].ID {
		t.Fatalf("pending = %+v, want only the latest due row", pending)
	}
	if err := store.MarkSynced(ctx, []uint{pending[0].ID}); err != nil {
		t.Fatal(err)
	}
	deleted, cleared, err := pruneRetiredOplogs(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 2 || cleared != 0 {
		t.Fatalf("deleted = %d, cleared = %d, want 2 and 0", deleted, cleared)
	}
	var remaining []database.Oplog
	if err := db.Order("id asc").Find(&remaining).Error; err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 2 || remaining[0].ID != rows[1].ID || remaining[1].ID != rows[3].ID {
		t.Fatalf("remaining = %+v, want the delayed and synced latest rows", remaining)
	}
	for _, key := range []string{"pending", "synced"} {
		if got := latestLocalChangeTimestamp(CloudSyncRecord{EntityType: EntityPluginSetting, PluginID: "stock", Key: key}); got != 20 {
			t.Fatalf("local timestamp for %s = %d, want 20", key, got)
		}
	}
	deleted, cleared, err = pruneRetiredOplogs(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 0 || cleared != 0 {
		t.Fatalf("repeat cleanup: deleted = %d, cleared = %d, want 0 and 0", deleted, cleared)
	}
}

func TestVacuumOplogStorageShrinksFreedPages(t *testing.T) {
	initCloudSyncTestDatabase(t)
	ctx := context.Background()
	db := database.GetDB()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	// Reuse one physical connection so a leaked PRAGMA cannot hide in the pool.
	sqlDB.SetMaxOpenConns(1)
	var busyTimeoutBefore int
	if err := sqlDB.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeoutBefore); err != nil {
		t.Fatal(err)
	}
	previousMinFreeBytes := oplogVacuumMinFreeBytes
	oplogVacuumMinFreeBytes = 1
	t.Cleanup(func() {
		oplogVacuumMinFreeBytes = previousMinFreeBytes
	})

	payload := strings.Repeat("x", 512*1024)
	rows := []database.Oplog{
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "cache", Operation: OpUpsert, Timestamp: 10, Value: payload, SyncedToCloud: true, CloudSyncDiscarded: true},
		{EntityType: EntityPluginSetting, EntityID: "stock", Key: "cache", Operation: OpUpsert, Timestamp: 20, Value: "current"},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := pruneRetiredOplogs(ctx, db); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(util.GetLocation().GetUserDataDirectory(), "wox.db")
	before, err := os.Stat(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	vacuumed, err := vacuumOplogStorage(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if !vacuumed {
		t.Fatal("expected freelist pages to be vacuumed")
	}
	after, err := os.Stat(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if after.Size() >= before.Size() {
		t.Fatalf("database size = %d, want it below %d", after.Size(), before.Size())
	}

	var busyTimeoutAfter int
	if err := sqlDB.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeoutAfter); err != nil {
		t.Fatal(err)
	}
	if busyTimeoutAfter != busyTimeoutBefore {
		t.Fatalf("busy timeout = %d, want %d", busyTimeoutAfter, busyTimeoutBefore)
	}
	freeBytes, err := sqliteFreeBytes(ctx, sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	if freeBytes != 0 {
		t.Fatalf("freelist bytes = %d, want 0", freeBytes)
	}
}
