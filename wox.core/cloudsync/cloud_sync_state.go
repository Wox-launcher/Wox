package cloudsync

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"wox/database"

	"gorm.io/gorm"
)

const cloudSyncStateID = 1

var cloudSyncStateMu sync.Mutex

func LoadCloudSyncState(ctx context.Context) (*database.CloudSyncState, error) {
	cloudSyncStateMu.Lock()
	defer cloudSyncStateMu.Unlock()

	db := database.GetDB()
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var state database.CloudSyncState
	err := db.First(&state, cloudSyncStateID).Error
	if err == nil {
		return &state, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	state = database.CloudSyncState{ID: cloudSyncStateID}
	if err := db.Create(&state).Error; err != nil {
		return nil, err
	}

	return &state, nil
}

func SaveCloudSyncState(ctx context.Context, state *database.CloudSyncState) error {
	cloudSyncStateMu.Lock()
	defer cloudSyncStateMu.Unlock()

	if state == nil {
		return fmt.Errorf("cloud sync state is nil")
	}

	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	if state.ID == 0 {
		state.ID = cloudSyncStateID
	}

	return db.Save(state).Error
}

// ResetCloudSyncState clears account-scoped sync progress so the next login starts from a clean bootstrap state.
func ResetCloudSyncState(ctx context.Context) error {
	if err := clearCloudSyncRecordVersions(); err != nil {
		return err
	}
	return SaveCloudSyncState(ctx, &database.CloudSyncState{ID: cloudSyncStateID})
}

// clearCloudSyncRecordVersions drops last-write-wins baselines. They are not account-scoped,
// so leaving them after logout would hide the next account's older records.
func clearCloudSyncRecordVersions() error {
	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	return db.Where("1 = 1").Delete(&database.CloudSyncRecordVersion{}).Error
}

func UpdateCloudSyncState(ctx context.Context, update func(state *database.CloudSyncState)) (*database.CloudSyncState, error) {
	state, err := LoadCloudSyncState(ctx)
	if err != nil {
		return nil, err
	}

	if update != nil {
		update(state)
	}

	if err := SaveCloudSyncState(ctx, state); err != nil {
		return nil, err
	}

	return state, nil
}

// MarkCloudSyncBootstrapPending keeps the UI in a syncing state until background bootstrap work completes.
func MarkCloudSyncBootstrapPending(ctx context.Context) {
	_, _ = UpdateCloudSyncState(ctx, func(state *database.CloudSyncState) {
		state.Bootstrapped = false
		state.LastError = ""
		state.BackoffUntil = 0
		state.RetryCount = 0
	})
}

// MarkCloudSyncBootstrapComplete marks bootstrap complete after background restore or initial push finishes.
func MarkCloudSyncBootstrapComplete(ctx context.Context) {
	_, _ = UpdateCloudSyncState(ctx, func(state *database.CloudSyncState) {
		state.Bootstrapped = true
		state.LastError = ""
		state.BackoffUntil = 0
		state.RetryCount = 0
	})
}

// RecordCloudSyncBootstrapFailure persists background bootstrap errors for the settings UI.
func RecordCloudSyncBootstrapFailure(ctx context.Context, err error) {
	if err == nil {
		return
	}

	_, _ = UpdateCloudSyncState(ctx, func(state *database.CloudSyncState) {
		state.Bootstrapped = false
		state.LastError = err.Error()
		state.RetryCount++
		state.BackoffUntil = 0
	})
}
