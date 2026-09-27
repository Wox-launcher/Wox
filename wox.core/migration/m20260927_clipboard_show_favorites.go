package migration

import (
	"context"
	"errors"
	"wox/database"
	"wox/setting"

	"gorm.io/gorm"
)

const (
	clipboardPluginID                = "5f815d98-27f5-488d-a756-c317ea39935b"
	legacyHideFavoritesSettingKey    = "hide_favorites_in_default_search"
	showFavoritesByDefaultSettingKey = "show_favorites_by_default"
)

func init() { Register(&clipboardShowFavoritesMigration{}) }

type clipboardShowFavoritesMigration struct{}

func (m *clipboardShowFavoritesMigration) ID() string { return "20260927_clipboard_show_favorites" }

func (m *clipboardShowFavoritesMigration) Description() string {
	return "Keep existing users' clipboard favorite visibility when the new default becomes hidden."
}

// IsNeeded leaves fresh installs on the new default and upgrades existing settings or profiles.
func (m *clipboardShowFavoritesMigration) IsNeeded(_ context.Context, db *gorm.DB) (bool, error) {
	var count int64
	if err := db.Model(&database.PluginSetting{}).Where("plugin_id = ? AND key = ?", clipboardPluginID, legacyHideFavoritesSettingKey).Count(&count).Error; err != nil || count > 0 {
		return count > 0, err
	}
	if err := db.Model(&database.PluginSetting{}).Where("plugin_id = ? AND key = ?", clipboardPluginID, showFavoritesByDefaultSettingKey).Count(&count).Error; err != nil || count > 0 {
		return false, err
	}
	existing, err := hasExistingWoxUserSettings(db)
	if err != nil || existing {
		return existing, err
	}
	if err := db.Model(&database.PluginSetting{}).Where("plugin_id = ?", clipboardPluginID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Up preserves an explicit old choice and fills in the previous visible default for existing profiles.
func (m *clipboardShowFavoritesMigration) Up(_ context.Context, tx *gorm.DB) error {
	var legacy database.PluginSetting
	legacyErr := tx.Where("plugin_id = ? AND key = ?", clipboardPluginID, legacyHideFavoritesSettingKey).First(&legacy).Error
	if legacyErr != nil && !errors.Is(legacyErr, gorm.ErrRecordNotFound) {
		return legacyErr
	}
	var current database.PluginSetting
	currentErr := tx.Where("plugin_id = ? AND key = ?", clipboardPluginID, showFavoritesByDefaultSettingKey).First(&current).Error
	if currentErr != nil && !errors.Is(currentErr, gorm.ErrRecordNotFound) {
		return currentErr
	}
	store := setting.NewPluginSettingStore(tx, clipboardPluginID)
	if errors.Is(currentErr, gorm.ErrRecordNotFound) {
		show := "true"
		if legacyErr == nil {
			var hide string
			if err := store.Get(legacyHideFavoritesSettingKey, &hide); err != nil {
				return err
			}
			if hide == "true" {
				show = "false"
			}
		}
		if err := store.SetWithSync(showFavoritesByDefaultSettingKey, show, legacyErr != nil || !legacy.IsLocal); err != nil {
			return err
		}
	}
	if legacyErr == nil {
		return store.DeleteWithSync(legacyHideFavoritesSettingKey, !legacy.IsLocal)
	}
	return nil
}
