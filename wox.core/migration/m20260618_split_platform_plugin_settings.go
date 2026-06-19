package migration

import (
	"context"
	"errors"
	"wox/cloudsync"
	"wox/database"
	"wox/setting"
	"wox/util"

	"gorm.io/gorm"
)

const (
	appPluginID             = "ea2b6859-14bc-4c89-9c88-627da7379141"
	explorerPluginID        = "6cde8bec-3f19-44f6-8a8b-d3ba3712d04e"
	wpmPluginID             = "e2c5f005-6c73-43c8-bc53-ab04def265b2"
	folderPluginID          = "527ba64f-c8f5-4fc7-bb98-306f79d27f32"
	shellPluginID           = "8a4b5c6d-7e8f-9a0b-1c2d-3e4f5a6b7c8d"
	browserBookmarkPluginID = "95d041d3-be7e-4b20-8517-88dda2db280b"
	webSearchPluginID       = "c1e350a7-c521-4dc3-b4ff-509f720fde86"
	webViewPluginID         = "2ac1b5cf-bf55-41f0-8c34-421c323be780"
)

func init() {
	Register(&splitPlatformPluginSettingsMigration{})
}

type splitPlatformPluginSettingsMigration struct{}

type pluginSettingTarget struct {
	pluginID string
	keys     []string
}

func (m *splitPlatformPluginSettingsMigration) ID() string {
	return "20260618_split_platform_plugin_settings"
}

func (m *splitPlatformPluginSettingsMigration) Description() string {
	return "Move platform-dependent system plugin settings to per-platform keys and fold web search/webview settings back to global keys."
}

func (m *splitPlatformPluginSettingsMigration) Up(ctx context.Context, tx *gorm.DB) error {
	for _, target := range []pluginSettingTarget{
		{pluginID: appPluginID, keys: []string{"AppDirectories", "IgnoreRules"}},
		{pluginID: fileSearchPluginID, keys: []string{"roots", "ignorePatterns"}},
		{pluginID: explorerPluginID, keys: []string{"enableTypeToSearch", "quickJumpPaths"}},
		{pluginID: wpmPluginID, keys: []string{"localPluginDirectories"}},
		{pluginID: folderPluginID, keys: []string{"favorites"}},
		{pluginID: shellPluginID, keys: []string{"shellCommands"}},
		{pluginID: browserBookmarkPluginID, keys: []string{"indexBrowsers"}},
	} {
		if err := migratePluginSettingsToCurrentPlatform(tx, target.pluginID, target.keys); err != nil {
			return err
		}
	}

	for _, target := range []pluginSettingTarget{
		{pluginID: webSearchPluginID, keys: []string{"defaultBrowser", "webSearches"}},
		{pluginID: webViewPluginID, keys: []string{"sites"}},
	} {
		if err := migratePluginSettingsToGlobal(tx, target.pluginID, target.keys); err != nil {
			return err
		}
	}

	return nil
}

func migratePluginSettingsToCurrentPlatform(tx *gorm.DB, pluginID string, keys []string) error {
	for _, key := range keys {
		if err := movePluginSettingToCurrentPlatform(tx, pluginID, key); err != nil {
			return err
		}
		if err := convertPluginSettingOplogsToCurrentPlatform(tx, pluginID, key); err != nil {
			return err
		}
		if err := appendPluginSettingDeleteOplog(tx, pluginID, key); err != nil {
			return err
		}
	}
	return nil
}

func migratePluginSettingsToGlobal(tx *gorm.DB, pluginID string, keys []string) error {
	for _, key := range keys {
		if err := moveCurrentPlatformPluginSettingToGlobal(tx, pluginID, key); err != nil {
			return err
		}
		if err := convertCurrentPlatformPluginSettingOplogsToGlobal(tx, pluginID, key); err != nil {
			return err
		}
		if err := appendPluginSettingDeleteOplog(tx, pluginID, setting.PlatformSettingKey(key, util.GetCurrentPlatform())); err != nil {
			return err
		}
	}
	return nil
}

