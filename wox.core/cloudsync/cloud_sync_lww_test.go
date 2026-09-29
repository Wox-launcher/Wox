package cloudsync

import (
	"context"
	"errors"
	"testing"
	"time"

	"wox/database"
)

func TestApplyRecordsKeepsNewerLocalChange(t *testing.T) {
	ctx := context.Background()
	initCloudSyncTestDatabase(t)
	if err := database.GetDB().Create(&database.Oplog{
		EntityType: EntityWoxSetting,
		EntityID:   "LWWKeepLocal",
		Operation:  OpUpsert,
		Key:        "LWWKeepLocal",
		Value:      "local-theme",
		Timestamp:  200,
	}).Error; err != nil {
		t.Fatalf("create oplog: %v", err)
	}

	applier := &testCloudSyncApplier{}
	manager := NewCloudSyncManager(DefaultCloudSyncConfig(), CloudSyncDependencies{
		Crypto:  testCloudSyncCrypto{},
		Applier: applier,
	})
	if err := manager.applyRecords(ctx, []CloudSyncRecord{
		{EntityType: EntityWoxSetting, Key: "LWWKeepLocal", Op: OpUpsert, ClientTs: 100, Value: &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "remote-theme"}},
	}); err != nil {
		t.Fatalf("apply records: %v", err)
	}
	if _, applied := applier.wox["LWWKeepLocal"]; applied {
		t.Fatalf("older remote theme was applied: %#v", applier.wox)
	}

	var pending int64
	if err := database.GetDB().Model(&database.Oplog{}).Where("key = ? AND cloud_sync_discarded = ?", "LWWKeepLocal", false).Count(&pending).Error; err != nil {
		t.Fatalf("count oplog: %v", err)
	}
	if pending != 1 {
		t.Fatalf("pending local oplogs = %d, want 1", pending)
	}
}

func TestApplyRecordsUsesNewerRemoteChangeAndDropsOlderPendingUpload(t *testing.T) {
	ctx := context.Background()
	initCloudSyncTestDatabase(t)
	if err := database.GetDB().Create(&[]database.Oplog{
		{EntityType: EntityWoxSetting, EntityID: "LWWRemoteWins", Operation: OpUpsert, Key: "LWWRemoteWins", Value: "old-local", Timestamp: 50},
		{EntityType: EntityWoxSetting, EntityID: "LWWRemoteWins", Operation: OpUpsert, Key: "LWWRemoteWins", Value: "newer-local", Timestamp: 180},
	}).Error; err != nil {
		t.Fatalf("create oplogs: %v", err)
	}

	applier := &testCloudSyncApplier{}
	manager := NewCloudSyncManager(DefaultCloudSyncConfig(), CloudSyncDependencies{
		Crypto:  testCloudSyncCrypto{},
		Applier: applier,
	})
	records := []CloudSyncRecord{
		{EntityType: EntityWoxSetting, Key: "LWWRemoteWins", Op: OpUpsert, ClientTs: 200, Value: &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "newer-remote"}},
		{EntityType: EntityWoxSetting, Key: "LWWRemoteWins", Op: OpUpsert, ClientTs: 100, Value: &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "older-remote"}},
	}
	if err := manager.applyRecords(ctx, records); err != nil {
		t.Fatalf("apply records: %v", err)
	}
	if got := applier.wox["LWWRemoteWins"]; got != "newer-remote" {
		t.Fatalf("applied theme = %q, want newer-remote", got)
	}

	var discarded int64
	if err := database.GetDB().Model(&database.Oplog{}).Where("key = ? AND cloud_sync_discarded = ?", "LWWRemoteWins", true).Count(&discarded).Error; err != nil {
		t.Fatalf("count discarded: %v", err)
	}
	if discarded != 2 {
		t.Fatalf("discarded older oplogs = %d, want 2", discarded)
	}
}

