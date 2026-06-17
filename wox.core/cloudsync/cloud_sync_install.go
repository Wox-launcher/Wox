package cloudsync

import (
	"context"
	"encoding/json"
	"fmt"
	"wox/database"
	"wox/util"
)

// LogInstalledPluginUpsert records a syncable installed-plugin state change.
func LogInstalledPluginUpsert(ctx context.Context, value InstalledPluginValue) error {
	if value.ID == "" {
		return fmt.Errorf("installed plugin id is empty")
	}
	return logInstallOplog(ctx, EntityInstalledPlugin, value.ID, value, OpUpsert)
}

// LogInstalledPluginDelete records a syncable installed-plugin removal.
func LogInstalledPluginDelete(ctx context.Context, pluginID string) error {
	if pluginID == "" {
		return fmt.Errorf("installed plugin id is empty")
	}
	return logInstallOplog(ctx, EntityInstalledPlugin, pluginID, nil, OpDelete)
}

// LogInstalledThemeUpsert records a syncable installed-theme state change.
func LogInstalledThemeUpsert(ctx context.Context, value InstalledThemeValue) error {
	if value.ID == "" {
		return fmt.Errorf("installed theme id is empty")
	}
	return logInstallOplog(ctx, EntityInstalledTheme, value.ID, value, OpUpsert)
}

// LogInstalledThemeDelete records a syncable installed-theme removal.
func LogInstalledThemeDelete(ctx context.Context, themeID string) error {
	if themeID == "" {
		return fmt.Errorf("installed theme id is empty")
	}
	return logInstallOplog(ctx, EntityInstalledTheme, themeID, nil, OpDelete)
}

// logInstallOplog stores install-list changes in the same encrypted oplog path
// used by settings, with entityID as the stable per-plugin/theme key.
func logInstallOplog(ctx context.Context, entityType string, entityID string, value interface{}, op string) error {
	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database is not initialized")
	}

	rawValue := ""
	if op == OpUpsert {
		encoded, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to serialize install sync value: %w", err)
		}
		rawValue = string(encoded)
	}

	oplog := database.Oplog{
		EntityType: entityType,
		EntityID:   entityID,
		Operation:  op,
		Key:        entityID,
		Value:      rawValue,
		Timestamp:  util.GetSystemTimestamp(),
	}
	if err := db.Create(&oplog).Error; err != nil {
		return err
	}
	NotifyOplogChanged()
	return nil
}
