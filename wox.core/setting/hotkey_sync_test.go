package setting

import (
	"encoding/json"
	"testing"
	"wox/cloudsync"
	"wox/database"
	"wox/util"

	"github.com/stretchr/testify/require"
)

func TestSharedHotkeyOplogsRequireOptIn(t *testing.T) {
	db := openSettingSyncDB(t)
	settings := NewWoxSetting(NewWoxSettingStore(db))
	sharedKey := cloudsync.SharedHotkeyKey("QueryHotkeys")
	query := QueryHotkey{Hotkey: "ctrl+k", Query: "find project"}
	load := func(key string) []database.Oplog {
		var rows []database.Oplog
		require.NoError(t, db.Where("key = ?", key).Order("id").Find(&rows).Error)
		return rows
	}
	require.False(t, settings.CloudSyncCrossPlatformHotkeys.Get())
	require.NoError(t, settings.QueryHotkeys.Set([]QueryHotkey{query}))
	require.Len(t, load(settings.QueryHotkeys.Key()), 1)
	require.Empty(t, load(sharedKey))

	require.NoError(t, settings.CloudSyncCrossPlatformHotkeys.Set(true))
	require.Len(t, load(cloudsync.SharedHotkeyKey("MainHotkey")), 1)
	rows := load(sharedKey)
	require.Len(t, rows, 1)
	require.Less(t, load(cloudsync.CrossPlatformHotkeysSetting)[0].ID, rows[0].ID)
	var shared cloudsync.SharedHotkey
	require.NoError(t, json.Unmarshal([]byte(rows[0].Value), &shared))
	require.Equal(t, util.GetCurrentPlatform(), shared.Platform)
	encoded, err := json.Marshal([]QueryHotkey{query})
	require.NoError(t, err)
	require.JSONEq(t, string(encoded), string(shared.Value))

	query.Hotkey = "ctrl+j"
	require.NoError(t, settings.QueryHotkeys.Set([]QueryHotkey{query}))
	require.Len(t, load(sharedKey), 2)
	require.NoError(t, settings.CloudSyncCrossPlatformHotkeys.Set(false))
	require.NoError(t, settings.QueryHotkeys.Set(nil))
	require.Len(t, load(sharedKey), 2)
	require.Len(t, load(settings.QueryHotkeys.Key()), 3)
}
