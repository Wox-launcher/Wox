package settingadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"wox/cloudsync"
	"wox/database"
	"wox/plugin"
	"wox/setting"
	"wox/ui"
	"wox/util"
)

type LocalSnapshotter struct{}

func NewLocalSnapshotter() *LocalSnapshotter {
	return &LocalSnapshotter{}
}

// EnqueueLocalSnapshot captures persisted local settings as upsert oplogs for an explicit full push.
func (s *LocalSnapshotter) EnqueueLocalSnapshot(ctx context.Context) error {
	oplogs, err := s.collectLocalSnapshotOplogs(ctx)
	if err != nil {
		return err
	}
	return createSnapshotOplogs(oplogs)
}

// EnqueueMissingLocalSnapshot captures only persisted local records whose cloud identity is absent remotely.
func (s *LocalSnapshotter) EnqueueMissingLocalSnapshot(ctx context.Context, remoteKeys []cloudsync.CloudSyncRecordKey) error {
	remote := cloudSyncRecordKeySet(remoteKeys)
	oplogs, err := s.collectLocalSnapshotOplogs(ctx)
	if err != nil {
		return err
	}

	missing := make([]database.Oplog, 0, len(oplogs))
	for _, oplog := range oplogs {
		if _, exists := remote[cloudSyncOplogIdentity(oplog)]; exists {
			continue
		}
		missing = append(missing, oplog)
	}

	return createSnapshotOplogs(missing)
}

// collectLocalSnapshotOplogs builds the same local upsert set for both full and missing-key snapshots.
func (s *LocalSnapshotter) collectLocalSnapshotOplogs(ctx context.Context) ([]database.Oplog, error) {
	db := database.GetDB()
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var woxSettings []database.WoxSetting
	if err := db.Find(&woxSettings).Error; err != nil {
		return nil, err
	}

	var pluginSettings []database.PluginSetting
	if err := db.Find(&pluginSettings).Error; err != nil {
		return nil, err
	}

	syncableWoxSettings := currentWoxSettingSyncability(ctx)
	disabledPlugins := currentCloudSyncDisabledPlugins(ctx)
	timestamp := util.GetSystemTimestamp()
	oplogs := make([]database.Oplog, 0, len(woxSettings)+len(pluginSettings))

	for _, item := range woxSettings {
		if !isCurrentPlatformSettingKey(item.Key) {
			continue
		}
		if syncable, ok := syncableWoxSettings[item.Key]; ok && !syncable {
			continue
		}
		oplogs = append(oplogs, database.Oplog{
			EntityType: cloudsync.EntityWoxSetting,
			EntityID:   item.Key,
			Operation:  cloudsync.OpUpsert,
			Key:        item.Key,
			Value:      item.Value,
			Timestamp:  timestamp,
		})
	}

	for _, item := range pluginSettings {
		if !isCurrentPlatformSettingKey(item.Key) {
			continue
		}
		if _, blocked := disabledPlugins[item.PluginID]; blocked {
			continue
		}
		oplogs = append(oplogs, database.Oplog{
			EntityType: cloudsync.EntityPluginSetting,
			EntityID:   item.PluginID,
			Operation:  cloudsync.OpUpsert,
			Key:        item.Key,
			Value:      item.Value,
			Timestamp:  timestamp,
		})
	}

	if err := appendInstalledPluginOplogs(ctx, disabledPlugins, timestamp, &oplogs); err != nil {
		return nil, err
	}
	if err := appendInstalledThemeOplogs(ctx, timestamp, &oplogs); err != nil {
		return nil, err
	}

	return oplogs, nil
}

// createSnapshotOplogs persists snapshot rows through the normal pending-oplog table.
func createSnapshotOplogs(oplogs []database.Oplog) error {
	if len(oplogs) == 0 {
		return nil
	}

	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	return db.CreateInBatches(&oplogs, 100).Error
}

// appendInstalledPluginOplogs snapshots store-installed plugins that can be reproduced on another device.
func appendInstalledPluginOplogs(ctx context.Context, disabledPlugins map[string]struct{}, timestamp int64, oplogs *[]database.Oplog) error {
	for _, instance := range plugin.GetPluginManager().GetPluginInstances() {
		if instance.IsSystemPlugin || instance.IsDevPlugin {
			continue
		}
		if _, blocked := disabledPlugins[instance.Metadata.Id]; blocked {
			continue
		}

		manifest, ok := resolveStorePluginManifest(ctx, instance.Metadata.Id)
		if !ok {
			util.GetLogger().Warn(ctx, fmt.Sprintf("skip plugin install snapshot for %s: store manifest not found", instance.Metadata.Id))
			continue
		}

		manifestJSON, err := json.Marshal(manifest)
		if err != nil {
			return fmt.Errorf("failed to encode installed plugin sync value for %s: %w", manifest.Id, err)
		}
		value := cloudsync.InstalledPluginValue{
			ID:       manifest.Id,
			Version:  manifest.Version,
			Source:   cloudsync.InstallSyncSourceStore,
			Manifest: manifestJSON,
		}
		rawValue, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to encode installed plugin snapshot for %s: %w", manifest.Id, err)
		}
		*oplogs = append(*oplogs, database.Oplog{
			EntityType: cloudsync.EntityInstalledPlugin,
			EntityID:   manifest.Id,
			Operation:  cloudsync.OpUpsert,
			Key:        manifest.Id,
			Value:      string(rawValue),
			Timestamp:  timestamp,
		})
	}

	return nil
}

