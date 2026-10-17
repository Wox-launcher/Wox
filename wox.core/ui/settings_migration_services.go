package ui

import (
	"context"

	"wox/common"
	"wox/plugin"
	"wox/plugin/system/migrateapply"
	"wox/plugin/system/notes"
	"wox/ui/contract"
)

func (s *CoreServices) ImportMigratedQuicklinks(ctx context.Context, _ string, items []contract.MigratedQuicklink) []contract.MigratedItemResult {
	results := make([]contract.MigratedItemResult, len(items))
	if migrateapply.ImportQuicklinks == nil {
		return migrationUnavailable(results, migrationIDs(len(items), func(index int) string { return items[index].ItemID }))
	}
	incoming := make([]migrateapply.Quicklink, len(items))
	for index, item := range items {
		incoming[index] = migrateapply.Quicklink{Title: item.Title, Keyword: item.Keyword, URL: item.URL, Browser: item.Browser}
	}
	for index, result := range migrateapply.ImportQuicklinks(ctx, incoming) {
		results[index] = contract.MigratedItemResult{ID: items[index].ItemID, Status: result.Status, ErrorCode: result.ErrorCode}
	}
	return results
}

func (s *CoreServices) ImportMigratedFavorites(ctx context.Context, _ string, items []contract.MigratedFavorite) []contract.MigratedItemResult {
	results := make([]contract.MigratedItemResult, len(items))
	if migrateapply.ImportFavorites == nil {
		return migrationUnavailable(results, migrationIDs(len(items), func(index int) string { return items[index].ItemID }))
	}
	incoming := make([]migrateapply.Favorite, len(items))
	for index, item := range items {
		incoming[index] = migrateapply.Favorite{Name: item.Name, Path: item.Path}
	}
	for index, result := range migrateapply.ImportFavorites(ctx, incoming) {
		results[index] = contract.MigratedItemResult{ID: items[index].ItemID, Status: result.Status, ErrorCode: result.ErrorCode}
	}
	return results
}

func (s *CoreServices) ImportMigratedNotes(ctx context.Context, _ string, items []contract.MigratedNote) []contract.MigratedItemResult {
	_ = ctx
	results := make([]contract.MigratedItemResult, len(items))
	target, _ := systemPlugin(common.NotesPluginID).(*notes.Plugin)
	if target == nil {
		return migrationUnavailable(results, migrationIDs(len(items), func(index int) string { return items[index].ItemID }))
	}
	incoming := make([]notes.ImportedNote, len(items))
	for index, item := range items {
		incoming[index] = notes.ImportedNote{
			Source: item.Source, SourceID: item.SourceID, Fingerprint: item.Fingerprint,
			Title: item.Title, Body: item.Body, Document: item.Document,
		}
	}
	for index, result := range target.ImportPlainNotes(incoming) {
		results[index] = contract.MigratedItemResult{ID: items[index].ItemID, Status: result.Status, ErrorCode: result.ErrorCode}
	}
	return results
}

func (s *CoreServices) ImportMigratedClipboard(ctx context.Context, _ string, items []contract.MigratedClipboard) []contract.MigratedItemResult {
	results := make([]contract.MigratedItemResult, len(items))
	if migrateapply.ImportClipboardItems == nil {
		return migrationUnavailable(results, migrationIDs(len(items), func(index int) string { return items[index].ItemID }))
	}
	incoming := make([]migrateapply.ClipboardItem, len(items))
	for index, item := range items {
		incoming[index] = migrateapply.ClipboardItem{Text: item.Text, ImagePath: item.ImagePath, CreatedAt: item.CreatedAt, Pinned: item.Pinned}
	}
	for index, result := range migrateapply.ImportClipboardItems(ctx, incoming) {
		results[index] = contract.MigratedItemResult{ID: items[index].ItemID, Status: result.Status, ErrorCode: result.ErrorCode}
	}
	return results
}

func (s *CoreServices) ImportMigratedResultBindings(ctx context.Context, _ string, items []contract.MigratedResultBinding) []contract.MigratedItemResult {
	results := make([]contract.MigratedItemResult, len(items))
	manager := plugin.GetPluginManager()
	if manager == nil {
		return migrationUnavailable(results, migrationIDs(len(items), func(index int) string { return items[index].ItemID }))
	}
	for index, item := range items {
		query := plugin.ResultBindingQuery{Search: item.Search, Command: item.Command, TriggerKeyword: item.TriggerKeyword}.PluginQuery()
		found, err := manager.QueryPlugin(ctx, item.PluginID, query)
		picked, ok := plugin.SelectQueryResult(found, plugin.ResultBindingSelector{
			IdentityKey: item.IdentityKey, Path: item.Path, Title: item.Title, Keyword: item.Keyword,
			RequireSettingsPath: item.RequireSettingsPath,
		})
		if !ok && item.TriggerKeyword != "" && len(found) > 0 {
			picked = found[0]
			ok = true
		}
		if err != nil || !ok {
			results[index] = contract.MigratedItemResult{ID: item.ItemID, Status: "failed", ErrorCode: "missing"}
			continue
		}
		status, _ := manager.UpsertResultBinding(ctx, item.PluginID, query, picked, item.Alias, item.Hotkey)
		switch status {
		case "imported", "exists":
			results[index] = contract.MigratedItemResult{ID: item.ItemID, Status: status}
		case "conflict":
			results[index] = contract.MigratedItemResult{ID: item.ItemID, Status: status, ErrorCode: "conflict"}
		default:
			code := status
			if code == "" || code == "ready" {
				code = "failed"
			}
			results[index] = contract.MigratedItemResult{ID: item.ItemID, Status: "failed", ErrorCode: code}
		}
	}
	return results
}

func systemPlugin(id string) any {
	manager := plugin.GetPluginManager()
	if manager == nil {
		return nil
	}
	return manager.GetSystemPlugin(id)
}

func migrationUnavailable(results []contract.MigratedItemResult, ids []string) []contract.MigratedItemResult {
	for index := range results {
		id := ""
		if index < len(ids) {
			id = ids[index]
		}
		results[index] = contract.MigratedItemResult{ID: id, Status: "failed", ErrorCode: "unavailable"}
	}
	return results
}

func migrationIDs(count int, id func(int) string) []string {
	ids := make([]string, count)
	for index := 0; index < count; index++ {
		ids[index] = id(index)
	}
	return ids
}