func TestApplyRecordsKeepsNewerAppliedRemoteChange(t *testing.T) {
	ctx := context.Background()
	initCloudSyncTestDatabase(t)
	applier := &testCloudSyncApplier{}
	manager := NewCloudSyncManager(DefaultCloudSyncConfig(), CloudSyncDependencies{
		Crypto:  testCloudSyncCrypto{},
		Applier: applier,
	})
	if err := manager.applyRecords(ctx, []CloudSyncRecord{
		{EntityType: EntityPluginSetting, PluginID: "notes", Key: "note:1", Op: OpUpsert, ClientTs: 300, Value: &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "new"}},
	}); err != nil {
		t.Fatalf("apply newer: %v", err)
	}
	if err := manager.applyRecords(ctx, []CloudSyncRecord{
		{EntityType: EntityPluginSetting, PluginID: "notes", Key: "note:1", Op: OpUpsert, ClientTs: 100, Value: &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "old"}},
	}); err != nil {
		t.Fatalf("apply older: %v", err)
	}
	if got := applier.plugins["notes:note:1"]; got != "new" {
		t.Fatalf("plugin setting = %q, want new", got)
	}
}

func TestApplyRecordsEqualTimestampReplacesLocalAndDropsPendingUpload(t *testing.T) {
	ctx := context.Background()
	initCloudSyncTestDatabase(t)
	if err := database.GetDB().Create(&database.Oplog{
		EntityType: EntityWoxSetting,
		EntityID:   "LWWTie",
		Operation:  OpUpsert,
		Key:        "LWWTie",
		Value:      "local-value",
		Timestamp:  100,
	}).Error; err != nil {
		t.Fatalf("create oplog: %v", err)
	}

	applier := &testCloudSyncApplier{}
	manager := NewCloudSyncManager(DefaultCloudSyncConfig(), CloudSyncDependencies{
		Crypto:  testCloudSyncCrypto{},
		Applier: applier,
	})
	if err := manager.applyRecords(ctx, []CloudSyncRecord{
		{EntityType: EntityWoxSetting, Key: "LWWTie", Op: OpUpsert, ClientTs: 100, Value: &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "remote-value"}},
	}); err != nil {
		t.Fatalf("apply records: %v", err)
	}
	if got := applier.wox["LWWTie"]; got != "remote-value" {
		t.Fatalf("applied value = %q, want remote-value", got)
	}
	var pending int64
	if err := database.GetDB().Model(&database.Oplog{}).Where("key = ? AND cloud_sync_discarded = ?", "LWWTie", false).Count(&pending).Error; err != nil {
		t.Fatalf("count pending: %v", err)
	}
	if pending != 0 {
		t.Fatalf("pending oplogs = %d, want 0", pending)
	}
}

