package settingadapter

import (
	"context"
	"fmt"
	"reflect"
	"wox/cloudsync"
	"wox/database"
	"wox/setting"
	"wox/util"
)

type LocalSnapshotter struct{}

func NewLocalSnapshotter() *LocalSnapshotter {
	return &LocalSnapshotter{}
}

// EnqueueLocalSnapshot captures persisted local settings as upsert oplogs for an explicit full push.
func (s *LocalSnapshotter) EnqueueLocalSnapshot(ctx context.Context) error {
	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	var woxSettings []database.WoxSetting
	if err := db.Find(&woxSettings).Error; err != nil {
		return err
	}

	var pluginSettings []database.PluginSetting
	if err := db.Find(&pluginSettings).Error; err != nil {
		return err
	}

	syncableWoxSettings := currentWoxSettingSyncability(ctx)
	disabledPlugins := currentCloudSyncDisabledPlugins(ctx)
	timestamp := util.GetSystemTimestamp()
	oplogs := make([]database.Oplog, 0, len(woxSettings)+len(pluginSettings))

	for _, item := range woxSettings {
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

	if len(oplogs) == 0 {
		return nil
	}

	return db.CreateInBatches(&oplogs, 100).Error
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
