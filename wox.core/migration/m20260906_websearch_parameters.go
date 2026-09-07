package migration

import (
	"context"
	"encoding/json"
	"strings"
	"wox/database"
	"wox/util/browser"

	"gorm.io/gorm"
)

func init() { Register(&webSearchParametersMigration{}) }

type webSearchParametersMigration struct{}

func (m *webSearchParametersMigration) ID() string { return "20260906_websearch_parameters" }

func (m *webSearchParametersMigration) Description() string {
	return "Migrate Web Search query placeholders to named parameters and normalize legacy browser inheritance."
}

// Up upgrades stored Web Search settings through the runner's transaction and sync conventions.
func (m *webSearchParametersMigration) Up(_ context.Context, tx *gorm.DB) error {
	var rows []database.PluginSetting
	if err := tx.Where("plugin_id = ? AND (key = ? OR key LIKE ?)", webSearchPluginID, "webSearches", "webSearches@%").Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		value, changed := migrateWebSearchParametersJSON(row.Value)
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

// migrateWebSearchParametersJSON preserves unknown fields and malformed payloads instead of decoding into the current plugin model.
func migrateWebSearchParametersJSON(value string) (string, bool) {
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
			if next := migrateWebSearchQueryAliases(title); next != title {
				entry["Title"], _ = json.Marshal(next)
				changed = true
			}
		}
		var urls []string
		if json.Unmarshal(entry["Urls"], &urls) == nil {
			urlsChanged := false
			for i, template := range urls {
				if next := migrateWebSearchQueryAliases(template); next != template {
					urls[i] = next
					urlsChanged = true
				}
			}
			if urlsChanged {
				entry["Urls"], _ = json.Marshal(urls)
				changed = true
			}
		}
		var oldBrowser string
		raw, exists := entry["Browser"]
		if !exists || json.Unmarshal(raw, &oldBrowser) == nil {
			normalized := browser.NormalizeBrowserID(oldBrowser)
			if normalized == "" || normalized == "system" {
				normalized = "default"
			}
			if normalized != oldBrowser {
				entry["Browser"], _ = json.Marshal(normalized)
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

func migrateWebSearchQueryAliases(text string) string {
	text = strings.ReplaceAll(text, "{lower_query}", "{wox:parameter?name=query&case=lower}")
	text = strings.ReplaceAll(text, "{upper_query}", "{wox:parameter?name=query&case=upper}")
	return strings.ReplaceAll(text, "{query}", "{wox:parameter?name=query}")
}