func TestRestoreAppliesOlderCloudOverNewerLocal(t *testing.T) {
	ctx := context.Background()
	initCloudSyncTestDatabase(t)
	if err := database.GetDB().Create(&[]database.Oplog{
		{EntityType: EntityWoxSetting, EntityID: "LWWRestore", Operation: OpUpsert, Key: "LWWRestore", Value: "local-theme", Timestamp: 200},
		{EntityType: EntityWoxSetting, EntityID: "LWWRestore", Operation: OpUpsert, Key: "LWWRestore", Value: "synced-theme", Timestamp: 250, SyncedToCloud: true},
	}).Error; err != nil {
		t.Fatalf("create oplogs: %v", err)
	}
	if err := database.GetDB().Create(&database.CloudSyncRecordVersion{
		EntityType: EntityWoxSetting, EntityID: "LWWRestore", Key: "LWWRestore", ClientTs: 300,
	}).Error; err != nil {
		t.Fatalf("create version: %v", err)
	}

	applier := &testCloudSyncApplier{}
	manager := NewCloudSyncManager(DefaultCloudSyncConfig(), CloudSyncDependencies{
		Crypto:  testCloudSyncCrypto{},
		Applier: applier,
	})
	if err := manager.applyRecordsWithDetails(ctx, []CloudSyncRecord{
		{EntityType: EntityWoxSetting, Key: "LWWRestore", Op: OpUpsert, ClientTs: 100, Value: &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "cloud-theme"}},
	}, nil, true).Err(); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if got := applier.wox["LWWRestore"]; got != "cloud-theme" {
		t.Fatalf("restored theme = %q, want cloud-theme", got)
	}
	var pending int64
	if err := database.GetDB().Model(&database.Oplog{}).Where("key = ? AND cloud_sync_discarded = ?", "LWWRestore", false).Count(&pending).Error; err != nil {
		t.Fatalf("count oplogs: %v", err)
	}
	if pending != 0 {
		t.Fatalf("local oplogs kept = %d, want 0", pending)
	}
	var version database.CloudSyncRecordVersion
	if err := database.GetDB().Where("key = ?", "LWWRestore").Take(&version).Error; err != nil {
		t.Fatalf("load version: %v", err)
	}
	if version.ClientTs != 100 {
		t.Fatalf("baseline = %d, want 100", version.ClientTs)
	}

	if err := manager.applyRecords(ctx, []CloudSyncRecord{
		{EntityType: EntityWoxSetting, Key: "LWWRestore", Op: OpUpsert, ClientTs: 50, Value: &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "older-cloud"}},
	}); err != nil {
		t.Fatalf("apply older incremental: %v", err)
	}
	if got := applier.wox["LWWRestore"]; got != "cloud-theme" {
		t.Fatalf("older incremental replaced restore: %q", got)
	}
	if err := manager.applyRecords(ctx, []CloudSyncRecord{
		{EntityType: EntityWoxSetting, Key: "LWWRestore", Op: OpUpsert, ClientTs: 150, Value: &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "newer-cloud"}},
	}); err != nil {
		t.Fatalf("apply newer incremental: %v", err)
	}
	if got := applier.wox["LWWRestore"]; got != "newer-cloud" {
		t.Fatalf("newer incremental = %q, want newer-cloud", got)
	}
}

func TestRestoreKeepsLocalEditWrittenAfterRemoteApply(t *testing.T) {
	ctx := context.Background()
	initCloudSyncTestDatabase(t)
	if err := database.GetDB().Create(&database.Oplog{
		EntityType: EntityWoxSetting,
		EntityID:   "LWWRestoreRace",
		Operation:  OpUpsert,
		Key:        "LWWRestoreRace",
		Value:      "old-local",
		Timestamp:  200,
	}).Error; err != nil {
		t.Fatalf("create oplog: %v", err)
	}

	finished := make(chan error, 1)
	applier := &testCloudSyncApplier{duringApply: func() {
		entered := make(chan struct{})
		go func() {
			close(entered)
			finished <- WithLocalSyncMutation(func() error {
				if err := database.GetDB().Save(&database.WoxSetting{Key: "LWWRestoreRace", Value: "user-edit"}).Error; err != nil {
					return err
				}
				return database.GetDB().Create(&database.Oplog{
					EntityType: EntityWoxSetting,
					EntityID:   "LWWRestoreRace",
					Operation:  OpUpsert,
					Key:        "LWWRestoreRace",
					Value:      "user-edit",
					Timestamp:  500,
				}).Error
			})
		}()
		<-entered
		select {
		case err := <-finished:
			t.Errorf("local edit finished while restore still held the apply section: %v", err)
		case <-time.After(50 * time.Millisecond):
		}
	}}
	manager := NewCloudSyncManager(DefaultCloudSyncConfig(), CloudSyncDependencies{
		Crypto:  testCloudSyncCrypto{},
		Applier: applier,
	})
	if err := manager.applyRecordsWithDetails(ctx, []CloudSyncRecord{
		{EntityType: EntityWoxSetting, Key: "LWWRestoreRace", Op: OpUpsert, ClientTs: 100, Value: &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "cloud-theme"}},
	}, nil, true).Err(); err != nil {
		t.Fatalf("restore: %v", err)
	}
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("local edit: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("local edit did not finish after restore released the apply section")
	}

	if got := applier.wox["LWWRestoreRace"]; got != "cloud-theme" {
		t.Fatalf("applied value = %q, want cloud-theme", got)
	}
	var setting database.WoxSetting
	if err := database.GetDB().Where("key = ?", "LWWRestoreRace").Take(&setting).Error; err != nil {
		t.Fatalf("load setting: %v", err)
	}
	if setting.Value != "user-edit" {
		t.Fatalf("local setting = %q, want user-edit", setting.Value)
	}
	var pending int64
	if err := database.GetDB().Model(&database.Oplog{}).Where("key = ? AND value = ? AND cloud_sync_discarded = ?", "LWWRestoreRace", "user-edit", false).Count(&pending).Error; err != nil {
		t.Fatalf("count pending: %v", err)
	}
	if pending != 1 {
		t.Fatalf("pending user-edit oplogs = %d, want 1", pending)
	}
	var discarded int64
	if err := database.GetDB().Model(&database.Oplog{}).Where("key = ? AND value = ? AND cloud_sync_discarded = ?", "LWWRestoreRace", "old-local", true).Count(&discarded).Error; err != nil {
		t.Fatalf("count discarded: %v", err)
	}
	if discarded != 1 {
		t.Fatalf("discarded old oplogs = %d, want 1", discarded)
	}
}

