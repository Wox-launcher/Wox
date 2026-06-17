package cloudsync

import (
	"context"
	"fmt"
	"wox/database"
	"wox/util"

	"gorm.io/gorm"
)

type DefaultOplogStore struct{}

func NewDefaultOplogStore() *DefaultOplogStore {
	return &DefaultOplogStore{}
}

func (s *DefaultOplogStore) LoadPending(ctx context.Context, limit int) ([]database.Oplog, error) {
	_ = ctx
	db := database.GetDB()
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	now := util.GetSystemTimestamp()
	var oplogs []database.Oplog
	query := db.Where("synced_to_cloud = ? AND cloud_sync_discarded = ? AND (sync_after IS NULL OR sync_after = 0 OR sync_after <= ?)", false, false, now).Order("id asc")
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
	_ = ctx
	db := database.GetDB()
	if db == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	now := util.GetSystemTimestamp()
	var count int64
	if err := db.Model(&database.Oplog{}).Where("synced_to_cloud = ? AND cloud_sync_discarded = ? AND (sync_after IS NULL OR sync_after = 0 OR sync_after <= ?)", false, false, now).Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
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
