package cloudsync

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"wox/database"
	"wox/util"

	"gorm.io/gorm"
)

const defaultCloudSyncHistoryRetentionLimit = 200

var cloudSyncHistoryMu sync.Mutex

// DefaultCloudSyncHistoryStore writes local sync history to the main Wox database.
type DefaultCloudSyncHistoryStore struct {
	RetentionLimit int
}

// NewDefaultCloudSyncHistoryStore creates the local history store used by the sync manager and launcher plugin.
func NewDefaultCloudSyncHistoryStore() *DefaultCloudSyncHistoryStore {
	return &DefaultCloudSyncHistoryStore{RetentionLimit: defaultCloudSyncHistoryRetentionLimit}
}

// Record persists one local sync attempt and trims old rows so the history stays bounded.
func (s *DefaultCloudSyncHistoryStore) Record(ctx context.Context, record CloudSyncHistoryRecord) error {
	cloudSyncHistoryMu.Lock()
	defer cloudSyncHistoryMu.Unlock()

	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	if record.FinishedAt == 0 {
		record.FinishedAt = util.GetSystemTimestamp()
	}
	if record.StartedAt == 0 {
		record.StartedAt = record.FinishedAt
	}
	if record.DurationMs < 0 {
		record.DurationMs = 0
	}

	entityCounts := record.EntityCounts
	if entityCounts == nil {
		entityCounts = map[string]int{}
	}
	entityCountsJSON, err := json.Marshal(entityCounts)
	if err != nil {
		return fmt.Errorf("failed to encode cloud sync history entity counts: %w", err)
	}

	row := database.CloudSyncHistory{
		Operation:        record.Operation,
		Reason:           record.Reason,
		Status:           record.Status,
		StartedAt:        record.StartedAt,
		FinishedAt:       record.FinishedAt,
		DurationMs:       record.DurationMs,
		ItemCount:        record.ItemCount,
		EntityCountsJSON: string(entityCountsJSON),
		Error:            record.Error,
	}
	if err := db.Create(&row).Error; err != nil {
		return err
	}

	return s.trimLocked(db)
}

// ListRecent returns the newest local sync history rows first.
func (s *DefaultCloudSyncHistoryStore) ListRecent(ctx context.Context, limit int) ([]CloudSyncHistoryRecord, error) {
	_ = ctx

	db := database.GetDB()
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if limit <= 0 {
		limit = defaultCloudSyncHistoryRetentionLimit
	}

	var rows []database.CloudSyncHistory
	if err := db.Order("started_at DESC, id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}

	records := make([]CloudSyncHistoryRecord, 0, len(rows))
	for _, row := range rows {
		entityCounts := map[string]int{}
		if row.EntityCountsJSON != "" {
			if err := json.Unmarshal([]byte(row.EntityCountsJSON), &entityCounts); err != nil {
				return nil, fmt.Errorf("failed to decode cloud sync history entity counts: %w", err)
			}
		}

		records = append(records, CloudSyncHistoryRecord{
			ID:           row.ID,
			Operation:    row.Operation,
			Reason:       row.Reason,
			Status:       row.Status,
			StartedAt:    row.StartedAt,
			FinishedAt:   row.FinishedAt,
			DurationMs:   row.DurationMs,
			ItemCount:    row.ItemCount,
			EntityCounts: entityCounts,
			Error:        row.Error,
		})
	}

	return records, nil
}

func (s *DefaultCloudSyncHistoryStore) trimLocked(db *gorm.DB) error {
	limit := s.RetentionLimit
	if limit <= 0 {
		limit = defaultCloudSyncHistoryRetentionLimit
	}

	var oldIDs []uint
	if err := db.Model(&database.CloudSyncHistory{}).Order("started_at DESC, id DESC").Offset(limit).Pluck("id", &oldIDs).Error; err != nil {
		return err
	}
	if len(oldIDs) == 0 {
		return nil
	}

	return db.Delete(&database.CloudSyncHistory{}, oldIDs).Error
}