func TestApplyComparesLocalTimestampInsideTheMutationLock(t *testing.T) {
	ctx := context.Background()
	initCloudSyncTestDatabase(t)
	const key = "LWWCompareLock"
	if err := database.GetDB().Create(&database.Oplog{
		EntityType: EntityWoxSetting, EntityID: key, Operation: OpUpsert, Key: key, Value: "old-local", Timestamp: 50,
	}).Error; err != nil {
		t.Fatalf("create oplog: %v", err)
	}
	if err := database.GetDB().Save(&database.WoxSetting{Key: key, Value: "old-local"}).Error; err != nil {
		t.Fatalf("save setting: %v", err)
	}

	finished := make(chan error, 1)
	beforeReadLocalChangeTimestamp = func() {
		entered := make(chan struct{})
		go func() {
			close(entered)
			finished <- WithLocalSyncMutation(func() error {
				if err := database.GetDB().Save(&database.WoxSetting{Key: key, Value: "user-edit"}).Error; err != nil {
					return err
				}
				return database.GetDB().Create(&database.Oplog{
					EntityType: EntityWoxSetting, EntityID: key, Operation: OpUpsert, Key: key, Value: "user-edit", Timestamp: 200,
				}).Error
			})
		}()
		<-entered
		select {
		case err := <-finished:
			t.Errorf("local write landed before the timestamp comparison finished: %v", err)
		case <-time.After(50 * time.Millisecond):
		}
	}
	t.Cleanup(func() { beforeReadLocalChangeTimestamp = nil })

	applier := &testCloudSyncApplier{duringApply: func() {
		if err := database.GetDB().Save(&database.WoxSetting{Key: key, Value: "remote-value"}).Error; err != nil {
			t.Errorf("save applied value: %v", err)
		}
	}}
	manager := NewCloudSyncManager(DefaultCloudSyncConfig(), CloudSyncDependencies{
		Crypto:  testCloudSyncCrypto{},
		Applier: applier,
	})
	if err := manager.applyRecords(ctx, []CloudSyncRecord{
		{EntityType: EntityWoxSetting, Key: key, Op: OpUpsert, ClientTs: 100, Value: &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "remote-value"}},
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("local write: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("local write did not finish after comparison and apply")
	}
	if got := applier.wox[key]; got != "remote-value" {
		t.Fatalf("applied value = %q, want remote-value", got)
	}
	var setting database.WoxSetting
	if err := database.GetDB().Where("key = ?", key).Take(&setting).Error; err != nil {
		t.Fatalf("load setting: %v", err)
	}
	if setting.Value != "user-edit" {
		t.Fatalf("local setting = %q, want user-edit", setting.Value)
	}
	var pending int64
	if err := database.GetDB().Model(&database.Oplog{}).Where("key = ? AND value = ? AND cloud_sync_discarded = ?", key, "user-edit", false).Count(&pending).Error; err != nil {
		t.Fatalf("count pending: %v", err)
	}
	if pending != 1 {
		t.Fatalf("pending user-edit oplogs = %d, want 1", pending)
	}
	var discarded int64
	if err := database.GetDB().Model(&database.Oplog{}).Where("key = ? AND value = ? AND cloud_sync_discarded = ?", key, "old-local", true).Count(&discarded).Error; err != nil {
		t.Fatalf("count discarded: %v", err)
	}
	if discarded != 1 {
		t.Fatalf("discarded old oplogs = %d, want 1", discarded)
	}
}

func TestRestoreKeepsOplogWrittenByReentrantCallback(t *testing.T) {
	ctx := context.Background()
	initCloudSyncTestDatabase(t)
	const key = "LWWCallback"
	existing := database.Oplog{
		EntityType: EntityWoxSetting, EntityID: key, Operation: OpUpsert, Key: key, Value: "old-local", Timestamp: 200,
	}
	if err := database.GetDB().Create(&existing).Error; err != nil {
		t.Fatalf("create oplog: %v", err)
	}

	applier := &testCloudSyncApplier{duringApply: func() {
		if err := WithLocalSyncMutation(func() error {
			if err := database.GetDB().Model(&database.Oplog{}).Where("id = ?", existing.ID).Updates(map[string]any{
				"value":     "updated-edit",
				"timestamp": 800,
			}).Error; err != nil {
				return err
			}
			return database.GetDB().Create(&database.Oplog{
				EntityType: EntityWoxSetting, EntityID: key, Operation: OpUpsert, Key: key, Value: "callback-edit", Timestamp: 900,
			}).Error
		}); err != nil {
			t.Errorf("callback write: %v", err)
		}
	}}
	manager := NewCloudSyncManager(DefaultCloudSyncConfig(), CloudSyncDependencies{
		Crypto:  testCloudSyncCrypto{},
		Applier: applier,
	})
	if err := manager.applyRecordsWithDetails(ctx, []CloudSyncRecord{
		{EntityType: EntityWoxSetting, Key: key, Op: OpUpsert, ClientTs: 100, Value: &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "cloud-theme"}},
	}, nil, true).Err(); err != nil {
		t.Fatalf("restore: %v", err)
	}

	var updated database.Oplog
	if err := database.GetDB().Where("id = ?", existing.ID).Take(&updated).Error; err != nil {
		t.Fatalf("load updated oplog: %v", err)
	}
	if updated.CloudSyncDiscarded || updated.Value != "updated-edit" {
		t.Fatalf("updated oplog = %+v, want pending updated-edit", updated)
	}
	var created int64
	if err := database.GetDB().Model(&database.Oplog{}).Where("key = ? AND value = ? AND cloud_sync_discarded = ?", key, "callback-edit", false).Count(&created).Error; err != nil {
		t.Fatalf("count created: %v", err)
	}
	if created != 1 {
		t.Fatalf("pending callback oplogs = %d, want 1", created)
	}
}

