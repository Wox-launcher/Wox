package setting

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
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

// GetMRUItems retrieves MRU items sorted by usage with smart scoring
func (m *MRUManager) GetMRUItems(ctx context.Context, limit int) ([]MRUItem, error) {
	var records []database.MRURecord

	// Only return items with use_count >= 3 to ensure quality
	err := m.db.Where("use_count >= ?", 3).Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query MRU records: %w", err)
	}

	// Calculate smart scores for each record
	type scoredRecord struct {
		record database.MRURecord
		score  int64
	}

	scoredRecords := make([]scoredRecord, 0, len(records))
	currentTimestamp := util.GetSystemTimestamp()

	for _, record := range records {
		score := m.calculateMRUScore(record, currentTimestamp)
		scoredRecords = append(scoredRecords, scoredRecord{
			record: record,
			score:  score,
		})
	}

	// Sort by score descending
	sort.Slice(scoredRecords, func(i, j int) bool {
		return scoredRecords[i].score > scoredRecords[j].score
	})

	// Apply limit
	if limit > 0 && len(scoredRecords) > limit {
		scoredRecords = scoredRecords[:limit]
	}

	items := make([]MRUItem, 0, len(scoredRecords))
	for _, sr := range scoredRecords {
		record := sr.record
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

// calculateMRUScore calculates a smart score for MRU items based on usage patterns
// This algorithm is inspired by calculateResultScore in plugin/manager.go
func (m *MRUManager) calculateMRUScore(record database.MRURecord, currentTimestamp int64) int64 {
	var score int64 = 0

	// Base score from use count (logarithmic scaling to prevent dominance)
	// Use count of 3-10: score 10-30, 11-50: score 35-70, 51+: score 75+
	useCountScore := int64(math.Log(float64(record.UseCount)) * 15)
	score += useCountScore

	// Time-based scoring using fibonacci sequence (similar to calculateResultScore)
	// More recent usage gets higher weight
	hours := (currentTimestamp - record.LastUsed) / 1000 / 60 / 60
	if hours < 24*7 { // Within 7 days
		fibonacciIndex := int(math.Ceil(float64(hours) / 24))
		if fibonacciIndex > 7 {
			fibonacciIndex = 7
		}
		if fibonacciIndex < 1 {
			fibonacciIndex = 1
		}
		fibonacci := []int64{5, 8, 13, 21, 34, 55, 89}
		score += fibonacci[7-fibonacciIndex]
	} else if hours < 24*30 { // Within 30 days but older than 7 days
		score += 3 // Small bonus for recent but not very recent usage
	}
	// Items older than 30 days get no time bonus

	// Frequency bonus: items used more frequently get higher scores
	// Calculate average usage frequency (uses per day since creation)
	daysSinceCreation := (currentTimestamp - record.CreatedAt.Unix()*1000) / 1000 / 60 / 60 / 24
	if daysSinceCreation > 0 {
		frequencyScore := int64(float64(record.UseCount) / float64(daysSinceCreation) * 10)
		if frequencyScore > 50 { // Cap frequency bonus
			frequencyScore = 50
		}
		score += frequencyScore
	}

	return score
}
