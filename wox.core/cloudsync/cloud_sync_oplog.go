package cloudsync

import (
	"context"
	"fmt"
	"time"
	"wox/database"
	"wox/util"

	"gorm.io/gorm"
)

type DefaultOplogStore struct{}

func NewDefaultOplogStore() *DefaultOplogStore {
	return &DefaultOplogStore{}
}

func (s *DefaultOplogStore) LoadPending(ctx context.Context, limit int) ([]database.Oplog, error) {
	started := time.Now()
	defer func() {
		util.GetLogger().Debug(ctx, fmt.Sprintf("cloud_sync_timing stage=load_pending limit=%d costMs=%d", limit, time.Since(started).Milliseconds()))
	}()
	db := database.GetDB()
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var oplogs []database.Oplog
	query := pendingCloudSyncOplogs(db.WithContext(ctx)).Order("id asc")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&oplogs).Error; err != nil {
		return nil, err
	}

	return oplogs, nil
}

// CountPending returns the current number of due local oplogs waiting for cloud upload.
func (s *DefaultOplogStore) CountPending(ctx context.Context) (int, error) {
	started := time.Now()
	defer func() {
		util.GetLogger().Debug(ctx, fmt.Sprintf("cloud_sync_timing stage=count_pending costMs=%d", time.Since(started).Milliseconds()))
	}()
	db := database.GetDB()
	if db == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	var count int64
	if err := pendingCloudSyncOplogs(db.WithContext(ctx)).Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

// pendingCloudSyncOplogs selects the latest state per identity before applying timing and batch limits.
func pendingCloudSyncOplogs(db *gorm.DB) *gorm.DB {
	// Include delayed, synced and discarded rows in ranking: retiring the latest
	// state must never make an older value eligible for upload again.
	ranked := db.Model(&database.Oplog{}).Select(`id, ROW_NUMBER() OVER (
		PARTITION BY entity_type, entity_id, key ORDER BY timestamp DESC, id DESC
	) AS sync_rank`)
	latest := db.Table("(?) AS latest_oplogs", ranked).Select("id").Where("sync_rank = 1")
	return db.Model(&database.Oplog{}).Where("id IN (?)", latest).
		Where("synced_to_cloud = ? AND cloud_sync_discarded = ? AND (sync_after IS NULL OR sync_after = 0 OR sync_after <= ?)", false, false, util.GetSystemTimestamp())
}

func (s *DefaultOplogStore) MarkSynced(ctx context.Context, ids []uint) error {
	_ = ctx
	if len(ids) == 0 {
		return nil
	}

	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	return db.Model(&database.Oplog{}).Where("id IN ?", ids).Updates(map[string]interface{}{
		"synced_to_cloud":              true,
		"cloud_sync_push_failed_count": 0,
		"cloud_sync_last_push_error":   "",
	}).Error
}

// MarkPushFailed records per-oplog rejection state so one bad row does not block later rows forever.
func (s *DefaultOplogStore) MarkPushFailed(ctx context.Context, failures []CloudSyncOplogPushFailure) error {
	_ = ctx
	if len(failures) == 0 {
		return nil
	}

	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for _, failure := range failures {
			if failure.ID == 0 {
				continue
			}
			if err := tx.Model(&database.Oplog{}).Where("id = ?", failure.ID).Updates(map[string]interface{}{
				"cloud_sync_push_failed_count": failure.FailedCount,
				"cloud_sync_last_push_error":   failure.LastError,
				"cloud_sync_discarded":         failure.Discarded,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
