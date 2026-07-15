package telemetry

import (
	"context"
	"errors"
	"sync"
	"wox/database"
	"wox/util"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	telemetryStateID = 1
	schemaVersion    = 1
)

type TelemetryState struct {
	InstallID       string `json:"install_id"`
	LastSentAt      int64  `json:"last_sent_at"`
	LastSentVersion string `json:"last_sent_version"`
}

var (
	telemetryStateInstance *TelemetryState
	telemetryStateMutex    sync.Mutex
)

func GetTelemetryState() *TelemetryState {
	telemetryStateMutex.Lock()
	defer telemetryStateMutex.Unlock()

	if telemetryStateInstance == nil {
		telemetryStateInstance = &TelemetryState{}
		telemetryStateInstance.load()
	}
	return telemetryStateInstance
}

func (s *TelemetryState) load() {
	db := database.GetDB()
	if db != nil {
		var state database.TelemetryState
		err := db.First(&state, telemetryStateID).Error
		if err == nil && state.InstallID != "" {
			s.InstallID = state.InstallID
			s.LastSentAt = state.LastSentAt
			s.LastSentVersion = state.LastSentVersion
			return
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			util.GetLogger().Warn(context.Background(), "failed to load telemetry state: "+err.Error())
		}
	}

	s.InstallID = uuid.NewString()
	s.LastSentAt = 0
	s.LastSentVersion = ""
	if err := s.save(); err != nil {
		util.GetLogger().Warn(context.Background(), "failed to save initial telemetry state: "+err.Error())
	}
}

func (s *TelemetryState) save() error {
	db := database.GetDB()
	if db == nil {
		return errors.New("database not initialized")
	}
	return db.Save(&database.TelemetryState{
		ID:              telemetryStateID,
		InstallID:       s.InstallID,
		LastSentAt:      s.LastSentAt,
		LastSentVersion: s.LastSentVersion,
	}).Error
}

func (s *TelemetryState) UpdateLastSent(version string, timestamp int64) {
	s.LastSentAt = timestamp
	s.LastSentVersion = version
	if err := s.save(); err != nil {
		util.GetLogger().Warn(context.Background(), "failed to save telemetry state after update: "+err.Error())
	}
}

func (s *TelemetryState) ShouldSendPresence(currentVersion string, intervalHours int) bool {
	if s.InstallID == "" {
		return false
	}
	if s.LastSentAt == 0 {
		return true
	}
	if s.LastSentVersion != currentVersion {
		return true
	}
	hoursSinceLastSent := (util.GetSystemTimestamp() - s.LastSentAt) / (1000 * 60 * 60)
	return hoursSinceLastSent >= int64(intervalHours)
}

func (s *TelemetryState) Reset() {
	s.InstallID = uuid.NewString()
	s.LastSentAt = 0
	s.LastSentVersion = ""
	if err := s.save(); err != nil {
		util.GetLogger().Warn(context.Background(), "failed to save telemetry state after reset: "+err.Error())
	}
}

// Delete removes persisted telemetry state and resets the singleton.
// After calling Delete, the next call to GetTelemetryState() will create a new InstallID.
func DeleteTelemetryState(ctx context.Context) {
	telemetryStateMutex.Lock()
	defer telemetryStateMutex.Unlock()

	if db := database.GetDB(); db != nil {
		if err := db.Delete(&database.TelemetryState{}, telemetryStateID).Error; err != nil {
			util.GetLogger().Warn(ctx, "failed to delete telemetry state: "+err.Error())
		}
	}
	telemetryStateInstance = nil
	util.GetLogger().Info(ctx, "telemetry state deleted")
}

// ResetTelemetryState resets the in-memory state and generates a new InstallID.
func ResetTelemetryState(ctx context.Context) {
	telemetryStateMutex.Lock()
	defer telemetryStateMutex.Unlock()

	if telemetryStateInstance == nil {
		telemetryStateInstance = &TelemetryState{}
	}
	telemetryStateInstance.Reset()
	util.GetLogger().Info(ctx, "telemetry state reset")
}
