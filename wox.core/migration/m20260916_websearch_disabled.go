package migration

import (
	"context"
	"encoding/json"
	"wox/database"

	"gorm.io/gorm"
)

func init() { Register(&webSearchDisabledMigration{}) }

type webSearchDisabledMigration struct{}

func (m *webSearchDisabledMigration) ID() string { return "20260916_websearch_disabled" }

func (m *webSearchDisabledMigration) Description() string {
	return "Replace Web Search Enabled with Disabled so new rows stay on unless the user opts out."
}

// Up inverts stored Enabled flags through the runner's transaction and sync conventions.
func (m *webSearchDisabledMigration) Up(_ context.Context, tx *gorm.DB) error {
	var rows []database.PluginSetting
	if err := tx.Where("plugin_id = ? AND (key = ? OR key LIKE ?)", webSearchPluginID, "webSearches", "webSearches@%").Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		value, changed := migrateWebSearchDisabledJSON(row.Value)
		if !changed {
			continue
		}
		row.Value = value
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
		if !row.IsLocal {
			if err := appendPluginSettingUpsertOplog(tx, webSearchPluginID, row.Key, value); err != nil {
				return err
			}
		}
	}
	return nil
}

// migrateWebSearchDisabledJSON preserves unknown fields. Legacy rows without Enabled were off.
func migrateWebSearchDisabledJSON(value string) (string, bool) {
	var entries []map[string]json.RawMessage
	if json.Unmarshal([]byte(value), &entries) != nil {
		return value, false
	}
	changed := false
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		if _, exists := entry["Disabled"]; exists {
			continue
		}
		disabled := true
		if raw, exists := entry["Enabled"]; exists {
			var enabled bool
			if json.Unmarshal(raw, &enabled) == nil {
				disabled = !enabled
			}
		}
		payload, err := json.Marshal(disabled)
		if err != nil {
			continue
		}
		entry["Disabled"] = payload
		delete(entry, "Enabled")
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
