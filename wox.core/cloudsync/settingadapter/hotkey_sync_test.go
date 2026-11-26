package settingadapter

import (
	"encoding/json"
	"testing"
	"wox/cloudsync"
	"wox/setting"

	"github.com/stretchr/testify/require"
)

func TestMapSharedQueryHotkeysInBothDirections(t *testing.T) {
	windows := setting.QueryHotkey{Hotkey: "ctrl+shift+k", Query: "run C:\\work\\ctrl.txt", Name: "Run", IsSilentExecution: true}
	raw, err := json.Marshal([]setting.QueryHotkey{windows})
	require.NoError(t, err)
	mac, err := mapSharedHotkey("QueryHotkeys", cloudsync.SharedHotkey{Platform: "windows", Value: raw}, "darwin")
	require.NoError(t, err)
	expected := windows
	expected.Hotkey = "cmd+shift+k"
	require.Equal(t, []setting.QueryHotkey{expected}, mac)

	expected.Hotkey = "cmd+shift+j"
	raw, err = json.Marshal([]setting.QueryHotkey{expected})
	require.NoError(t, err)
	back, err := mapSharedHotkey("QueryHotkeys", cloudsync.SharedHotkey{Platform: "darwin", Value: raw}, "windows")
	require.NoError(t, err)
	windows.Hotkey = "ctrl+shift+j"
	require.Equal(t, []setting.QueryHotkey{windows}, back)
}
