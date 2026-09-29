package cloudsync

import (
	"context"
	"errors"
	"fmt"

	"wox/database"
	"wox/util"

	"gorm.io/gorm"
)

// remoteTimestampWins is the shared tie rule. An equal client_ts applies the remote value
// and drops the matching pending upload, so the local database and the upload queue stay aligned.
func remoteTimestampWins(remoteTs int64, localTs int64) bool {
	return remoteTs >= localTs
}

// beforeReadLocalChangeTimestamp runs inside the apply critical section, before
// the local timestamp is read. Tests use it to show that read cannot race a local write.
var beforeReadLocalChangeTimestamp func()

// remoteChangeIsOlder reports whether a pulled change lost last-write-wins.
// client_ts is the writer's modification time. A missing timestamp cannot be
// compared, so that record is still applied. Caller must hold the sync mutation
// lock so a local write cannot land between this read and the later apply.
func remoteChangeIsOlder(record CloudSyncRecord) (bool, int64) {
	if record.ClientTs <= 0 {
		return false, 0
	}
	if beforeReadLocalChangeTimestamp != nil {
		beforeReadLocalChangeTimestamp()
	}
	localTs := latestLocalChangeTimestamp(record)
	if localTs <= 0 || remoteTimestampWins(record.ClientTs, localTs) {
		return false, localTs
	}
	return true, localTs
}

// latestLocalChangeTimestamp is the newest local oplog or applied remote client_ts for one identity.
func latestLocalChangeTimestamp(record CloudSyncRecord) int64 {
	db := database.GetDB()
	if db == nil {
		return 0
	}
	entityID := syncRecordEntityID(record)

	var oplogTs int64
	_ = db.Model(&database.Oplog{}).
		Where("entity_type = ? AND entity_id = ? AND key = ? AND cloud_sync_discarded = ?", record.EntityType, entityID, record.Key, false).
		Select("COALESCE(MAX(timestamp), 0)").
		Scan(&oplogTs).Error

	var versionTs int64
	_ = db.Model(&database.CloudSyncRecordVersion{}).
		Where("entity_type = ? AND entity_id = ? AND key = ?", record.EntityType, entityID, record.Key).
		Select("COALESCE(MAX(client_ts), 0)").
		Scan(&versionTs).Error

	if oplogTs > versionTs {
		return oplogTs
	}
	return versionTs
}

// oplogIDBatchSize keeps each cleanup statement under SQLite's variable limit.
// One key can accumulate tens of thousands of synced oplogs, and a single IN
// clause over all of them fails with "too many SQL variables".
var oplogIDBatchSize = 500

// preApplyOplog is one local oplog row observed before the remote value is applied.
type preApplyOplog struct {
	ID        uint
	Timestamp int64
	Value     string
}

// noteRemoteChangeForApply is the bookkeeping step after a remote value is applied.
// Tests replace it to simulate a version-save or oplog-cleanup failure.
var noteRemoteChangeForApply = noteRemoteChange

// noteRemoteChange records the applied client_ts and drops local uploads that lost
// to this remote change. preexisting is the identity's oplog set from before apply.
// Rows created or edited by a reentrant callback are left queued.
func noteRemoteChange(record CloudSyncRecord, restore bool, preexisting []preApplyOplog) error {
	if restore {
		if err := forceAppliedClientTimestamp(record); err != nil {
			return err
		}
		return discardPreApplyOplogs(record, true, preexisting)
	}
	if record.ClientTs <= 0 {
		return nil
	}
	if err := rememberAppliedClientTimestamp(record); err != nil {
		return err
	}
	return discardPreApplyOplogs(record, false, preexisting)
}

func rememberAppliedClientTimestamp(record CloudSyncRecord) error {
	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	entityID := syncRecordEntityID(record)
	var existing database.CloudSyncRecordVersion
	err := db.Where("entity_type = ? AND entity_id = ? AND key = ?", record.EntityType, entityID, record.Key).Take(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(&database.CloudSyncRecordVersion{
			EntityType: record.EntityType,
			EntityID:   entityID,
			Key:        record.Key,
			ClientTs:   record.ClientTs,
		}).Error
	}
	if err != nil {
		return err
	}
	if remoteTimestampWins(existing.ClientTs, record.ClientTs) {
		return nil
	}
	return db.Model(&database.CloudSyncRecordVersion{}).
		Where("entity_type = ? AND entity_id = ? AND key = ?", record.EntityType, entityID, record.Key).
		Update("client_ts", record.ClientTs).Error
}

