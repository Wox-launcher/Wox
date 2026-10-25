package websearch

import (
	"context"
	"encoding/json"
	"strings"

	"wox/common"
	"wox/plugin"
	"wox/plugin/system/migrateapply"
)

const (
	importStatusImported = "imported"
	importStatusExists   = "exists"
	importStatusConflict = "conflict"
	importStatusFailed   = "failed"
)

// ImportedSearch is one quicklink mapped onto a Web Search row.
type ImportedSearch struct {
	Title   string
	Keyword string
	URL     string
	Browser string
}

// ImportedSearchResult is the outcome for one ImportedSearch.
type ImportedSearchResult struct {
	Status    string
	ErrorCode string
}

// ImportSearches appends searches and registers their keywords. An existing keyword is not overwritten.
func (r *WebSearchPlugin) ImportSearches(ctx context.Context, items []ImportedSearch) []ImportedSearchResult {
	results := make([]ImportedSearchResult, len(items))
	if r == nil || r.api == nil {
		for index := range results {
			results[index] = ImportedSearchResult{Status: importStatusFailed, ErrorCode: "unavailable"}
		}
		return results
	}
	icons := make([]common.WoxImage, len(items))
	for index, item := range items {
		link := strings.TrimSpace(item.URL)
		if link == "" {
			continue
		}
		icons[index] = r.indexWebSearchIcon(ctx, webSearch{Urls: []string{link}})
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.webSearches = r.loadWebSearches(ctx)
	changed := false
	for index, item := range items {
		results[index], changed = r.importOneSearch(ctx, item, icons[index], changed)
	}
	if changed {
		raw, err := json.Marshal(r.webSearches)
		if err != nil {
			for index := range results {
				if results[index].Status == importStatusImported {
					results[index] = ImportedSearchResult{Status: importStatusFailed, ErrorCode: "failed"}
				}
			}
		} else {
			r.api.SaveSetting(ctx, webSearchesSettingKey, string(raw), false)
			r.webSearches = r.loadWebSearches(ctx)
		}
	}
	// Loading replaces the structs and clears triggerRegistered. Register again even when
	// nothing was appended, or an existing keyword stays routed and returns no result.
	r.registerTriggerKeywords(ctx)
	return results
}

func (r *WebSearchPlugin) importOneSearch(ctx context.Context, item ImportedSearch, icon common.WoxImage, changed bool) (ImportedSearchResult, bool) {
	keyword := strings.TrimSpace(item.Keyword)
	link := strings.TrimSpace(item.URL)
	title := strings.TrimSpace(item.Title)
	browser := strings.TrimSpace(item.Browser)
	if browser == "" {
		browser = webSearchBrowserSystem
	}
	if keyword == "" || link == "" || title == "" {
		return ImportedSearchResult{Status: importStatusFailed, ErrorCode: "failed"}, changed
	}
	if icon.IsEmpty() {
		icon = webSearchIcon
	}
	candidate := webSearch{Title: title, Keyword: keyword, Urls: []string{link}, Browser: browser, Icon: icon}
	if _, err := candidate.parameters(); err != nil {
		return ImportedSearchResult{Status: importStatusFailed, ErrorCode: "placeholder"}, changed
	}
	for _, existing := range r.webSearches {
		if !strings.EqualFold(existing.Keyword, keyword) {
			continue
		}
		if sameImportedSearch(existing, candidate) {
			return ImportedSearchResult{Status: importStatusExists}, changed
		}
		return ImportedSearchResult{Status: importStatusConflict, ErrorCode: "conflict"}, changed
	}
	registered := r.api.RegisterTriggerKeyword(ctx, plugin.RegisterTriggerKeywordOption{Keyword: keyword})
	if !registered.Success {
		return ImportedSearchResult{Status: importStatusFailed, ErrorCode: "keyword_taken"}, changed
	}
	r.webSearches = append(r.webSearches, candidate)
	return ImportedSearchResult{Status: importStatusImported}, true
}

func sameImportedSearch(existing, candidate webSearch) bool {
	existingBrowser := existing.Browser
	if existingBrowser == "" || existingBrowser == webSearchBrowserUseDefault {
		existingBrowser = webSearchBrowserSystem
	}
	candidateBrowser := candidate.Browser
	if candidateBrowser == "" || candidateBrowser == webSearchBrowserUseDefault {
		candidateBrowser = webSearchBrowserSystem
	}
	existingURL := ""
	if len(existing.Urls) > 0 {
		existingURL = existing.Urls[0]
	}
	return existing.Title == candidate.Title && existingURL == candidate.Urls[0] && existingBrowser == candidateBrowser
}

func importMigratedQuicklinks(ctx context.Context, items []migrateapply.Quicklink) []migrateapply.Result {
	results := make([]migrateapply.Result, len(items))
	var target *WebSearchPlugin
	if manager := plugin.GetPluginManager(); manager != nil {
		target, _ = manager.GetSystemPlugin(common.WebSearchPluginID).(*WebSearchPlugin)
	}
	if target == nil {
		for index := range results {
			results[index] = migrateapply.Result{Status: importStatusFailed, ErrorCode: "unavailable"}
		}
		return results
	}
	incoming := make([]ImportedSearch, len(items))
	for index, item := range items {
		incoming[index] = ImportedSearch{Title: item.Title, Keyword: item.Keyword, URL: item.URL, Browser: item.Browser}
	}
	for index, result := range target.ImportSearches(ctx, incoming) {
		results[index] = migrateapply.Result{Status: result.Status, ErrorCode: result.ErrorCode}
	}
	return results
}
