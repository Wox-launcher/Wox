package ui

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"wox/cloudsync"
	corehotkey "wox/hotkey"
	"wox/plugin"
	"wox/privacy"
	"wox/setting"
)

// updateLocalHotkeySetting uses the same sync lock as remote registration and persistence.
func updateLocalHotkeySetting(ctx context.Context, settings *setting.WoxSetting, key, value string) error {
	var saveErr error
	switch key {
	case "MainHotkey":
		if value != settings.MainHotkey.Get() {
			if err := GetUIManager().RegisterMainHotkey(ctx, value); err != nil {
				return err
			}
		}
		saveErr = settings.MainHotkey.Set(value)
	case "SelectionHotkey":
		if value != settings.SelectionHotkey.Get() {
			if err := GetUIManager().RegisterSelectionHotkey(ctx, value); err != nil {
				return err
			}
		}
		saveErr = settings.SelectionHotkey.Set(value)
	case "RefinementHotkey":
		saveErr = settings.RefinementHotkey.Set(value)
	case "ActionPanelHotkey":
		saveErr = settings.ActionPanelHotkey.Set(value)
	case "QueryHotkeys":
		queryHotkeys, err := parseQueryHotkeysSettingValue(value)
		if err != nil {
			return err
		}
		if err := plugin.ValidateQueryHotkeysAgainstBindings(queryHotkeys, resultBindingsFromWoxSetting(settings)); err != nil {
			return err
		}
		config := corehotkey.WoxConfigFromSetting(settings)
		config.QueryHotkeys = queryHotkeys
		if err := GetUIManager().registerWoxHotkeys(ctx, config, true); err != nil {
			return err
		}
		return settings.QueryHotkeys.Set(queryHotkeys)
	case "ResultBindings":
		bindings, err := parseResultBindingsSettingValue(value)
		if err != nil {
			return err
		}
		return GetUIManager().applyResultBindings(ctx, bindings)
	}
	if saveErr != nil {
		return saveErr
	}
	return privacy.RefreshPreservedSettings(settings)
}

// ApplySyncedHotkeySetting saves the mapped settings even when a native binding is unavailable.
func (m *Manager) ApplySyncedHotkeySetting(ctx context.Context, key string, value any) error {
	settings := setting.GetSettingManager().GetWoxSetting(ctx)
	config := corehotkey.WoxConfigFromSetting(settings)
	var previous any
	var persist func() error
	switch key {
	case "MainHotkey":
		previous, config.MainHotkey = config.MainHotkey, value.(string)
		persist = func() error { return settings.MainHotkey.SetLocal(config.MainHotkey) }
	case "SelectionHotkey":
		previous, config.SelectionHotkey = config.SelectionHotkey, value.(string)
		persist = func() error { return settings.SelectionHotkey.SetLocal(config.SelectionHotkey) }
	case "QueryHotkeys":
		previous, config.QueryHotkeys = settings.QueryHotkeys.Get(), value.([]setting.QueryHotkey)
		persist = func() error { return settings.QueryHotkeys.SetLocal(config.QueryHotkeys) }
	case "ResultBindings":
		local := settings.ResultBindings.Get()
		previous = local
		config.ResultBindings = setting.MergeResultHotkeysAndAliases(value.([]setting.ResultBinding), local)
		value = config.ResultBindings
		persist = func() error { return settings.ResultBindings.SetLocal(config.ResultBindings) }
	case "ActionPanelHotkey":
		return settings.ActionPanelHotkey.SetLocal(value.(string))
	case "RefinementHotkey":
		return settings.RefinementHotkey.SetLocal(value.(string))
	default:
		return fmt.Errorf("unsupported synced shortcut setting %q", key)
	}
	if reflect.DeepEqual(previous, value) {
		return nil
	}
	err := m.hotkeyService.ApplySyncedConfig(ctx, config, persist)
	unavailable := errors.Is(err, corehotkey.ErrHotkeyRegistrationConflict)
	if err == nil || unavailable {
		m.syncMainHotkeyToolbarWarning(ctx, config.MainHotkey, err)
	}
	if unavailable || (err == nil && key == "SelectionHotkey" && config.SelectionHotkey != "" &&
		corehotkey.EffectiveSelectionHotkeyForRuntime(config.SelectionHotkey) == "") {
		return cloudsync.ErrSyncedHotkeyUnavailable
	}
	return err
}
