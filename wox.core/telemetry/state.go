package telemetry

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"wox/util"

	"github.com/google/uuid"
)

const (
	telemetryFileName = "telemetry_state.json"
	schemaVersion     = 1
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
	filePath := s.getFilePath()
	if !util.IsFileExists(filePath) {
		s.InstallID = uuid.New().String()
		s.LastSentAt = 0
		s.LastSentVersion = ""
		if err := s.save(); err != nil {
			util.GetLogger().Warn(context.Background(), "failed to save initial telemetry state: "+err.Error())
		}
		return
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		util.GetLogger().Warn(context.Background(), "failed to read telemetry state, creating new: "+err.Error())
		s.InstallID = uuid.New().String()
		s.LastSentAt = 0
		s.LastSentVersion = ""
		return
	}

	var state TelemetryState
	if err := json.Unmarshal(data, &state); err != nil {
		util.GetLogger().Warn(context.Background(), "failed to parse telemetry state, creating new: "+err.Error())
		s.InstallID = uuid.New().String()
		s.LastSentAt = 0
		s.LastSentVersion = ""
		return
	}

	s.InstallID = state.InstallID
	s.LastSentAt = state.LastSentAt
	s.LastSentVersion = state.LastSentVersion
}

func (s *TelemetryState) save() error {
	filePath := s.getFilePath()
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.Marshal(s)
	if err != nil {
		return err
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return err
	}

	return nil
}

func (s *TelemetryState) getFilePath() string {
	return filepath.Join(util.GetLocation().GetWoxDataDirectory(), telemetryFileName)
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
	s.InstallID = uuid.New().String()
	s.LastSentAt = 0
	s.LastSentVersion = ""
	if err := s.save(); err != nil {
		util.GetLogger().Warn(context.Background(), "failed to save telemetry state after reset: "+err.Error())
	}
}

// Delete removes the telemetry state file and resets the singleton.
// After calling Delete, the next call to GetTelemetryState() will create a new InstallID.
func DeleteTelemetryState(ctx context.Context) {
	telemetryStateMutex.Lock()
	defer telemetryStateMutex.Unlock()

	if telemetryStateInstance != nil {
		telemetryStateInstance.deleteFile()
		telemetryStateInstance = nil
	}
	util.GetLogger().Info(ctx, "telemetry state deleted")
}

func (s *TelemetryState) deleteFile() {
	filePath := s.getFilePath()
	os.Remove(filePath)
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