func movePluginSettingToCurrentPlatform(tx *gorm.DB, pluginID string, key string) error {
	var row database.PluginSetting
	err := tx.Where("plugin_id = ? AND key = ?", pluginID, key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	targetKey := setting.PlatformSettingKey(key, util.GetCurrentPlatform())
	inserted, err := upsertPluginSettingIfMissing(tx, pluginID, targetKey, row.Value)
	if err != nil {
		return err
	}
	if inserted {
		if err := appendPluginSettingUpsertOplog(tx, pluginID, targetKey, row.Value); err != nil {
			return err
		}
	}
	if err := tx.Delete(&database.PluginSetting{PluginID: pluginID, Key: key}).Error; err != nil {
		return err
	}
	return nil
}

func moveCurrentPlatformPluginSettingToGlobal(tx *gorm.DB, pluginID string, key string) error {
	platformKey := setting.PlatformSettingKey(key, util.GetCurrentPlatform())
	var row database.PluginSetting
	err := tx.Where("plugin_id = ? AND key = ?", pluginID, platformKey).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	if err := tx.Save(&database.PluginSetting{PluginID: pluginID, Key: key, Value: row.Value}).Error; err != nil {
		return err
	}
	if err := appendPluginSettingUpsertOplog(tx, pluginID, key, row.Value); err != nil {
		return err
	}
	if err := tx.Delete(&database.PluginSetting{PluginID: pluginID, Key: platformKey}).Error; err != nil {
		return err
	}
	return nil
}

func upsertPluginSettingIfMissing(tx *gorm.DB, pluginID string, key string, value string) (bool, error) {
	var existing database.PluginSetting
	err := tx.Where("plugin_id = ? AND key = ?", pluginID, key).First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	if err == nil {
		return false, nil
	}
	return true, tx.Save(&database.PluginSetting{PluginID: pluginID, Key: key, Value: value}).Error
}

func convertPluginSettingOplogsToCurrentPlatform(tx *gorm.DB, pluginID string, key string) error {
	targetKey := setting.PlatformSettingKey(key, util.GetCurrentPlatform())
	return convertPluginSettingOplogs(tx, pluginID, key, targetKey)
}

func convertCurrentPlatformPluginSettingOplogsToGlobal(tx *gorm.DB, pluginID string, key string) error {
	sourceKey := setting.PlatformSettingKey(key, util.GetCurrentPlatform())
	return convertPluginSettingOplogs(tx, pluginID, sourceKey, key)
}

func convertPluginSettingOplogs(tx *gorm.DB, pluginID string, sourceKey string, targetKey string) error {
	var rows []database.Oplog
	if err := tx.Where("entity_type = ? AND entity_id = ? AND key = ? AND synced_to_cloud = ?", cloudsync.EntityPluginSetting, pluginID, sourceKey, false).Find(&rows).Error; err != nil {
		return err
	}

	for _, row := range rows {
		if err := tx.Create(&database.Oplog{
			EntityType: row.EntityType,
			EntityID:   row.EntityID,
			Operation:  row.Operation,
			Key:        targetKey,
			Value:      row.Value,
			Timestamp:  row.Timestamp,
			SyncAfter:  row.SyncAfter,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&database.Oplog{}).Where("id = ?", row.ID).Update("synced_to_cloud", true).Error; err != nil {
			return err
		}
	}

	return nil
}

func appendPluginSettingDeleteOplog(tx *gorm.DB, pluginID string, key string) error {
	return tx.Create(&database.Oplog{
		EntityType: cloudsync.EntityPluginSetting,
		EntityID:   pluginID,
		Operation:  cloudsync.OpDelete,
		Key:        key,
		Timestamp:  util.GetSystemTimestamp(),
	}).Error
}

func appendPluginSettingUpsertOplog(tx *gorm.DB, pluginID string, key string, value string) error {
	return tx.Create(&database.Oplog{
		EntityType: cloudsync.EntityPluginSetting,
		EntityID:   pluginID,
		Operation:  cloudsync.OpUpsert,
		Key:        key,
		Value:      value,
		Timestamp:  util.GetSystemTimestamp(),
	}).Error
}
