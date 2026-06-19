package settingadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"wox/cloudsync"
	"wox/common"
	"wox/database"
	"wox/plugin"
	"wox/setting"
	"wox/ui"
	"wox/util"
)

type LocalSettingApplier struct{}

func NewLocalSettingApplier() *LocalSettingApplier {
	return &LocalSettingApplier{}
}

func (a *LocalSettingApplier) ApplyWoxSetting(ctx context.Context, key string, op string, rawValue string) error {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if woxSetting == nil {
		return fmt.Errorf("wox setting not initialized")
	}

	store := setting.NewWoxSettingStore(database.GetDB())
	previousValue, hadPrevious := loadStoredString(store, key)

	if value, ok := findWoxSettingValueByKey(woxSetting, key); ok {
		switch op {
		case cloudsync.OpDelete:
			return value.DeleteLocal()
		case cloudsync.OpUpsert:
			if err := value.SetFromString(rawValue); err != nil {
				return err
			}
			if shouldNotifySettingChange(op, hadPrevious, previousValue, rawValue) {
				ui.GetUIManager().PostSettingUpdate(ctx, key, rawValue)
			}
			return nil
		default:
			return fmt.Errorf("unknown oplog op: %s", op)
		}
	}

	switch op {
	case cloudsync.OpDelete:
		return store.Delete(key)
	case cloudsync.OpUpsert:
		if err := store.Set(key, rawValue); err != nil {
			return err
		}
		if shouldNotifySettingChange(op, hadPrevious, previousValue, rawValue) {
			ui.GetUIManager().PostSettingUpdate(ctx, key, rawValue)
		}
		return nil
	default:
		return fmt.Errorf("unknown oplog op: %s", op)
	}
}

func (a *LocalSettingApplier) ApplyPluginSetting(ctx context.Context, pluginID string, key string, op string, rawValue string) error {
	store := setting.NewPluginSettingStore(database.GetDB(), pluginID)
	previousValue, hadPrevious := loadStoredStringPlugin(store, key)

	switch op {
	case cloudsync.OpDelete:
		return store.Delete(key)
	case cloudsync.OpUpsert:
		if err := store.Set(key, rawValue); err != nil {
			return err
		}
		if shouldNotifySettingChange(op, hadPrevious, previousValue, rawValue) {
			notifyPluginSettingChanged(ctx, pluginID, normalizePluginSettingKey(key), rawValue)
		}
		return nil
	default:
		return fmt.Errorf("unknown oplog op: %s", op)
	}
}

// ApplyInstalledPlugin replays remote plugin install-list changes without
// writing new local install oplogs.
func (a *LocalSettingApplier) ApplyInstalledPlugin(ctx context.Context, pluginID string, op string, rawValue string) error {
	switch op {
	case cloudsync.OpDelete:
		return uninstallPluginLocal(ctx, pluginID)
	case cloudsync.OpUpsert:
		manifest, ok, err := decodeInstalledPluginManifest(ctx, pluginID, rawValue)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		if !plugin.IsAnySupportedInCurrentOS(manifest.SupportedOS) {
			util.GetLogger().Info(ctx, fmt.Sprintf("skip installed plugin sync for %s: unsupported on current OS", manifest.Id))
			return nil
		}
		if shouldSkipPluginInstall(ctx, manifest) {
			return nil
		}
		return plugin.GetStoreManager().InstallLocal(ctx, manifest)
	default:
		return fmt.Errorf("unknown oplog op: %s", op)
	}
}

// ApplyInstalledTheme replays remote theme install-list changes without writing
// new local install oplogs.
func (a *LocalSettingApplier) ApplyInstalledTheme(ctx context.Context, themeID string, op string, rawValue string) error {
	switch op {
	case cloudsync.OpDelete:
		theme := ui.GetUIManager().GetThemeById(themeID)
		if theme.ThemeId == "" || theme.IsSystem {
			return nil
		}
		return ui.GetStoreManager().UninstallLocal(ctx, theme)
	case cloudsync.OpUpsert:
		theme, ok, err := decodeInstalledTheme(ctx, themeID, rawValue)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		return ui.GetStoreManager().InstallLocal(ctx, theme)
	default:
		return fmt.Errorf("unknown oplog op: %s", op)
	}
}

type syncValue interface {
	Key() string
	SetFromString(value string) error
	DeleteLocal() error
}

func findWoxSettingValueByKey(woxSetting *setting.WoxSetting, key string) (syncValue, bool) {
	if woxSetting == nil {
		return nil, false
	}

	v := reflect.ValueOf(woxSetting).Elem()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if field.Kind() != reflect.Pointer || field.IsNil() {
			continue
		}
		value, ok := field.Interface().(syncValue)
		if !ok {
			continue
		}
		if value.Key() == key {
			return value, true
		}
	}

	return nil, false
}

