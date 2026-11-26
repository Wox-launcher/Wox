package setting

import (
	"encoding/json"
	"errors"
	"wox/cloudsync"
	"wox/database"
	"wox/util"

	"gorm.io/gorm"
)

// enqueueSharedHotkeyChange writes the shared copy in the setting's oplog transaction.
func (s *WoxSettingStore) enqueueSharedHotkeyChange(key string, value any) error {
	if key == cloudsync.CrossPlatformHotkeysSetting {
		if value == true {
			return s.enqueueCurrentHotkeys()
		}
		return nil
	}
	base, platform, ok := SplitPlatformSettingKey(key)
	if !ok || platform != util.GetCurrentPlatform() || cloudsync.SharedHotkeyKey(base) == "" {
		return nil
	}
	var enabled bool
	if err := s.Get(cloudsync.CrossPlatformHotkeysSetting, &enabled); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if !enabled {
		return nil
	}
	return s.enqueueSharedHotkey(base, value)
}

// enqueueCurrentHotkeys seeds the account from the device that explicitly enables sync.
func (s *WoxSettingStore) enqueueCurrentHotkeys() error {
	oplogs, err := NewWoxSetting(s).SharedHotkeyOplogs(util.GetSystemTimestamp())
	if err != nil {
		return err
	}
	return s.db.Create(&oplogs).Error
}

// SharedHotkeyOplogs describes the current shortcuts for an explicit cloud snapshot.
func (s *WoxSetting) SharedHotkeyOplogs(timestamp int64) ([]database.Oplog, error) {
	oplogs := make([]database.Oplog, 0, 6)
	for _, item := range []struct {
		base  string
		value any
	}{
		{"MainHotkey", s.MainHotkey.Get()},
		{"SelectionHotkey", s.SelectionHotkey.Get()},
		{"ActionPanelHotkey", s.ActionPanelHotkey.Get()},
		{"RefinementHotkey", s.RefinementHotkey.Get()},
		{"QueryHotkeys", s.QueryHotkeys.Get()},
		{"ResultBindings", s.ResultBindings.Get()},
	} {
		oplog, err := sharedHotkeyOplog(item.base, item.value)
		if err != nil {
			return nil, err
		}
		oplog.Timestamp = timestamp
		oplogs = append(oplogs, oplog)
	}
	return oplogs, nil
}

func (s *WoxSettingStore) enqueueSharedHotkey(base string, value any) error {
	oplog, err := sharedHotkeyOplog(base, value)
	if err != nil {
		return err
	}
	return writeCloudSyncOplog(s.db, oplog)
}

// sharedHotkeyOplog serializes the shared value while excluding result aliases.
func sharedHotkeyOplog(base string, value any) (database.Oplog, error) {
	if bindings, ok := value.([]ResultBinding); ok {
		hotkeys := make([]ResultBinding, 0, len(bindings))
		for _, binding := range bindings {
			if binding.Hotkey != "" {
				binding.Alias = ""
				hotkeys = append(hotkeys, binding)
			}
		}
		value = hotkeys
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return database.Oplog{}, err
	}
	payload, err := json.Marshal(cloudsync.SharedHotkey{
		Platform: util.GetCurrentPlatform(),
		Value:    encoded,
	})
	key := cloudsync.SharedHotkeyKey(base)
	return database.Oplog{EntityType: cloudsync.EntityWoxSetting, EntityID: key, Key: key, Value: string(payload), Operation: cloudsync.OpUpsert}, err
}
