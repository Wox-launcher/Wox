package cloudsync

import (
	"context"
	"fmt"
	"wox/database"
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

	var oplogs []database.Oplog
	query := db.Where("synced_to_cloud = ?", false).Order("id asc")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&oplogs).Error; err != nil {
		return nil, err
	}

	return oplogs, nil
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

	return db.Model(&database.Oplog{}).Where("id IN ?", ids).Update("synced_to_cloud", true).Error
}