func TestInstalledPluginApplyDoesNotHoldSyncLock(t *testing.T) {
	ctx := context.Background()
	initCloudSyncTestDatabase(t)
	const key = "LWWInstallLock"
	if err := database.GetDB().Create(&database.Oplog{
		EntityType: EntityInstalledPlugin, EntityID: key, Operation: OpUpsert, Key: key, Value: "old-install", Timestamp: 50,
	}).Error; err != nil {
		t.Fatalf("create oplog: %v", err)
	}

	acquired := make(chan struct{})
	finished := make(chan error, 1)
	applier := &testCloudSyncApplier{duringApply: func() {
		go func() {
			finished <- WithLocalSyncMutation(func() error {
				close(acquired)
				return database.GetDB().Create(&database.Oplog{
					EntityType: EntityInstalledPlugin, EntityID: key, Operation: OpUpsert, Key: key, Value: "user-install", Timestamp: 900,
				}).Error
			})
		}()
		select {
		case <-acquired:
		case <-time.After(time.Second):
			t.Error("sync lock stayed held during plugin install")
		}
	}}
	manager := NewCloudSyncManager(DefaultCloudSyncConfig(), CloudSyncDependencies{
		Crypto:  testCloudSyncCrypto{},
		Applier: applier,
	})
	if err := manager.applyRecords(ctx, []CloudSyncRecord{
		{EntityType: EntityInstalledPlugin, Key: key, Op: OpUpsert, ClientTs: 100, Value: &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "{}"}},
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("local install oplog: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("local install oplog did not finish")
	}
	var pending int64
	if err := database.GetDB().Model(&database.Oplog{}).Where("key = ? AND value = ? AND cloud_sync_discarded = ?", key, "user-install", false).Count(&pending).Error; err != nil {
		t.Fatalf("count pending: %v", err)
	}
	if pending != 1 {
		t.Fatalf("pending user install oplogs = %d, want 1", pending)
	}
	var discarded int64
	if err := database.GetDB().Model(&database.Oplog{}).Where("key = ? AND value = ? AND cloud_sync_discarded = ?", key, "old-install", true).Count(&discarded).Error; err != nil {
		t.Fatalf("count discarded: %v", err)
	}
	if discarded != 1 {
		t.Fatalf("discarded old install oplogs = %d, want 1", discarded)
	}
}

func TestHistoricalOplogCleanupPagesSyncedRows(t *testing.T) {
	ctx := context.Background()
	initCloudSyncTestDatabase(t)
	originalBatch := oplogIDBatchSize
	oplogIDBatchSize = 2
	t.Cleanup(func() { oplogIDBatchSize = originalBatch })

	const incrementalKey = "LWWHistoryIncremental"
	const restoreKey = "LWWHistoryRestore"
	rows := make([]database.Oplog, 0, 11)
	for i := 0; i < 5; i++ {
		rows = append(rows, database.Oplog{
			EntityType: EntityWoxSetting, EntityID: incrementalKey, Operation: OpUpsert, Key: incrementalKey, Value: "synced", Timestamp: 10, SyncedToCloud: true,
		})
		rows = append(rows, database.Oplog{
			EntityType: EntityWoxSetting, EntityID: restoreKey, Operation: OpUpsert, Key: restoreKey, Value: "synced", Timestamp: 300, SyncedToCloud: true,
		})
	}
	rows = append(rows, database.Oplog{
		EntityType: EntityWoxSetting, EntityID: incrementalKey, Operation: OpUpsert, Key: incrementalKey, Value: "pending", Timestamp: 50,
	})
	if err := database.GetDB().Create(&rows).Error; err != nil {
		t.Fatalf("create oplogs: %v", err)
	}

	manager := NewCloudSyncManager(DefaultCloudSyncConfig(), CloudSyncDependencies{
		Crypto:  testCloudSyncCrypto{},
		Applier: &testCloudSyncApplier{},
	})
	if err := manager.applyRecords(ctx, []CloudSyncRecord{
		{EntityType: EntityWoxSetting, Key: incrementalKey, Op: OpUpsert, ClientTs: 100, Value: &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "remote"}},
	}); err != nil {
		t.Fatalf("incremental apply: %v", err)
	}
	assertOplogCount(t, incrementalKey, "synced", false, 5)
	assertOplogCount(t, incrementalKey, "pending", true, 1)

	if err := manager.applyRecordsWithDetails(ctx, []CloudSyncRecord{
		{EntityType: EntityWoxSetting, Key: restoreKey, Op: OpUpsert, ClientTs: 100, Value: &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "cloud"}},
	}, nil, true).Err(); err != nil {
		t.Fatalf("restore: %v", err)
	}
	assertOplogCount(t, restoreKey, "synced", true, 5)
}

