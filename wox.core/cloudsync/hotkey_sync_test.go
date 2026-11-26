package cloudsync

import (
	"context"
	"testing"
	"wox/util"

	"github.com/stretchr/testify/require"
)

type hotkeySyncTestApplier struct {
	testCloudSyncApplier
	enabled     bool
	unavailable bool
}

func (a *hotkeySyncTestApplier) CrossPlatformHotkeysEnabled() bool { return a.enabled }

func (a *hotkeySyncTestApplier) ApplyWoxSetting(ctx context.Context, key, op, value string) error {
	if key == CrossPlatformHotkeysSetting {
		a.enabled = value == "true"
	}
	if err := a.testCloudSyncApplier.ApplyWoxSetting(ctx, key, op, value); err != nil {
		return err
	}
	if a.unavailable && SharedHotkeyBase(key) != "" {
		return ErrSyncedHotkeyUnavailable
	}
	return nil
}

func TestCrossPlatformHotkeyPolicyAndConflictHistory(t *testing.T) {
	initCloudSyncTestDatabase(t)
	ctx := context.Background()
	applier := &hotkeySyncTestApplier{}
	manager := NewCloudSyncManager(DefaultCloudSyncConfig(), CloudSyncDependencies{
		Crypto: testCloudSyncCrypto{}, Applier: applier,
	})
	record := func(key, value string, ts int64) CloudSyncRecord {
		return CloudSyncRecord{EntityType: EntityWoxSetting, Key: key, Op: OpUpsert, ClientTs: ts,
			Value: &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: value},
		}
	}
	shared := SharedHotkeyKey("QueryHotkeys")
	local := "QueryHotkeys@" + util.GetCurrentPlatform()
	require.NoError(t, manager.applyRecords(ctx, []CloudSyncRecord{
		record(shared, "ignored while off", 110), record(local, "local", 110),
	}))
	require.Equal(t, "local", applier.wox[local])
	require.NotContains(t, applier.wox, shared)

	require.NoError(t, manager.applyRecords(ctx, []CloudSyncRecord{
		record(shared, "first device", 210), record(local, "ignored while on", 210),
		record(CrossPlatformHotkeysSetting, "true", 210),
	}))
	require.Equal(t, "first device", applier.wox[shared])
	require.Equal(t, "local", applier.wox[local])
	require.NoError(t, manager.applyRecords(ctx, []CloudSyncRecord{record(shared, "second device", 211)}))
	require.Equal(t, "second device", applier.wox[shared])

	applier.unavailable = true
	result := manager.applyRecordsWithDetails(ctx, []CloudSyncRecord{record(shared, "conflict", 230)}, nil, false)
	require.NoError(t, result.Err())
	require.Len(t, result.Details, 1)
	require.Equal(t, CloudSyncHistoryStatusHotkeyUnavailable, result.Details[0].Status)
	require.Equal(t, "conflict", applier.wox[shared])
	require.Equal(t, 1, result.Succeeded)
	require.Equal(t, int64(230), latestLocalChangeTimestamp(record(shared, "", 230)))

	require.NoError(t, manager.applyRecords(ctx, []CloudSyncRecord{
		record(local, "same platform again", 240), record(CrossPlatformHotkeysSetting, "false", 240),
	}))
	require.Equal(t, "same platform again", applier.wox[local])
}

func TestRestoreSnapshotAppliesSharedHotkeysAfterPolicyFromLaterPage(t *testing.T) {
	initCloudSyncTestDatabase(t)
	shared := SharedHotkeyKey("ActionPanelHotkey")
	legacy := "ActionPanelHotkey@" + util.GetCurrentPlatform()
	records := []CloudSyncRecord{
		{EntityType: EntityWoxSetting, Key: shared, Op: OpUpsert, Value: &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "shared"}},
		{EntityType: EntityWoxSetting, Key: legacy, Op: OpUpsert, Value: &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "legacy"}},
		{EntityType: EntityWoxSetting, Key: CrossPlatformHotkeysSetting, Op: OpUpsert, Value: &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "true"}},
	}
	client := &testCloudSyncClient{snapshotResponses: []*CloudSyncPullResponse{
		{Records: records[:2], HasMore: true, NextCursor: "1"},
		{Records: records[2:]},
	}}
	applier := &hotkeySyncTestApplier{}
	manager := NewCloudSyncManager(DefaultCloudSyncConfig(), CloudSyncDependencies{
		Client: client, Crypto: testCloudSyncCrypto{}, Applier: applier,
		DeviceProvider: testCloudSyncDeviceProvider{deviceID: "device-a"},
	})
	require.NoError(t, manager.RestoreSnapshot(context.Background()))
	require.Equal(t, "shared", applier.wox[shared])
	require.NotContains(t, applier.wox, legacy)
}
