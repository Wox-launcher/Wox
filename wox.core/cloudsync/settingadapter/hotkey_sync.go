package settingadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"wox/cloudsync"
	"wox/setting"
	"wox/ui"
	"wox/util"
	"wox/util/hotkey"
)

func (a *LocalSettingApplier) CrossPlatformHotkeysEnabled() bool {
	settings := setting.GetSettingManager().GetWoxSetting(context.Background())
	return settings != nil && settings.CloudSyncCrossPlatformHotkeys.Get()
}

// applySyncedResultAliases leaves shared hotkeys intact when applying same-platform aliases.
func applySyncedResultAliases(settings *setting.WoxSetting, op, raw string) error {
	var aliases []setting.ResultBinding
	switch op {
	case cloudsync.OpUpsert:
		if err := json.Unmarshal([]byte(raw), &aliases); err != nil {
			return err
		}
	case cloudsync.OpDelete:
	default:
		return fmt.Errorf("unexpected result alias operation %q", op)
	}
	return settings.ResultBindings.SetLocal(setting.MergeResultHotkeysAndAliases(settings.ResultBindings.Get(), aliases))
}

// applySharedHotkey maps an account shortcut to the receiving device before registration.
func (a *LocalSettingApplier) applySharedHotkey(ctx context.Context, base, op, raw string) error {
	if op != cloudsync.OpUpsert {
		return fmt.Errorf("unexpected shared shortcut operation %q", op)
	}
	var shared cloudsync.SharedHotkey
	if err := json.Unmarshal([]byte(raw), &shared); err != nil {
		return err
	}
	value, err := mapSharedHotkey(base, shared, util.GetCurrentPlatform())
	if err != nil {
		return fmt.Errorf("map shared shortcut %s: %w", base, err)
	}
	return ui.GetUIManager().ApplySyncedHotkeySetting(ctx, base, value)
}

// mapSharedHotkey converts only the key combinations within each typed setting.
func mapSharedHotkey(base string, shared cloudsync.SharedHotkey, target string) (any, error) {
	mapKey := func(key string) (string, error) {
		return hotkey.MapHotkeyBetweenPlatforms(key, shared.Platform, target)
	}
	switch base {
	case "QueryHotkeys":
		var entries []setting.QueryHotkey
		if err := json.Unmarshal(shared.Value, &entries); err != nil {
			return nil, err
		}
		for i := range entries {
			key, err := mapKey(entries[i].Hotkey)
			if err != nil {
				return nil, err
			}
			entries[i].Hotkey = key
		}
		return entries, nil
	case "ResultBindings":
		var entries []setting.ResultBinding
		if err := json.Unmarshal(shared.Value, &entries); err != nil {
			return nil, err
		}
		for i := range entries {
			key, err := mapKey(entries[i].Hotkey)
			if err != nil {
				return nil, err
			}
			entries[i].Hotkey = key
		}
		return entries, nil
	default:
		var key string
		if err := json.Unmarshal(shared.Value, &key); err != nil {
			return nil, err
		}
		return mapKey(key)
	}
}