func assertOplogCount(t *testing.T, key string, value string, discarded bool, want int64) {
	t.Helper()
	var count int64
	err := database.GetDB().Model(&database.Oplog{}).
		Where("key = ? AND value = ? AND cloud_sync_discarded = ?", key, value, discarded).
		Count(&count).Error
	if err != nil {
		t.Fatalf("count %s/%s discarded=%v: %v", key, value, discarded, err)
	}
	if count != want {
		t.Fatalf("%s %s discarded=%v count = %d, want %d", key, value, discarded, count, want)
	}
}

func TestPullBookkeepingFailureDoesNotAdvanceCursor(t *testing.T) {
	ctx := context.Background()
	initCloudSyncTestDatabase(t)
	original := noteRemoteChangeForApply
	noteRemoteChangeForApply = func(record CloudSyncRecord, restore bool, preexisting []preApplyOplog) error {
		return errors.New("cleanup failed")
	}
	t.Cleanup(func() {
		noteRemoteChangeForApply = original
	})
	if err := SaveCloudSyncState(ctx, &database.CloudSyncState{ID: cloudSyncStateID, Cursor: "10"}); err != nil {
		t.Fatalf("save state: %v", err)
	}
	if err := database.GetDB().Create(&database.Oplog{
		EntityType: EntityWoxSetting,
		EntityID:   "LWWBookkeeping",
		Operation:  OpUpsert,
		Key:        "LWWBookkeeping",
		Value:      "old-local",
		Timestamp:  50,
	}).Error; err != nil {
		t.Fatalf("create oplog: %v", err)
	}

	client := &testCloudSyncClient{pullResponses: []*CloudSyncPullResponse{{
		Records: []CloudSyncRecord{{
			EntityType: EntityWoxSetting,
			Key:        "LWWBookkeeping",
			Op:         OpUpsert,
			ClientTs:   100,
			Value:      &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "remote"},
		}},
		NextCursor: "99",
	}}}
	applier := &testCloudSyncApplier{}
	manager := NewCloudSyncManager(DefaultCloudSyncConfig(), CloudSyncDependencies{
		Client:         client,
		Crypto:         testCloudSyncCrypto{},
		DeviceProvider: testCloudSyncDeviceProvider{deviceID: "device-a"},
		OplogStore:     &testCloudSyncOplogStore{},
		Applier:        applier,
	})
	manager.Pull(ctx, "test")
	manager.PushPending(ctx, "test")

	state, err := LoadCloudSyncState(ctx)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.Cursor != "10" {
		t.Fatalf("cursor = %q, want 10", state.Cursor)
	}
	if state.BackoffUntil == 0 {
		t.Fatal("backoff was not started after bookkeeping failure")
	}
	if len(client.pushRequests) != 0 {
		t.Fatalf("push requests = %d, want 0", len(client.pushRequests))
	}
	var pending int64
	if err := database.GetDB().Model(&database.Oplog{}).Where("key = ? AND cloud_sync_discarded = ?", "LWWBookkeeping", false).Count(&pending).Error; err != nil {
		t.Fatalf("count pending: %v", err)
	}
	if pending != 1 {
		t.Fatalf("pending oplogs = %d, want 1", pending)
	}
	if got := applier.wox["LWWBookkeeping"]; got != "remote" {
		t.Fatalf("applied value = %q, want remote", got)
	}
}

