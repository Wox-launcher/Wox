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
