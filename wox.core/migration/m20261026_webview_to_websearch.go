package migration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"wox/cloudsync"
	"wox/database"

	"gorm.io/gorm"
)

func init() { Register(&webViewToWebSearchMigration{}) }

type webViewToWebSearchMigration struct{}

func (*webViewToWebSearchMigration) ID() string { return "20261026_webview_to_websearch" }

func (*webViewToWebSearchMigration) Description() string {
	return "Move WebView sites into WebSearch triggers without retaining the webview command."
}

// Up moves each settings scope atomically; deleting its source makes repeated runs harmless.
func (*webViewToWebSearchMigration) Up(_ context.Context, tx *gorm.DB) error {
	var globalSearches database.PluginSetting
	err := tx.Where("plugin_id = ? AND key = ?", webSearchPluginID, "webSearches").First(&globalSearches).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	var sources []database.PluginSetting
	if err := tx.Where("plugin_id = ? AND (key = ? OR key LIKE ?)", webViewPluginID, "sites", "sites@%").Order("key").Find(&sources).Error; err != nil {
		return err
	}
	var disabledSetting database.PluginSetting
	err = tx.Where("plugin_id = ? AND key = ?", webViewPluginID, "Disabled").First(&disabledSetting).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	pluginDisabled := disabledSetting.Value == "true"
	for _, source := range sources {
		target := database.PluginSetting{PluginID: webSearchPluginID, Key: "webSearches" + strings.TrimPrefix(source.Key, "sites")}
		err := tx.Where("plugin_id = ? AND key = ?", target.PluginID, target.Key).First(&target).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) && source.Key != "sites" {
			// A new platform override must retain searches previously inherited from the global table.
			target.Value, target.IsLocal = globalSearches.Value, globalSearches.IsLocal
		}
		value, added, err := mergeWebViewSites(source.Value, target.Value, pluginDisabled)
		if err != nil {
			return fmt.Errorf("migrate WebView %s: %w", source.Key, err)
		}
		if added {
			target.Value = value
			// Never publish local-only sites by merging them into a previously synced setting.
			target.IsLocal = target.IsLocal || source.IsLocal
			if err := tx.Save(&target).Error; err != nil {
				return err
			}
			if target.IsLocal {
				if err := tx.Model(&database.Oplog{}).
					Where("entity_type = ? AND entity_id = ? AND key = ? AND synced_to_cloud = ? AND cloud_sync_discarded = ?",
						cloudsync.EntityPluginSetting, target.PluginID, target.Key, false, false).
					Update("cloud_sync_discarded", true).Error; err != nil {
					return err
				}
			} else {
				if err := appendPluginSettingUpsertOplog(tx, target.PluginID, target.Key, target.Value); err != nil {
					return err
				}
			}
		}
		if err := tx.Delete(&source).Error; err != nil {
			return err
		}
		if !source.IsLocal {
			if err := appendPluginSettingDeleteOplog(tx, source.PluginID, source.Key); err != nil {
				return err
			}
		}
	}
	return nil
}

// mergeWebViewSites preserves unknown fields and existing searches; collisions get a new trigger.
func mergeWebViewSites(source, target string, pluginDisabled bool) (string, bool, error) {
	var sites, searches []map[string]json.RawMessage
	if strings.TrimSpace(source) != "" {
		if err := json.Unmarshal([]byte(source), &sites); err != nil {
			return "", false, err
		}
	}
	if strings.TrimSpace(target) != "" {
		if err := json.Unmarshal([]byte(target), &searches); err != nil {
			return "", false, err
		}
	}
	used := map[string]bool{"*": true}
	for _, search := range searches {
		var keyword string
		_ = json.Unmarshal(search["Keyword"], &keyword)
		used[strings.ToLower(keyword)] = true
	}
	added := false
	for _, site := range sites {
		if site == nil {
			continue
		}
		var keyword, url string
		if json.Unmarshal(site["Keyword"], &keyword) != nil || strings.TrimSpace(keyword) == "" ||
			json.Unmarshal(site["Url"], &url) != nil || strings.TrimSpace(url) == "" {
			return "", false, fmt.Errorf("site has an invalid keyword or URL")
		}
		title := strings.TrimSpace(keyword)
		// Site commands could contain spaces; a WebSearch trigger must be one token.
		keyword = strings.Join(strings.Fields(keyword), "-")
		base := keyword
		for suffix := 1; used[strings.ToLower(keyword)]; suffix++ {
			keyword = base + "-web"
			if suffix > 1 {
				keyword = fmt.Sprintf("%s-web-%d", base, suffix)
			}
		}
		used[strings.ToLower(keyword)] = true
		site["Keyword"], _ = json.Marshal(keyword)
		site["Title"], _ = json.Marshal(title)
		site["Urls"], _ = json.Marshal([]string{url})
		site["OpenInWebView"] = json.RawMessage("true")
		if css, ok := site["InjectCss"]; ok {
			site["InjectCSS"] = css
		}
		if _, ok := site["KeepInBackground"]; !ok {
			// Older WebView entries kept pages alive unless CacheDisabled was explicitly true.
			var disabled bool
			_ = json.Unmarshal(site["CacheDisabled"], &disabled)
			site["KeepInBackground"], _ = json.Marshal(!disabled)
		}
		if pluginDisabled {
			site["Disabled"] = json.RawMessage("true")
		}
		delete(site, "Url")
		delete(site, "InjectCss")
		delete(site, "CacheDisabled")
		searches = append(searches, site)
		added = true
	}
	if !added {
		return target, false, nil
	}
	payload, err := json.Marshal(searches)
	return string(payload), true, err
}
