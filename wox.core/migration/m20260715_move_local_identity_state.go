package migration

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"wox/database"
	"wox/util"

	"gorm.io/gorm"
)

const localIdentityStateID = 1

func init() {
	Register(&moveLocalIdentityStateMigration{})
}

type moveLocalIdentityStateMigration struct{}

func (m *moveLocalIdentityStateMigration) ID() string {
	return "20260715_move_local_identity_state"
}

func (m *moveLocalIdentityStateMigration) Description() string {
	return "Move local device identity and telemetry state from files into the database."
}

// Up preserves valid legacy state before the runtime stops reading the files.
func (m *moveLocalIdentityStateMigration) Up(ctx context.Context, tx *gorm.DB) error {
	location := util.GetLocation()
	devicePath := filepath.Join(location.GetWoxDataDirectory(), "device_id")
	telemetryPath := filepath.Join(location.GetWoxDataDirectory(), "telemetry_state.json")

	var identity database.DeviceIdentity
	if err := tx.First(&identity, localIdentityStateID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		if raw, readErr := os.ReadFile(devicePath); readErr == nil {
			if deviceID := strings.TrimSpace(string(raw)); deviceID != "" {
				if err := tx.Create(&database.DeviceIdentity{ID: localIdentityStateID, DeviceID: deviceID}).Error; err != nil {
					return err
				}
			}
		} else if !errors.Is(readErr, os.ErrNotExist) {
			util.GetLogger().Warn(ctx, "failed to read legacy device identity: "+readErr.Error())
		}
	} else if err != nil {
		return err
	}

	var telemetry database.TelemetryState
	if err := tx.First(&telemetry, localIdentityStateID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		if raw, readErr := os.ReadFile(telemetryPath); readErr == nil {
			var legacy struct {
				InstallID       string `json:"install_id"`
				LastSentAt      int64  `json:"last_sent_at"`
				LastSentVersion string `json:"last_sent_version"`
			}
			if err := json.Unmarshal(raw, &legacy); err != nil {
				util.GetLogger().Warn(ctx, "failed to parse legacy telemetry state: "+err.Error())
			} else if strings.TrimSpace(legacy.InstallID) != "" {
				if err := tx.Create(&database.TelemetryState{ID: localIdentityStateID, InstallID: legacy.InstallID, LastSentAt: legacy.LastSentAt, LastSentVersion: legacy.LastSentVersion}).Error; err != nil {
					return err
				}
			}
		} else if !errors.Is(readErr, os.ErrNotExist) {
			util.GetLogger().Warn(ctx, "failed to read legacy telemetry state: "+readErr.Error())
		}
	} else if err != nil {
		return err
	}

	return nil
}

// AfterCommit removes legacy files only after their database migration is durable.
func (m *moveLocalIdentityStateMigration) AfterCommit(ctx context.Context) error {
	location := util.GetLocation()
	for _, path := range []string{
		filepath.Join(location.GetWoxDataDirectory(), "device_id"),
		filepath.Join(location.GetWoxDataDirectory(), "telemetry_state.json"),
	} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	util.GetLogger().Info(ctx, "migrated local device identity and telemetry state into the database")
	return nil
}
