package migration

import (
	"context"
	"errors"
	"wox/cloudsync"
	"wox/database"

	"gorm.io/gorm"
)

const (
	legacyQueryShortcutsSettingKey = "QueryShortcuts"
	queryAliasesSettingKey         = "QueryAliases"
)

func init() { Register(&renameQueryShortcutsMigration{}) }

type renameQueryShortcutsMigration struct{}

func (m *renameQueryShortcutsMigration) ID() string {
	return "20260919_rename_query_shortcuts"
}

func (m *renameQueryShortcutsMigration) Description() string {
	return "Rename persisted QueryShortcuts to QueryAliases so the store key matches the user-facing alias name."
}

func (m *renameQueryShortcutsMigration) Up(_ context.Context, tx *gorm.DB) error {
	if err := renameQueryShortcutsSettingRow(tx); err != nil {
		return err
	}
	return renameQueryShortcutsOplogs(tx)
}

func renameQueryShortcutsSettingRow(tx *gorm.DB) error {
	var legacy database.WoxSetting
	err := tx.Where("key = ?", legacyQueryShortcutsSettingKey).First(&legacy).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	var existing database.WoxSetting
	err = tx.Where("key = ?", queryAliasesSettingKey).First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := tx.Create(&database.WoxSetting{Key: queryAliasesSettingKey, Value: legacy.Value}).Error; err != nil {
			return err
		}
	}
	return tx.Delete(&database.WoxSetting{Key: legacyQueryShortcutsSettingKey}).Error
}

func renameQueryShortcutsOplogs(tx *gorm.DB) error {
	return tx.Model(&database.Oplog{}).
		Where("entity_type = ? AND (key = ? OR entity_id = ?) AND synced_to_cloud = ?", cloudsync.EntityWoxSetting, legacyQueryShortcutsSettingKey, legacyQueryShortcutsSettingKey, false).
		Updates(map[string]any{"key": queryAliasesSettingKey, "entity_id": queryAliasesSettingKey}).Error
}
