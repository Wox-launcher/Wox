package snippets

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"wox/common"
	"wox/plugin"
	"wox/plugin/system/migrateapply"
	"wox/plugin/system/snippets/keyword"
	"wox/setting"
)

// importMigratedSnippets writes each source record through the active plugin's repository.
func importMigratedSnippets(ctx context.Context, items []migrateapply.Snippet) []migrateapply.Result {
	results := make([]migrateapply.Result, len(items))
	var target *Plugin
	if manager := plugin.GetPluginManager(); manager != nil {
		target, _ = manager.GetSystemPlugin(common.SnippetsPluginID).(*Plugin)
	}
	var repository *Repository
	if target != nil {
		repository = target.settingsRepository()
	}
	for i, item := range items {
		if repository == nil {
			results[i] = migrateapply.Result{Status: "failed", ErrorCode: "unavailable"}
			continue
		}
		if ctx.Err() != nil {
			results[i] = migrateapply.Result{Status: "failed", ErrorCode: "cancelled"}
			continue
		}
		results[i] = repository.importOne(item)
	}
	if target != nil && repository != nil {
		target.refreshExpansion(ctx)
	}
	return results
}

// importOne checks duplicate source identities and keyword conflicts inside the sync transaction.
func (r *Repository) importOne(item migrateapply.Snippet) migrateapply.Result {
	if strings.TrimSpace(item.Title) == "" {
		return migrateapply.Result{Status: "failed", ErrorCode: "invalid"}
	}
	if !keyword.Valid(item.Keyword) {
		return migrateapply.Result{Status: "failed", ErrorCode: "invalid"}
	}
	status := migrateapply.Result{Status: "imported"}
	err := r.store.UpdateWithSync(func(store *setting.PluginSettingStore) ([]setting.PluginSettingChange, error) {
		records, err := listRecords(store)
		if err != nil {
			return nil, err
		}
		for _, current := range records {
			sameSource := current.ImportSource == item.Source && item.Source != "" &&
				((item.SourceID != "" && current.ImportID == item.SourceID) || (item.Fingerprint != "" && current.ImportFingerprint == item.Fingerprint))
			if sameSource {
				if current.Name == item.Title && current.Keyword == item.Keyword && current.Text == item.Text {
					status = migrateapply.Result{Status: "exists"}
				} else {
					status = migrateapply.Result{Status: "conflict", ErrorCode: "conflict"}
				}
				return nil, nil
			}
			if item.Keyword != "" && current.Keyword == item.Keyword {
				status = migrateapply.Result{Status: "conflict", ErrorCode: "conflict"}
				return nil, nil
			}
		}
		record := Record{ID: uuid.NewString(), Name: item.Title, Keyword: item.Keyword, Text: item.Text,
			ImportSource: item.Source,
			ImportID:     item.SourceID, ImportFingerprint: item.Fingerprint}
		records = append(records, record)
		raw, err := json.Marshal(records)
		if err != nil {
			return nil, err
		}
		return []setting.PluginSettingChange{{Key: snippetsSetting, Value: string(raw), Syncable: true}}, nil
	})
	if err != nil {
		return migrateapply.Result{Status: "failed", ErrorCode: "failed"}
	}
	return status
}
