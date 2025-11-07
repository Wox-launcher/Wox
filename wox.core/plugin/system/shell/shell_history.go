package shell

import (
	"context"
	"fmt"
	"time"
	"wox/database"
	"wox/util"

	"gorm.io/gorm"
)

// ShellHistory represents a shell command execution history record
type ShellHistory struct {
	ID          string `gorm:"primaryKey"`
	Command     string `gorm:"not null;index"`
	Interpreter string `gorm:"not null"`
	Output      string `gorm:"type:text"` // Store output as text
	ExitCode    int
	Status      string // running, completed, failed, killed
	StartTime   int64  `gorm:"not null;index"`
	EndTime     int64
	Duration    int64 // Duration in milliseconds
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ShellHistoryManager manages shell command history in database
type ShellHistoryManager struct {
	db *gorm.DB
}

// NewShellHistoryManager creates a new shell history manager
func NewShellHistoryManager() *ShellHistoryManager {
	return &ShellHistoryManager{
		db: database.GetDB(),
	}
}

// Init initializes the shell history table
func (m *ShellHistoryManager) Init(ctx context.Context) error {
	err := m.db.AutoMigrate(&ShellHistory{})
	if err != nil {
		return fmt.Errorf("failed to migrate shell history table: %w", err)
	}
	return nil
}

// Create creates a new shell history record
func (m *ShellHistoryManager) Create(ctx context.Context, record *ShellHistory) error {
	return m.db.WithContext(ctx).Create(record).Error
}

// Update updates an existing shell history record
func (m *ShellHistoryManager) Update(ctx context.Context, record *ShellHistory) error {
	return m.db.WithContext(ctx).Save(record).Error
}

// UpdateOutput updates only the output field (for performance)
func (m *ShellHistoryManager) UpdateOutput(ctx context.Context, id string, output string) error {
	return m.db.WithContext(ctx).Model(&ShellHistory{}).
		Where("id = ?", id).
		Update("output", output).Error
}

// UpdateStatus updates the status and related fields
func (m *ShellHistoryManager) UpdateStatus(ctx context.Context, id string, status string, exitCode int, endTime int64, duration int64) error {
	return m.db.WithContext(ctx).Model(&ShellHistory{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":    status,
			"exit_code": exitCode,
			"end_time":  endTime,
			"duration":  duration,
		}).Error
}

// ResetForReexecute resets an existing record to a fresh running state for re-execution
func (m *ShellHistoryManager) ResetForReexecute(ctx context.Context, id string, command string, interpreter string, startTime int64) error {
	return m.db.WithContext(ctx).Model(&ShellHistory{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":      "running",
			"exit_code":   0,
			"output":      "",
			"start_time":  startTime,
			"end_time":    0,
			"duration":    0,
			"command":     command,
			"interpreter": interpreter,
		}).Error
}

// GetByID retrieves a shell history record by ID
func (m *ShellHistoryManager) GetByID(ctx context.Context, id string) (*ShellHistory, error) {
	var record ShellHistory
	err := m.db.WithContext(ctx).Where("id = ?", id).First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// List retrieves shell history records with pagination
func (m *ShellHistoryManager) List(ctx context.Context, limit int, offset int) ([]ShellHistory, error) {
	var records []ShellHistory
	err := m.db.WithContext(ctx).
		Order("start_time DESC").
		Limit(limit).
		Offset(offset).
		Find(&records).Error
	return records, err
}

// Search searches shell history by command
func (m *ShellHistoryManager) Search(ctx context.Context, keyword string, limit int) ([]ShellHistory, error) {
	var records []ShellHistory
	err := m.db.WithContext(ctx).
		Where("command LIKE ?", "%"+keyword+"%").
		Order("start_time DESC").
		Limit(limit).
		Find(&records).Error
	return records, err
}

// Delete deletes a shell history record by ID
func (m *ShellHistoryManager) Delete(ctx context.Context, id string) error {
	return m.db.WithContext(ctx).Delete(&ShellHistory{}, "id = ?", id).Error
}

// DeleteOldRecords deletes records older than the specified timestamp
func (m *ShellHistoryManager) DeleteOldRecords(ctx context.Context, beforeTimestamp int64) (int64, error) {
	result := m.db.WithContext(ctx).
		Where("start_time < ?", beforeTimestamp).
		Delete(&ShellHistory{})
	return result.RowsAffected, result.Error
}

// EnforceMaxCount ensures the total number of records doesn't exceed maxCount
func (m *ShellHistoryManager) EnforceMaxCount(ctx context.Context, maxCount int) (int64, error) {
	// First, count total records
	var totalCount int64
	err := m.db.WithContext(ctx).Model(&ShellHistory{}).Count(&totalCount).Error
	if err != nil {
		return 0, err
	}

	if totalCount <= int64(maxCount) {
		return 0, nil // No need to delete anything
	}

	// Get IDs of oldest records to delete
	var idsToDelete []string
	deleteCount := int(totalCount) - maxCount
	err = m.db.WithContext(ctx).
		Model(&ShellHistory{}).
		Order("start_time ASC").
		Limit(deleteCount).
		Pluck("id", &idsToDelete).Error
	if err != nil {
		return 0, err
	}

	// Delete the oldest records
	result := m.db.WithContext(ctx).
		Where("id IN ?", idsToDelete).
		Delete(&ShellHistory{})
	return result.RowsAffected, result.Error
}

// GetRecentCommands gets unique recent commands for autocomplete
func (m *ShellHistoryManager) GetRecentCommands(ctx context.Context, limit int) ([]string, error) {
	var commands []string
	err := m.db.WithContext(ctx).
		Model(&ShellHistory{}).
		Distinct("command").
		Order("start_time DESC").
		Limit(limit).
		Pluck("command", &commands).Error
	return commands, err
}

// GetRecentHistory gets recent history records with full details
func (m *ShellHistoryManager) GetRecentHistory(ctx context.Context, limit int) ([]*ShellHistory, error) {
	var histories []*ShellHistory
	err := m.db.WithContext(ctx).
		Order("start_time DESC").
		Limit(limit).
		Find(&histories).Error
	return histories, err
}

// shellHistoryTracker tracks a running command and periodically saves output
type shellHistoryTracker struct {
	manager       *ShellHistoryManager
	historyID     string
	state         *shellExecutionState
	stopChan      chan struct{}
	saveInterval  time.Duration
	lastSavedSize int
}

// newShellHistoryTracker creates a new history tracker
func newShellHistoryTracker(manager *ShellHistoryManager, historyID string, state *shellExecutionState) *shellHistoryTracker {
	return &shellHistoryTracker{
		manager:      manager,
		historyID:    historyID,
		state:        state,
		stopChan:     make(chan struct{}),
		saveInterval: 1 * time.Second, // Save every 1 second
	}
}

// start begins tracking and periodic saving
func (t *shellHistoryTracker) start(ctx context.Context) {
	util.Go(ctx, "shell history tracker", func() {
		ticker := time.NewTicker(t.saveInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				t.saveOutput(ctx)
			case <-t.stopChan:
				// Final save when stopped
				t.saveOutput(ctx)
				return
			case <-ctx.Done():
				return
			}
		}
	})
}

// saveOutput saves the current output to database
func (t *shellHistoryTracker) saveOutput(ctx context.Context) {
	t.state.mutex.RLock()
	currentOutput := t.state.output.String()
	currentSize := len(currentOutput)
	t.state.mutex.RUnlock()

	// Only save if output has changed
	if currentSize > t.lastSavedSize {
		err := t.manager.UpdateOutput(ctx, t.historyID, currentOutput)
		if err != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("Failed to save shell history output: %s", err.Error()))
		} else {
			t.lastSavedSize = currentSize
		}
	}
}

// stop stops the tracker and performs final save
func (t *shellHistoryTracker) stop(ctx context.Context, status string, exitCode int) {
	close(t.stopChan)

	// Calculate duration
	t.state.mutex.RLock()
	startTime := t.state.startTime
	endTime := t.state.endTime
	t.state.mutex.RUnlock()

	duration := endTime.Sub(startTime).Milliseconds()

	// Update final status
	err := t.manager.UpdateStatus(ctx, t.historyID, status, exitCode, endTime.UnixMilli(), duration)
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("Failed to update shell history status: %s", err.Error()))
	}
}