func loadStoredString(store *setting.WoxSettingStore, key string) (string, bool) {
	var value string
	if err := store.Get(key, &value); err != nil {
		return "", false
	}
	return value, true
}

func loadStoredStringPlugin(store *setting.PluginSettingStore, key string) (string, bool) {
	var value string
	if err := store.Get(key, &value); err != nil {
		return "", false
	}
	return value, true
}

// decodeInstalledPluginManifest resolves the downloadable manifest carried by
// the sync record or falls back to the current store cache.
func decodeInstalledPluginManifest(ctx context.Context, pluginID string, rawValue string) (plugin.StorePluginManifest, bool, error) {
	var value cloudsync.InstalledPluginValue
	if err := json.Unmarshal([]byte(rawValue), &value); err != nil {
		return plugin.StorePluginManifest{}, false, err
	}
	if len(value.Manifest) > 0 {
		var manifest plugin.StorePluginManifest
		if err := json.Unmarshal(value.Manifest, &manifest); err != nil {
			return plugin.StorePluginManifest{}, false, err
		}
		if manifest.Id == "" {
			manifest.Id = pluginID
		}
		return manifest, true, nil
	}

	if manifest, err := plugin.GetStoreManager().GetStorePluginManifestById(ctx, pluginID); err == nil {
		return manifest, true, nil
	}
	util.GetLogger().Warn(ctx, fmt.Sprintf("skip installed plugin sync for %s: store manifest not found", pluginID))
	return plugin.StorePluginManifest{}, false, nil
}

// decodeInstalledTheme resolves the full theme payload carried by the sync
// record or falls back to the current store cache.
func decodeInstalledTheme(ctx context.Context, themeID string, rawValue string) (common.Theme, bool, error) {
	var value cloudsync.InstalledThemeValue
	if err := json.Unmarshal([]byte(rawValue), &value); err != nil {
		return common.Theme{}, false, err
	}
	if len(value.Theme) > 0 {
		var theme common.Theme
		if err := json.Unmarshal(value.Theme, &theme); err != nil {
			return common.Theme{}, false, err
		}
		if theme.ThemeId == "" {
			theme.ThemeId = themeID
		}
		return theme, true, nil
	}

	for _, theme := range ui.GetStoreManager().GetThemes() {
		if theme.ThemeId == themeID {
			return theme, true, nil
		}
	}
	util.GetLogger().Warn(ctx, fmt.Sprintf("skip installed theme sync for %s: theme payload not found", themeID))
	return common.Theme{}, false, nil
}

// shouldSkipPluginInstall avoids reinstalling the same or newer local plugin
// while still allowing cloud sync to upgrade older installs.
func shouldSkipPluginInstall(ctx context.Context, manifest plugin.StorePluginManifest) bool {
	for _, instance := range plugin.GetPluginManager().GetPluginInstances() {
		if instance.Metadata.Id != manifest.Id {
			continue
		}
		if instance.Metadata.Version == manifest.Version || !plugin.IsVersionUpgradable(instance.Metadata.Version, manifest.Version) {
			return true
		}
		return false
	}
	return false
}

// uninstallPluginLocal removes cloud-synced plugins but leaves settings cleanup
// to separate plugin_setting delete records.
func uninstallPluginLocal(ctx context.Context, pluginID string) error {
	for _, instance := range plugin.GetPluginManager().GetPluginInstances() {
		if instance.Metadata.Id != pluginID {
			continue
		}
		if instance.IsSystemPlugin || instance.IsDevPlugin {
			return nil
		}
		return plugin.GetStoreManager().UninstallLocal(ctx, instance, true)
	}
	return nil
}

func shouldNotifySettingChange(op string, hadPrevious bool, previousValue string, newValue string) bool {
	if op != cloudsync.OpUpsert {
		return false
	}
	if !hadPrevious {
		return true
	}
	return previousValue != newValue
}

func normalizePluginSettingKey(key string) string {
	suffix := "@" + util.GetCurrentPlatform()
	if strings.HasSuffix(key, suffix) {
		return strings.TrimSuffix(key, suffix)
	}
	return key
}

func notifyPluginSettingChanged(ctx context.Context, pluginID string, key string, value string) {
	instances := plugin.GetPluginManager().GetPluginInstances()
	for _, instance := range instances {
		if instance.Metadata.Id != pluginID {
			continue
		}
		for _, callback := range instance.SettingChangeCallbacks {
			callback(ctx, key, value)
		}
		return
	}
}