func TestRestoreBookkeepingFailureDoesNotBootstrap(t *testing.T) {
	ctx := context.Background()
	initCloudSyncTestDatabase(t)
	original := noteRemoteChangeForApply
	noteRemoteChangeForApply = func(record CloudSyncRecord, restore bool, preexisting []preApplyOplog) error {
		return errors.New("cleanup failed")
	}
	t.Cleanup(func() {
		noteRemoteChangeForApply = original
	})
	if err := database.GetDB().Create(&database.Oplog{
		EntityType: EntityWoxSetting,
		EntityID:   "LWWRestoreBookkeeping",
		Operation:  OpUpsert,
		Key:        "LWWRestoreBookkeeping",
		Value:      "old-local",
		Timestamp:  50,
	}).Error; err != nil {
		t.Fatalf("create oplog: %v", err)
	}

	manager := NewCloudSyncManager(DefaultCloudSyncConfig(), CloudSyncDependencies{
		Client: &testCloudSyncClient{snapshotResponses: []*CloudSyncPullResponse{{
			Records: []CloudSyncRecord{{
				EntityType: EntityWoxSetting,
				Key:        "LWWRestoreBookkeeping",
				Op:         OpUpsert,
				ClientTs:   100,
				Value:      &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "cloud-theme"},
			}},
		}}},
		Crypto:         testCloudSyncCrypto{},
		DeviceProvider: testCloudSyncDeviceProvider{deviceID: "device-a"},
		Applier:        &testCloudSyncApplier{},
	})
	if err := manager.RestoreSnapshot(ctx); err == nil {
		t.Fatal("RestoreSnapshot succeeded, want bookkeeping error")
	}
	state, err := LoadCloudSyncState(ctx)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.Bootstrapped {
		t.Fatal("state bootstrapped = true, want false")
	}
	var pending int64
	if err := database.GetDB().Model(&database.Oplog{}).Where("key = ? AND cloud_sync_discarded = ?", "LWWRestoreBookkeeping", false).Count(&pending).Error; err != nil {
		t.Fatalf("count pending: %v", err)
	}
	if pending != 1 {
		t.Fatalf("pending oplogs = %d, want 1", pending)
	}
}