// appendInstalledThemeOplogs snapshots user-installed themes with their full current payload.
func appendInstalledThemeOplogs(ctx context.Context, timestamp int64, oplogs *[]database.Oplog) error {
	for _, theme := range ui.GetUIManager().GetAllThemes(ctx) {
		if theme.IsSystem {
			continue
		}

		themeJSON, err := json.Marshal(theme)
		if err != nil {
			return fmt.Errorf("failed to encode installed theme sync value for %s: %w", theme.ThemeId, err)
		}
		value := cloudsync.InstalledThemeValue{
			ID:      theme.ThemeId,
			Version: theme.Version,
			Source:  cloudsync.InstallSyncSourceUser,
			Theme:   themeJSON,
		}
		rawValue, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to encode installed theme snapshot for %s: %w", theme.ThemeId, err)
		}
		*oplogs = append(*oplogs, database.Oplog{
			EntityType: cloudsync.EntityInstalledTheme,
			EntityID:   theme.ThemeId,
			Operation:  cloudsync.OpUpsert,
			Key:        theme.ThemeId,
			Value:      string(rawValue),
			Timestamp:  timestamp,
		})
	}

	return nil
}

// resolveStorePluginManifest finds the downloadable manifest required to replay an installed plugin.
func resolveStorePluginManifest(ctx context.Context, pluginID string) (plugin.StorePluginManifest, bool) {
	store := plugin.GetStoreManager()
	if manifest, err := store.GetStorePluginManifestById(ctx, pluginID); err == nil {
		return manifest, true
	}

	for _, manifest := range store.GetStorePluginManifests(ctx) {
		if manifest.Id == pluginID {
			return manifest, true
		}
	}
	return plugin.StorePluginManifest{}, false
}

// cloudSyncRecordKeySet indexes remote record identities regardless of their current operation.
func cloudSyncRecordKeySet(keys []cloudsync.CloudSyncRecordKey) map[string]struct{} {
	result := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		result[cloudSyncRecordKeyIdentity(key)] = struct{}{}
	}
	return result
}

func cloudSyncRecordKeyIdentity(key cloudsync.CloudSyncRecordKey) string {
	return cloudSyncIdentity(key.EntityType, key.PluginID, key.Key)
}

func cloudSyncOplogIdentity(oplog database.Oplog) string {
	pluginID := ""
	if oplog.EntityType == cloudsync.EntityPluginSetting {
		pluginID = oplog.EntityID
	}
	return cloudSyncIdentity(oplog.EntityType, pluginID, oplog.Key)
}

func cloudSyncIdentity(entityType string, pluginID string, key string) string {
	return entityType + "\x00" + pluginID + "\x00" + key
}

func isCurrentPlatformSettingKey(key string) bool {
	_, platform, ok := setting.SplitPlatformSettingKey(key)
	return !ok || platform == util.GetCurrentPlatform()
}

type syncableWoxSettingValue interface {
	Key() string
	IsSyncable() bool
}

// currentWoxSettingSyncability reads typed setting definitions so local-only persisted keys stay out of snapshot oplogs.
func currentWoxSettingSyncability(ctx context.Context) map[string]bool {
	settingManager := setting.GetSettingManager()
	if settingManager == nil {
		return nil
	}

	woxSetting := settingManager.GetWoxSetting(ctx)
	if woxSetting == nil {
		return nil
	}

	result := map[string]bool{}
	v := reflect.ValueOf(woxSetting).Elem()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if field.Kind() != reflect.Pointer || field.IsNil() {
			continue
		}
		value, ok := field.Interface().(syncableWoxSettingValue)
		if !ok {
			continue
		}
		result[value.Key()] = value.IsSyncable()
	}

	return result
}

// currentCloudSyncDisabledPlugins returns the plugin IDs intentionally excluded from cloud sync.
func currentCloudSyncDisabledPlugins(ctx context.Context) map[string]struct{} {
	settingManager := setting.GetSettingManager()
	if settingManager == nil {
		return nil
	}

	woxSetting := settingManager.GetWoxSetting(ctx)
	if woxSetting == nil {
		return nil
	}

	disabled := map[string]struct{}{}
	for _, pluginId := range woxSetting.CloudSyncDisabledPlugins.Get() {
		if pluginId == "" {
			continue
		}
		disabled[pluginId] = struct{}{}
	}

	return disabled
}
