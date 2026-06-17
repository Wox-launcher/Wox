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
	recordKeysJSON, err := json.Marshal(record.Keys)
	if err != nil {
		return fmt.Errorf("failed to encode cloud sync history record keys: %w", err)
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
		RecordKeysJSON:   string(recordKeysJSON),
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
		record, err := decodeCloudSyncHistoryRow(row)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, nil
}

// Get returns one local sync history row by id for detail queries.
func (s *DefaultCloudSyncHistoryStore) Get(ctx context.Context, id uint) (*CloudSyncHistoryRecord, error) {
	_ = ctx

	db := database.GetDB()
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var row database.CloudSyncHistory
	if err := db.First(&row, id).Error; err != nil {
		return nil, err
	}

	record, err := decodeCloudSyncHistoryRow(row)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func decodeCloudSyncHistoryRow(row database.CloudSyncHistory) (CloudSyncHistoryRecord, error) {
	entityCounts := map[string]int{}
	if row.EntityCountsJSON != "" {
		if err := json.Unmarshal([]byte(row.EntityCountsJSON), &entityCounts); err != nil {
			return CloudSyncHistoryRecord{}, fmt.Errorf("failed to decode cloud sync history entity counts: %w", err)
		}
	}

	recordKeys := []CloudSyncRecordKey{}
	if row.RecordKeysJSON != "" {
		if err := json.Unmarshal([]byte(row.RecordKeysJSON), &recordKeys); err != nil {
			return CloudSyncHistoryRecord{}, fmt.Errorf("failed to decode cloud sync history record keys: %w", err)
		}
	}

	return CloudSyncHistoryRecord{
		ID:           row.ID,
		Operation:    row.Operation,
		Reason:       row.Reason,
		Status:       row.Status,
		StartedAt:    row.StartedAt,
		FinishedAt:   row.FinishedAt,
		DurationMs:   row.DurationMs,
		ItemCount:    row.ItemCount,
		EntityCounts: entityCounts,
		Keys:         recordKeys,
		Error:        row.Error,
	}, nil
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
