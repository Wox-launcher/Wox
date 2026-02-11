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
	ID            string `gorm:"primaryKey"`
	SessionID     string `gorm:"not null;index"`
	Command       string `gorm:"not null;index"`
	Interpreter   string `gorm:"not null"`
	OutputSummary string `gorm:"type:text"`
	OutputPath    string `gorm:"type:text"`
	ExitCode      int
	Status        string // running, completed, failed, killed
	StartTime     int64  `gorm:"not null;index"`
	EndTime       int64
	Duration      int64 // Duration in milliseconds
	CreatedAt     time.Time
	UpdatedAt     time.Time
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
	migrator := m.db.Migrator()

	// No backward compatibility is required for shell history.
	// If required columns are missing in existing table, recreate it with new schema.
	if migrator.HasTable(&ShellHistory{}) {
		requiredColumns := []string{"session_id", "output_summary", "output_path"}
		var missingColumns []string
		for _, column := range requiredColumns {
			if !migrator.HasColumn(&ShellHistory{}, column) {
				missingColumns = append(missingColumns, column)
			}
		}
		if len(missingColumns) > 0 {
			util.GetLogger().Warn(ctx, fmt.Sprintf("shell history schema missing columns %v, recreating table", missingColumns))
			if dropErr := migrator.DropTable(&ShellHistory{}); dropErr != nil {
				return fmt.Errorf("failed to recreate shell history table: %w", dropErr)
			}
		}
	}

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

// UpdateOutputSummary updates lightweight summary fields.
func (m *ShellHistoryManager) UpdateOutputSummary(ctx context.Context, id string, summary string, outputPath string) error {
	return m.db.WithContext(ctx).Model(&ShellHistory{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"output_summary": summary,
			"output_path":    outputPath,
		}).Error
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
func (m *ShellHistoryManager) ResetForReexecute(ctx context.Context, id string, sessionID string, command string, interpreter string, startTime int64, outputPath string) error {
	return m.db.WithContext(ctx).Model(&ShellHistory{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":         "running",
			"exit_code":      0,
			"session_id":     sessionID,
			"output_summary": "",
			"output_path":    outputPath,
			"start_time":     startTime,
			"end_time":       0,
			"duration":       0,
			"command":        command,
			"interpreter":    interpreter,
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

func (m *ShellHistoryManager) GetBySessionID(ctx context.Context, sessionID string) (*ShellHistory, error) {
	var record ShellHistory
	err := m.db.WithContext(ctx).Where("session_id = ?", sessionID).First(&record).Error
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

func (m *ShellHistoryManager) DeleteBySessionID(ctx context.Context, sessionID string) error {
	return m.db.WithContext(ctx).Delete(&ShellHistory{}, "session_id = ?", sessionID).Error
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
	outputPath    string
	stopChan      chan struct{}
	saveInterval  time.Duration
	lastSavedHash uint64
}

// newShellHistoryTracker creates a new history tracker
func newShellHistoryTracker(manager *ShellHistoryManager, historyID string, state *shellExecutionState, outputPath string) *shellHistoryTracker {
	return &shellHistoryTracker{
		manager:      manager,
		historyID:    historyID,
		state:        state,
		outputPath:   outputPath,
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
	currentOutput := t.state.summaryOutput
	t.state.mutex.RUnlock()

	hash := fnv1a(currentOutput)
	if hash == t.lastSavedHash {
		return
	}

	err := t.manager.UpdateOutputSummary(ctx, t.historyID, currentOutput, t.outputPath)
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("Failed to save shell history summary: %s", err.Error()))
	} else {
		t.lastSavedHash = hash
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

func fnv1a(s string) uint64 {
	const (
		offset64 uint64 = 1469598103934665603
		prime64  uint64 = 1099511628211
	)

	hash := offset64
	for i := 0; i < len(s); i++ {
		hash ^= uint64(s[i])
		hash *= prime64
	}
	return hash
}
