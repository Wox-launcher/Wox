package migration

import (
	"context"
	"errors"
	"wox/database"
	"wox/setting"
	"wox/util"

	"gorm.io/gorm"
)

func init() { Register(&actionPanelHotkeyMigration{}) }

type actionPanelHotkeyMigration struct{}

func (m *actionPanelHotkeyMigration) ID() string { return "20260917_action_panel_hotkey" }

func (m *actionPanelHotkeyMigration) Description() string {
	return "Keep existing users on primary+J for Action Hotkey after the new default becomes primary+K."
}

// IsNeeded writes the old shortcut only for databases that already have user settings.
// Fresh installs have no settings besides ThemeId from an earlier migration, so they keep K.
func (m *actionPanelHotkeyMigration) IsNeeded(_ context.Context, db *gorm.DB) (bool, error) {
	existing, err := hasExistingWoxUserSettings(db)
	if err != nil || !existing {
		return false, err
	}
	return hasMissingActionPanelHotkey(db)
}

func (m *actionPanelHotkeyMigration) Up(_ context.Context, tx *gorm.DB) error {
	store := setting.NewWoxSettingStore(tx)
	for _, platform := range []string{util.PlatformWindows, util.PlatformMacOS, util.PlatformLinux} {
		key := setting.PlatformSettingKey("ActionPanelHotkey", platform)
		var existing database.WoxSetting
		err := tx.Where("key = ?", key).First(&existing).Error
		if err == nil {
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := store.SetWithSync(key, setting.LegacyActionPanelHotkeyForPlatform(platform), true); err != nil {
			return err
		}
	}
	return nil
}

func hasExistingWoxUserSettings(db *gorm.DB) (bool, error) {
	var count int64
	err := db.Model(&database.WoxSetting{}).
		Where("key <> ? AND key NOT LIKE ?", "ThemeId", "ActionPanelHotkey@%").
		Count(&count).Error
	return count > 0, err
}

func hasMissingActionPanelHotkey(db *gorm.DB) (bool, error) {
	for _, platform := range []string{util.PlatformWindows, util.PlatformMacOS, util.PlatformLinux} {
		var existing database.WoxSetting
		err := db.Where("key = ?", setting.PlatformSettingKey("ActionPanelHotkey", platform)).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
	}
	return false, nil
}
