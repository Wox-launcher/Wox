package migration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"wox/database"

	"gorm.io/gorm"
)

const (
	oldDictationPluginID = "a3f7b8c2-d1e4-4f6a-9b0c-7e2d1a5f8b3e"
	newDictationPluginID = "eaadbcea-c907-4efe-b09b-dfc56e1dc758"
	oldTimerPluginID     = "a3f8c2e1-9b4d-4e7a-8c1f-2d5e6a7b8c9d"
	newTimerPluginID     = "b18d674a-0ae0-4d20-bd11-c5268b069306"
)

func init() { Register(&standalonePluginsMigration{}) }

type standalonePluginsMigration struct{}

func (*standalonePluginsMigration) ID() string { return "20261106_standalone_timer_dictation" }
func (*standalonePluginsMigration) Description() string {
	return "Migrate built-in Timer and Dictation settings to standalone plugins."
}

func (*standalonePluginsMigration) Up(_ context.Context, tx *gorm.DB) error {
	return migrateStandaloneSettings(tx)
}

// migrateStandaloneSettings preserves the old settings and never overwrites standalone values.
func migrateStandaloneSettings(tx *gorm.DB) error {
	for _, pair := range [][2]string{{oldTimerPluginID, newTimerPluginID}, {oldDictationPluginID, newDictationPluginID}} {
		var rows []database.PluginSetting
		if err := tx.Where("plugin_id = ?", pair[0]).Order("key").Find(&rows).Error; err != nil {
			return err
		}
		// The old default action is stored inside actions; convert it before copying standalone fields.
		if pair[0] == oldDictationPluginID {
			for _, row := range rows {
				if row.Key != "actions" {
					continue
				}
				actions, defaults, err := splitDictationDefaultAction(row.Value)
				if err != nil {
					return err
				}
				for key, value := range defaults {
					if err := copyStandaloneSetting(tx, database.PluginSetting{PluginID: pair[1], Key: key, Value: value, IsLocal: row.IsLocal}); err != nil {
						return err
					}
				}
				row.Value = actions
				row.PluginID = pair[1]
				if err := copyStandaloneSetting(tx, row); err != nil {
					return err
				}
			}
		}
		for _, row := range rows {
			if pair[0] == oldDictationPluginID && row.Key == "actions" {
				continue
			}
			if pair[0] == oldDictationPluginID && (row.Key == "inputDevice" || strings.HasPrefix(row.Key, "inputDevice@")) && row.Value == "system" {
				row.Value = ""
			}
			row.PluginID = pair[1]
			if err := copyStandaloneSetting(tx, row); err != nil {
				return err
			}
		}
	}
	return nil
}

// copyStandaloneSetting retains local-only flags and records synchronized values in the same transaction.
func copyStandaloneSetting(tx *gorm.DB, row database.PluginSetting) error {
	var existing database.PluginSetting
	err := tx.Where("plugin_id = ? AND key = ?", row.PluginID, row.Key).First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err := tx.Create(&row).Error; err != nil {
		return err
	}
	if !row.IsLocal {
		return appendPluginSettingUpsertOplog(tx, row.PluginID, row.Key, row.Value)
	}
	return nil
}

// splitDictationDefaultAction maps the old combined default action to the standalone fields.
func splitDictationDefaultAction(value string) (string, map[string]string, error) {
	var actions []map[string]json.RawMessage
	if strings.TrimSpace(value) == "" {
		return value, nil, nil
	}
	if err := json.Unmarshal([]byte(value), &actions); err != nil {
		return "", nil, fmt.Errorf("migrate dictation actions: %w", err)
	}
	defaults := map[string]string{}
	remaining := make([]map[string]json.RawMessage, 0, len(actions))
	for _, action := range actions {
		var id, kind string
		_ = json.Unmarshal(action["id"], &id)
		_ = json.Unmarshal(action["type"], &kind)
		if id != "default" && kind != "default" {
			remaining = append(remaining, action)
			continue
		}
		defaults["defaultAIModel"] = ""
		defaults["defaultAIRefineEnabled"] = "false"
		for source, target := range map[string]string{"hotkey": "defaultHotkey", "model": "defaultAIModel"} {
			var v string
			if json.Unmarshal(action[source], &v) == nil {
				defaults[target] = v
			}
		}
		var enabled bool
		if json.Unmarshal(action["aiRefineEnabled"], &enabled) == nil {
			if enabled {
				defaults["defaultAIRefineEnabled"] = "true"
			} else {
				defaults["defaultAIRefineEnabled"] = "false"
			}
		}
	}
	data, err := json.Marshal(remaining)
	return string(data), defaults, err
}