// forceAppliedClientTimestamp replaces the baseline with the restored cloud timestamp.
// A missing cloud timestamp clears the previous baseline instead of leaving a newer one in place.
func forceAppliedClientTimestamp(record CloudSyncRecord) error {
	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	entityID := syncRecordEntityID(record)
	query := db.Where("entity_type = ? AND entity_id = ? AND key = ?", record.EntityType, entityID, record.Key)
	if record.ClientTs <= 0 {
		return query.Delete(&database.CloudSyncRecordVersion{}).Error
	}
	var existing database.CloudSyncRecordVersion
	err := query.Take(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(&database.CloudSyncRecordVersion{
			EntityType: record.EntityType,
			EntityID:   entityID,
			Key:        record.Key,
			ClientTs:   record.ClientTs,
		}).Error
	}
	if err != nil {
		return err
	}
	if existing.ClientTs == record.ClientTs {
		return nil
	}
	return db.Model(&database.CloudSyncRecordVersion{}).
		Where("entity_type = ? AND entity_id = ? AND key = ?", record.EntityType, entityID, record.Key).
		Update("client_ts", record.ClientTs).Error
}

// loadPreApplyOplogs snapshots the oplogs cleanup may retire for one identity.
// Incremental sync only reads pending rows that lose the tie. Restore also reads
// synced history, in id pages, so a long history cannot exceed SQLite's variable limit.
func loadPreApplyOplogs(record CloudSyncRecord, restore bool) ([]preApplyOplog, error) {
	if !restore && record.ClientTs <= 0 {
		return nil, nil
	}
	db := database.GetDB()
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	entityID := syncRecordEntityID(record)
	batchSize := oplogIDBatchSize
	if batchSize < 1 {
		batchSize = 1
	}
	var preexisting []preApplyOplog
	var lastID uint
	for {
		query := db.Select("id", "timestamp", "value").
			Where("entity_type = ? AND entity_id = ? AND key = ? AND cloud_sync_discarded = ? AND id > ?", record.EntityType, entityID, record.Key, false, lastID)
		if !restore {
			query = query.Where("synced_to_cloud = ? AND timestamp <= ?", false, record.ClientTs)
		}
		query = query.Order("id asc").Limit(batchSize)
		var rows []database.Oplog
		if err := query.Find(&rows).Error; err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			preexisting = append(preexisting, preApplyOplog{
				ID:        row.ID,
				Timestamp: row.Timestamp,
				Value:     row.Value,
			})
			lastID = row.ID
		}
		if len(rows) < batchSize {
			break
		}
	}
	return preexisting, nil
}

// discardPreApplyOplogs retires oplogs that were already present before apply and
// were not rewritten by a reentrant callback. Restore includes synced rows so an
// older cloud baseline is not treated as stale by a previous local timestamp.
func discardPreApplyOplogs(record CloudSyncRecord, restore bool, preexisting []preApplyOplog) error {
	if len(preexisting) == 0 {
		return nil
	}
	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	batchSize := oplogIDBatchSize
	if batchSize < 1 {
		batchSize = 1
	}
	for start := 0; start < len(preexisting); start += batchSize {
		end := start + batchSize
		if end > len(preexisting) {
			end = len(preexisting)
		}
		chunk := preexisting[start:end]
		ids := make([]uint, 0, len(chunk))
		byID := make(map[uint]preApplyOplog, len(chunk))
		for _, row := range chunk {
			ids = append(ids, row.ID)
			byID[row.ID] = row
		}
		var current []database.Oplog
		if err := db.Select("id", "timestamp", "value", "synced_to_cloud", "cloud_sync_discarded").Where("id IN ?", ids).Find(&current).Error; err != nil {
			return err
		}
		discardIDs := make([]uint, 0, len(current))
		for _, row := range current {
			previous, ok := byID[row.ID]
			if !ok || row.CloudSyncDiscarded || row.Timestamp != previous.Timestamp || row.Value != previous.Value {
				continue
			}
			if !restore && (row.SyncedToCloud || row.Timestamp > record.ClientTs) {
				continue
			}
			discardIDs = append(discardIDs, row.ID)
		}
		if len(discardIDs) == 0 {
			continue
		}
		if err := db.Model(&database.Oplog{}).Where("id IN ?", discardIDs).Update("cloud_sync_discarded", true).Error; err != nil {
			return err
		}
	}
	return nil
}

// syncRecordEntityID matches the entity id used when the local oplog was written.
func syncRecordEntityID(record CloudSyncRecord) string {
	if record.EntityType == EntityPluginSetting {
		return record.PluginID
	}
	return record.Key
}

func logSkippedOlderRemoteChange(ctx context.Context, record CloudSyncRecord, localTs int64) {
	util.GetLogger().Info(ctx, fmt.Sprintf("skip cloud sync %s/%s: remote client_ts %d is older than local %d", record.EntityType, record.Key, record.ClientTs, localTs))
}
