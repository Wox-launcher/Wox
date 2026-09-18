package migration

import (
	"context"
	"encoding/json"
	"wox/database"

	"gorm.io/gorm"
)

func init() { Register(&webViewKeepInBackgroundMigration{}) }

type webViewKeepInBackgroundMigration struct{}

func (m *webViewKeepInBackgroundMigration) ID() string {
	return "20260918_webview_keep_in_background"
}

func (m *webViewKeepInBackgroundMigration) Description() string {
	return "Replace WebView Disable Cache with Keep running in the background so new sites release memory unless the user opts in."
}

// Up inverts stored CacheDisabled flags through the runner's transaction and sync conventions.
func (m *webViewKeepInBackgroundMigration) Up(_ context.Context, tx *gorm.DB) error {
	var rows []database.PluginSetting
	if err := tx.Where("plugin_id = ? AND (key = ? OR key LIKE ?)", webViewPluginID, "sites", "sites@%").Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		value, changed := migrateWebViewKeepInBackgroundJSON(row.Value)
		if !changed {
			continue
		}
		row.Value = value
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
		if !row.IsLocal {
			if err := appendPluginSettingUpsertOplog(tx, webViewPluginID, row.Key, value); err != nil {
				return err
			}
		}
	}
	return nil
}

// migrateWebViewKeepInBackgroundJSON preserves unknown fields. Legacy rows without CacheDisabled stayed alive.
func migrateWebViewKeepInBackgroundJSON(value string) (string, bool) {
	var entries []map[string]json.RawMessage
	if json.Unmarshal([]byte(value), &entries) != nil {
		return value, false
	}
	changed := false
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		if _, exists := entry["KeepInBackground"]; exists {
			continue
		}
		keepInBackground := true
		if raw, exists := entry["CacheDisabled"]; exists {
			var cacheDisabled bool
			if json.Unmarshal(raw, &cacheDisabled) == nil {
				keepInBackground = !cacheDisabled
			}
		}
		payload, err := json.Marshal(keepInBackground)
		if err != nil {
			continue
		}
		entry["KeepInBackground"] = payload
		delete(entry, "CacheDisabled")
		changed = true
	}
	if !changed {
		return value, false
	}
	next, err := json.Marshal(entries)
	if err != nil {
		return value, false
	}
	return string(next), true
}
