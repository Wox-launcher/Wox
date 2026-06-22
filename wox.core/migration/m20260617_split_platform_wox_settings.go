package migration

import (
	"context"
	"encoding/json"
	"errors"
	"wox/cloudsync"
	"wox/database"
	"wox/setting"
	"wox/util"

	"gorm.io/gorm"
)

func init() {
	Register(&splitPlatformWoxSettingsMigration{})
}

type splitPlatformWoxSettingsMigration struct{}

type legacyPlatformValues[T any] struct {
	MacValue   T
	WinValue   T
	LinuxValue T
}

func (m *splitPlatformWoxSettingsMigration) ID() string {
	return "20260617_split_platform_wox_settings"
}

func (m *splitPlatformWoxSettingsMigration) Description() string {
	return "Split legacy PlatformValue JSON settings into per-platform physical WoxSetting keys."
}

func (m *splitPlatformWoxSettingsMigration) Up(ctx context.Context, tx *gorm.DB) error {
	if err := migratePlatformSetting[string](tx, "MainHotkey"); err != nil {
		return err
	}
	if err := migratePlatformSetting[string](tx, "SelectionHotkey"); err != nil {
		return err
	}
	if err := migratePlatformSetting[[]setting.IgnoredHotkeyApp](tx, "IgnoredHotkeyApps"); err != nil {
		return err
	}
	if err := migratePlatformSetting[[]setting.QueryHotkey](tx, "QueryHotkeys"); err != nil {
		return err
	}
	if err := migratePlatformSetting[bool](tx, "EnableAutostart"); err != nil {
		return err
	}
	if err := migratePlatformSetting[bool](tx, "HttpProxyEnabled"); err != nil {
		return err
	}
	if err := migratePlatformSetting[string](tx, "HttpProxyUrl"); err != nil {
		return err
	}
	if err := migratePlatformSetting[string](tx, "CustomPythonPath"); err != nil {
		return err
	}
	if err := migratePlatformSetting[string](tx, "CustomNodejsPath"); err != nil {
		return err
	}
	return migratePlatformSetting[string](tx, "AppFontFamily")
}

// migratePlatformSetting converts one legacy PlatformValue setting and its pending cloud oplogs.
func migratePlatformSetting[T any](tx *gorm.DB, key string) error {
	if err := splitLegacyPlatformSettingRow[T](tx, key); err != nil {
		return err
	}
	return convertLegacyPlatformSettingOplogs[T](tx, key)
}

// splitLegacyPlatformSettingRow expands a base-key JSON row into physical per-platform rows.
func splitLegacyPlatformSettingRow[T any](tx *gorm.DB, key string) error {
	var row database.WoxSetting
	err := tx.Where("key = ?", key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	store := setting.NewWoxSettingStore(tx)
	var values legacyPlatformValues[T]
	if err := json.Unmarshal([]byte(row.Value), &values); err != nil {
		// Malformed legacy rows cannot produce per-platform values. Remove them so
		// the new physical key falls back to its normal default and future
		// snapshots do not keep carrying stale base-key payloads.
		return store.Delete(key)
	}

	for _, platform := range []string{util.PlatformWindows, util.PlatformMacOS, util.PlatformLinux} {
		targetKey := setting.PlatformSettingKey(key, platform)
		var existing database.WoxSetting
		err := tx.Where("key = ?", targetKey).First(&existing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			continue
		}

		value, ok := legacyPlatformValueForPlatform(values, platform)
		if !ok {
			continue
		}
		if err := store.Set(targetKey, value); err != nil {
			return err
		}
	}

	return store.Delete(key)
}

// convertLegacyPlatformSettingOplogs prevents pending base-key oplogs from uploading legacy JSON.
func convertLegacyPlatformSettingOplogs[T any](tx *gorm.DB, key string) error {
	var rows []database.Oplog
	if err := tx.Where("entity_type = ? AND key = ? AND synced_to_cloud = ?", cloudsync.EntityWoxSetting, key, false).Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	currentPlatform := util.GetCurrentPlatform()
	targetKey := setting.PlatformSettingKey(key, currentPlatform)
	for _, row := range rows {
		switch row.Operation {
		case cloudsync.OpUpsert:
			var values legacyPlatformValues[T]
			if err := json.Unmarshal([]byte(row.Value), &values); err == nil {
				value, ok := legacyPlatformValueForPlatform(values, currentPlatform)
				if ok {
					rawValue, serializeErr := setting.SerializeValue(value)
					if serializeErr != nil {
						return serializeErr
					}
					if err := tx.Create(&database.Oplog{
						EntityType: row.EntityType,
						EntityID:   targetKey,
						Operation:  row.Operation,
						Key:        targetKey,
						Value:      rawValue,
						Timestamp:  row.Timestamp,
						SyncAfter:  row.SyncAfter,
					}).Error; err != nil {
						return err
					}
				}
			}
		case cloudsync.OpDelete:
			if err := tx.Create(&database.Oplog{
				EntityType: row.EntityType,
				EntityID:   targetKey,
				Operation:  row.Operation,
				Key:        targetKey,
				Timestamp:  row.Timestamp,
				SyncAfter:  row.SyncAfter,
			}).Error; err != nil {
				return err
			}
		}

		// Old platform-base oplogs would re-upload the full legacy JSON blob.
		// Mark them consumed after translating the current-platform intent.
		if err := tx.Model(&database.Oplog{}).Where("id = ?", row.ID).Update("synced_to_cloud", true).Error; err != nil {
			return err
		}
	}

	return nil
}

// legacyPlatformValueForPlatform extracts one suffix value from the old JSON payload shape.
func legacyPlatformValueForPlatform[T any](value legacyPlatformValues[T], platform string) (T, bool) {
	switch platform {
	case util.PlatformWindows:
		return value.WinValue, true
	case util.PlatformMacOS:
		return value.MacValue, true
	case util.PlatformLinux:
		return value.LinuxValue, true
	default:
		var zero T
		return zero, false
	}
}
