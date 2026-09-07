package migration

import (
	"context"
	"encoding/json"
	"regexp"
	"wox/database"

	"gorm.io/gorm"
)

func init() { Register(&webSearchVariableSyntaxMigration{}) }

type webSearchVariableSyntaxMigration struct{}

func (m *webSearchVariableSyntaxMigration) ID() string { return "20260907_websearch_variable_syntax" }

func (m *webSearchVariableSyntaxMigration) Description() string {
	return "Rewrite Web Search {wox:parameter:name} and {query} placeholders to {wox:parameter?name=}."
}

var webSearchColonParameter = regexp.MustCompile(`\{wox:parameter:([^}?]+)\}`)

// Up upgrades stored Web Search templates through the runner's transaction and sync conventions.
func (m *webSearchVariableSyntaxMigration) Up(_ context.Context, tx *gorm.DB) error {
	var rows []database.PluginSetting
	if err := tx.Where("plugin_id = ? AND (key = ? OR key LIKE ?)", webSearchPluginID, "webSearches", "webSearches@%").Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		value, changed := migrateWebSearchVariableSyntaxJSON(row.Value)
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

// migrateWebSearchVariableSyntaxJSON preserves unknown fields and malformed payloads instead of decoding into the current plugin model.
func migrateWebSearchVariableSyntaxJSON(value string) (string, bool) {
	var entries []map[string]json.RawMessage
	if json.Unmarshal([]byte(value), &entries) != nil {
		return value, false
	}
	changed := false
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		var title string
		if json.Unmarshal(entry["Title"], &title) == nil {
			if next := migrateWebSearchVariableSyntax(title); next != title {
				entry["Title"], _ = json.Marshal(next)
				changed = true
			}
		}
		var urls []string
		if json.Unmarshal(entry["Urls"], &urls) == nil {
			urlsChanged := false
			for i, template := range urls {
				if next := migrateWebSearchVariableSyntax(template); next != template {
					urls[i] = next
					urlsChanged = true
				}
			}
			if urlsChanged {
				entry["Urls"], _ = json.Marshal(urls)
				changed = true
			}
		}
	}
	if !changed {
		return value, false
	}
	payload, err := json.Marshal(entries)
	if err != nil {
		return value, false
	}
	return string(payload), true
}

func migrateWebSearchVariableSyntax(text string) string {
	return migrateWebSearchQueryAliases(webSearchColonParameter.ReplaceAllString(text, "{wox:parameter?name=$1}"))
}
