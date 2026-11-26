package cloudsync

import (
	"context"
	"errors"
	"testing"
	"wox/database"

	"github.com/stretchr/testify/require"
)

type conflictPreserverFunc func(context.Context, Conflict) (func(), error)

func (f conflictPreserverFunc) PreserveConflict(ctx context.Context, conflict Conflict) (func(), error) {
	return f(ctx, conflict)
}

func TestConflictPreservationFailureKeepsLocalChangePending(t *testing.T) {
	initCloudSyncTestDatabase(t)
	ctx := context.Background()
	const key = "note:original"
	oplog := database.Oplog{EntityType: EntityPluginSetting, EntityID: "notes", Key: key, Operation: OpUpsert, Timestamp: 12, Value: "local"}
	require.NoError(t, database.GetDB().Create(&oplog).Error)
	applier := &testCloudSyncApplier{}
	saveErr := errors.New("archive is unwritable")
	manager := NewCloudSyncManager(DefaultCloudSyncConfig(), CloudSyncDependencies{
		Applier: applier, Crypto: testCloudSyncCrypto{},
		ConflictPreserver: conflictPreserverFunc(func(_ context.Context, conflict Conflict) (func(), error) {
			require.Equal(t, "local", conflict.LocalValue)
			require.Equal(t, "remote", conflict.RemoteValue)
			return nil, saveErr
		}),
	})
	result := manager.applyRecordsWithDetails(ctx, []CloudSyncRecord{
		{EntityType: EntityPluginSetting, PluginID: "notes", Key: key, Op: OpUpsert, ClientTs: 20, Value: &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "remote"}},
	}, nil, false)
	require.ErrorIs(t, result.haltErr, saveErr)
	require.Empty(t, applier.plugins)
	require.NoError(t, database.GetDB().First(&oplog, oplog.ID).Error)
	require.False(t, oplog.CloudSyncDiscarded)
}

func TestUnsupportedPluginSkipReportsStatusWithoutEnqueuingDelete(t *testing.T) {
	initCloudSyncTestDatabase(t)
	applier := &testCloudSyncApplier{installErr: ErrSkippedUnsupportedOS}
	manager := NewCloudSyncManager(DefaultCloudSyncConfig(), CloudSyncDependencies{
		Applier: applier, Crypto: testCloudSyncCrypto{},
	})
	result := manager.applyRecordsWithDetails(context.Background(), []CloudSyncRecord{
		{EntityType: EntityInstalledPlugin, Key: "other-os-plugin", Op: OpUpsert, ClientTs: 20, Value: &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "plugin-info"}},
	}, nil, false)
	require.NoError(t, result.Err())
	require.Len(t, result.Details, 1)
	require.Equal(t, CloudSyncHistoryStatusSkippedUnsupportedOS, result.Details[0].Status)
	var count int64
	require.NoError(t, database.GetDB().Model(&database.Oplog{}).Count(&count).Error)
	require.Zero(t, count)
}
