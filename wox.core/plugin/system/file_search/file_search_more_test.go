package system

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
	"wox/common"
	"wox/plugin"
	"wox/util/filesearch"
)

type fileSearchMoreTestAPI struct {
	fileSearchNavigationAPI
	limit    string
	keywords []string
}

func (a *fileSearchMoreTestAPI) GetTriggerKeywords(context.Context, plugin.GetTriggerKeywordsOption) plugin.GetTriggerKeywordsResult {
	if a.keywords == nil {
		return plugin.GetTriggerKeywordsResult{TriggerKeywords: []string{"*", "f"}}
	}
	return plugin.GetTriggerKeywordsResult{TriggerKeywords: a.keywords}
}

func (a *fileSearchMoreTestAPI) GetSetting(_ context.Context, key string) string {
	if key == fileGlobalResultLimitSettingKey {
		return a.limit
	}
	return ""
}

func (a *fileSearchMoreTestAPI) GetTranslation(ctx context.Context, key string) string {
	switch key {
	case "plugin_file_search_more":
		return "Search for more “%s” in File Search"
	case "plugin_file_search_more_subtitle":
		return "Showing the first %d results"
	default:
		return a.fileSearchToolbarTestAPI.GetTranslation(ctx, key)
	}
}

// TestFileSearchMoreResult checks truncation and prefixed queries with configured triggers, commands, and content hits.
func TestFileSearchMoreResult(t *testing.T) {
	for _, tc := range []struct {
		name        string
		count       int
		limit       string
		trigger     string
		scope       common.QueryScope
		content     bool
		visible     int
		more        bool
		keywords    []string
		wantTrigger string
		command     string
	}{
		{name: "empty"},
		{name: "below cap", count: 4, visible: 4},
		{name: "exact cap", count: 5, visible: 5},
		{name: "overflow", count: 6, visible: 5, more: true},
		{name: "content overflow", count: 6, content: true, visible: 5, more: true},
		{name: "custom cap", count: 3, limit: "2", visible: 2, more: true},
		{name: "custom exact cap", count: 2, limit: "2", visible: 2},
		{name: "custom trigger", count: 6, keywords: []string{"", "*", "files", "other"}, wantTrigger: "files", visible: 5, more: true},
		{name: "command", count: 6, command: "find", visible: 5, more: true},
		{name: "global only", count: 6, keywords: []string{"*"}, visible: 5},
		{name: "no triggers", count: 6, keywords: []string{}, visible: 5},
		{name: "triggered", count: 6, trigger: "files", visible: 6},
		{name: "scoped", count: 6, scope: common.QueryScope{Plugins: []common.QueryScopePlugin{{PluginID: PluginID}}}, visible: 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := &fileSearchMoreTestAPI{limit: tc.limit, keywords: tc.keywords}
			search := &FileSearchPlugin{api: api}
			query := plugin.Query{
				Type: plugin.QueryTypeInput, Search: `ext:go "项目 files" 100%`,
				TriggerKeyword: tc.trigger, Scope: tc.scope, Command: tc.command,
				Refinements: map[string]string{fileSearchTypeRefinementKey: fileSearchTypeRefinementFile},
			}
			items := make([]filesearch.SearchResult, tc.count)
			for i := range items {
				name := fmt.Sprintf("report-%d.txt", i)
				items[i] = filesearch.SearchResult{Path: filepath.Join(t.TempDir(), name), Name: name, Score: int64(tc.count - i), IsContentMatch: tc.content}
			}
			results := search.materializeFileSearchResults(t.Context(), query, items, false, &fileSearchQueryDiagnostics{})
			wantCount := tc.visible
			if tc.more {
				wantCount++
			}
			if len(results) != wantCount {
				t.Fatalf("result count = %d, want %d", len(results), wantCount)
			}
			for i := 0; i < tc.visible; i++ {
				if results[i].IdentityKey != items[i].Path {
					t.Fatalf("file result %d was replaced by navigation", i)
				}
			}
			if !tc.more {
				return
			}
			more := results[len(results)-1]
			if more.Title != fmt.Sprintf("Search for more “%s” in File Search", query.Search) || more.SubTitle != fmt.Sprintf("Showing the first %d results", tc.visible) {
				t.Fatalf("more result text = %q / %q", more.Title, more.SubTitle)
			}
			if more.Group != results[0].Group || more.GroupScore != results[0].GroupScore || more.Score >= 0 {
				t.Fatal("more result must sort below file hits in the same group")
			}
			if more.IdentityKey != "" || more.Preview.PreviewData != "" || more.DragData != nil {
				t.Fatal("more result must not act as a file")
			}
			if len(more.Actions) != 1 || !more.Actions[0].PreventHideAfterAction {
				t.Fatal("more result must keep Wox visible")
			}
			more.Actions[0].Action(t.Context(), plugin.ActionContext{})
			trigger := tc.wantTrigger
			if trigger == "" {
				trigger = "f"
			}
			wantText := trigger + " "
			if tc.command != "" {
				wantText += tc.command + " "
			}
			wantText += query.Search
			if api.query.QueryType != plugin.QueryTypeInput || api.query.QueryText != wantText || !api.query.QueryScope.IsEmpty() || !reflect.DeepEqual(api.query.QueryRefinements, query.Refinements) {
				t.Fatalf("more result did not preserve the prefixed query: %+v", api.query)
			}
		})
	}
}