func TestResetCloudSyncStateClearsRecordVersions(t *testing.T) {
	ctx := context.Background()
	initCloudSyncTestDatabase(t)
	if err := database.GetDB().Create(&database.CloudSyncRecordVersion{
		EntityType: EntityWoxSetting, EntityID: "LWWReset", Key: "LWWReset", ClientTs: 300,
	}).Error; err != nil {
		t.Fatalf("create version: %v", err)
	}
	if err := ResetCloudSyncState(ctx); err != nil {
		t.Fatalf("reset: %v", err)
	}
	var count int64
	if err := database.GetDB().Model(&database.CloudSyncRecordVersion{}).Count(&count).Error; err != nil {
		t.Fatalf("count versions: %v", err)
	}
	if count != 0 {
		t.Fatalf("record versions = %d, want 0", count)
	}
}

func TestPluginSettingCallbackRunsAfterSyncLockReleased(t *testing.T) {
	ctx := context.Background()
	initCloudSyncTestDatabase(t)
	const key = "LWWPluginNotify"
	acquired := make(chan struct{})
	applier := &pluginNotifyApplier{acquired: acquired}
	manager := NewCloudSyncManager(DefaultCloudSyncConfig(), CloudSyncDependencies{
		Crypto:  testCloudSyncCrypto{},
		Applier: applier,
	})
	if err := manager.applyRecords(ctx, []CloudSyncRecord{{
		EntityType: EntityPluginSetting,
		PluginID:   "py",
		Key:        key,
		Op:         OpUpsert,
		ClientTs:   100,
		Value:      &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "remote"},
	}}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("plugin callback write blocked on the sync lock")
	}
	if applier.calls != 1 {
		t.Fatalf("notifications = %d, want 1", applier.calls)
	}
	if got := applier.plugins["py:"+key]; got != "remote" {
		t.Fatalf("applied value = %q, want remote", got)
	}

	original := noteRemoteChangeForApply
	noteRemoteChangeForApply = func(record CloudSyncRecord, restore bool, preexisting []preApplyOplog) error {
		return errors.New("cleanup failed")
	}
	t.Cleanup(func() { noteRemoteChangeForApply = original })
	err := manager.applyRecords(ctx, []CloudSyncRecord{{
		EntityType: EntityPluginSetting,
		PluginID:   "py",
		Key:        "LWWPluginNotifyFail",
		Op:         OpUpsert,
		ClientTs:   100,
		Value:      &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "remote"},
	}})
	if err == nil {
		t.Fatal("apply succeeded, want bookkeeping error")
	}
	if applier.calls != 1 {
		t.Fatalf("notifications after bookkeeping failure = %d, want 1", applier.calls)
	}
}

type pluginNotifyApplier struct {
	testCloudSyncApplier
	acquired chan struct{}
	calls    int
}

func (a *pluginNotifyApplier) DeliverDeferredPluginSettingNotification(ctx context.Context, pluginID string, key string, value string) {
	_ = ctx
	_ = pluginID
	_ = key
	_ = value
	a.calls++
	done := make(chan struct{})
	go func() {
		_ = WithLocalSyncMutation(func() error {
			close(done)
			return nil
		})
	}()
	select {
	case <-done:
		if a.acquired != nil {
			close(a.acquired)
			a.acquired = nil
		}
	case <-time.After(time.Second):
	}
}
