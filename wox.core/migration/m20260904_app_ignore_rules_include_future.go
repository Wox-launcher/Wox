package migration

import (
	"context"
	"encoding/json"
	"strings"
	"wox/database"

	"gorm.io/gorm"
)

func init() {
	Register(&appIgnoreRulesIncludeFutureMigration{})
}

type appIgnoreRulesIncludeFutureMigration struct{}

func (m *appIgnoreRulesIncludeFutureMigration) ID() string {
	return "20260904_app_ignore_rules_include_future"
}

func (m *appIgnoreRulesIncludeFutureMigration) Description() string {
	return "Rewrite legacy Apps IgnoreRules Pattern-only rows to IncludeFuture so they stay dynamic."
}

func (m *appIgnoreRulesIncludeFutureMigration) Up(_ context.Context, tx *gorm.DB) error {
	var rows []database.PluginSetting
	if err := tx.Where("plugin_id = ? AND (key = ? OR key LIKE ?)", appPluginID, "IgnoreRules", "IgnoreRules@%").Find(&rows).Error; err != nil {
		return err
	}

	for _, row := range rows {
		migrated, changed := migrateAppIgnoreRulesJSON(row.Value)
		if !changed {
			continue
		}
		row.Value = migrated
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
		if err := appendPluginSettingUpsertOplog(tx, appPluginID, row.Key, migrated); err != nil {
			return err
		}
	}
	return nil
}

// migrateAppIgnoreRulesJSON turns old {Pattern} rows into {Pattern, IncludeFuture:true}.
func migrateAppIgnoreRulesJSON(value string) (string, bool) {
	if strings.TrimSpace(value) == "" {
		return value, false
	}

	var objects []map[string]any
	if err := json.Unmarshal([]byte(value), &objects); err != nil {
		return value, false
	}

	changed := false
	for _, object := range objects {
		if object == nil {
			continue
		}
		if _, ok := object["IncludeFuture"]; ok {
			continue
		}
		object["IncludeFuture"] = true
		delete(object, "Apps")
		delete(object, "App")
		changed = true
	}
	if !changed {
		return value, false
	}

	payload, err := json.Marshal(objects)
	if err != nil {
		return value, false
	}
	return string(payload), true
}
