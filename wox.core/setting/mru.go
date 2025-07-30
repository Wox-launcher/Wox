package setting

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"wox/common"
	"wox/database"
	"wox/util"

	"gorm.io/gorm"
)

// MRUItem represents a Most Recently Used item
type MRUItem struct {
	PluginID    string          `json:"pluginId"`
	Title       string          `json:"title"`
	SubTitle    string          `json:"subTitle"`
	Icon        common.WoxImage `json:"icon"`
	ContextData string          `json:"contextData"`
	LastUsed    int64           `json:"lastUsed"`
	UseCount    int             `json:"useCount"`
}

// MRUManager manages Most Recently Used items
type MRUManager struct {
	db *gorm.DB
}

// NewMRUManager creates a new MRU manager
func NewMRUManager(db *gorm.DB) *MRUManager {
	return &MRUManager{db: db}
}

// AddMRUItem adds or updates an MRU item
func (m *MRUManager) AddMRUItem(ctx context.Context, item MRUItem) error {
	hash := NewResultHash(item.PluginID, item.Title, item.SubTitle)
	
	// Serialize icon to JSON
	iconData, err := json.Marshal(item.Icon)
	if err != nil {
		return fmt.Errorf("failed to serialize icon: %w", err)
	}

	now := time.Now()
	timestamp := util.GetSystemTimestamp()

	// Check if record exists
	var existingRecord database.MRURecord
	err = m.db.Where("hash = ?", string(hash)).First(&existingRecord).Error
	
	if err == gorm.ErrRecordNotFound {
		// Create new record
		record := database.MRURecord{
			Hash:        string(hash),
			PluginID:    item.PluginID,
			Title:       item.Title,
			SubTitle:    item.SubTitle,
			Icon:        string(iconData),
			ContextData: item.ContextData,
			LastUsed:    timestamp,
			UseCount:    1,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		return m.db.Create(&record).Error
	} else if err != nil {
		return fmt.Errorf("failed to query MRU record: %w", err)
	} else {
		// Update existing record
		updates := map[string]interface{}{
			"last_used":    timestamp,
			"use_count":    existingRecord.UseCount + 1,
			"context_data": item.ContextData, // Update context data in case it changed
			"icon":         string(iconData), // Update icon in case it changed
			"updated_at":   now,
		}
		return m.db.Model(&existingRecord).Updates(updates).Error
	}
}

// GetMRUItems retrieves MRU items sorted by usage
func (m *MRUManager) GetMRUItems(ctx context.Context, limit int) ([]MRUItem, error) {
	var records []database.MRURecord
	
	// Order by last_used DESC, then by use_count DESC for items with same last_used time
	err := m.db.Order("last_used DESC, use_count DESC").Limit(limit).Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query MRU records: %w", err)
	}

	items := make([]MRUItem, 0, len(records))
	for _, record := range records {
		// Deserialize icon
		var icon common.WoxImage
		if err := json.Unmarshal([]byte(record.Icon), &icon); err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("failed to deserialize icon for MRU item %s: %s", record.Hash, err.Error()))
			icon = common.WoxImage{} // Use empty icon as fallback
		}

		items = append(items, MRUItem{
			PluginID:    record.PluginID,
			Title:       record.Title,
			SubTitle:    record.SubTitle,
			Icon:        icon,
			ContextData: record.ContextData,
			LastUsed:    record.LastUsed,
			UseCount:    record.UseCount,
		})
	}

	return items, nil
}

// RemoveMRUItem removes an MRU item by hash
func (m *MRUManager) RemoveMRUItem(ctx context.Context, pluginID, title, subTitle string) error {
	hash := NewResultHash(pluginID, title, subTitle)
	result := m.db.Where("hash = ?", string(hash)).Delete(&database.MRURecord{})
	if result.Error != nil {
		return fmt.Errorf("failed to remove MRU item: %w", result.Error)
	}
	
	util.GetLogger().Debug(ctx, fmt.Sprintf("removed MRU item: %s", hash))
	return nil
}

// CleanupOldMRUItems removes old MRU items to keep the database size manageable
func (m *MRUManager) CleanupOldMRUItems(ctx context.Context, keepCount int) error {
	// Keep only the most recent keepCount items
	subQuery := m.db.Model(&database.MRURecord{}).
		Select("hash").
		Order("last_used DESC, use_count DESC").
		Limit(keepCount)

	result := m.db.Where("hash NOT IN (?)", subQuery).Delete(&database.MRURecord{})
	if result.Error != nil {
		return fmt.Errorf("failed to cleanup old MRU items: %w", result.Error)
	}

	if result.RowsAffected > 0 {
		util.GetLogger().Info(ctx, fmt.Sprintf("cleaned up %d old MRU items", result.RowsAffected))
	}

	return nil
}

// GetMRUCount returns the total number of MRU items
func (m *MRUManager) GetMRUCount(ctx context.Context) (int64, error) {
	var count int64
	err := m.db.Model(&database.MRURecord{}).Count(&count).Error
	return count, err
}
