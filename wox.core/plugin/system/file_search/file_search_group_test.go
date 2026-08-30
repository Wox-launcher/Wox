package system

import (
	"testing"
	"wox/common"
	"wox/plugin"
)

func TestFileSearchResultGroupUsesFilesOnGlobalQuery(t *testing.T) {
	group, score := fileSearchResultGroup(plugin.Query{Type: plugin.QueryTypeInput, Search: "scottqian"})
	if group != "i18n:plugin_file_group" || score != 0 {
		t.Fatalf("global query group = %q/%d, want i18n:plugin_file_group/0", group, score)
	}
}

func TestFileSearchResultGroupStaysEmptyForTriggeredQuery(t *testing.T) {
	group, score := fileSearchResultGroup(plugin.Query{
		Type:           plugin.QueryTypeInput,
		TriggerKeyword: "f",
		Search:         "scottqian",
	})
	if group != "" || score != 0 {
		t.Fatalf("triggered query group = %q/%d, want empty", group, score)
	}
}

func TestFileSearchResultGroupStaysEmptyForScopedQuery(t *testing.T) {
	group, score := fileSearchResultGroup(plugin.Query{
		Type:   plugin.QueryTypeInput,
		Search: "scottqian",
		Scope:  common.QueryScope{Plugins: []common.QueryScopePlugin{{PluginID: PluginID}}},
	})
	if group != "" || score != 0 {
		t.Fatalf("scoped query group = %q/%d, want empty", group, score)
	}
}
