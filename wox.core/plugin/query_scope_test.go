package plugin

import (
	"testing"
	"wox/common"
)

func TestQueryIsGlobalQueryRespectsScope(t *testing.T) {
	query := Query{Type: QueryTypeInput, Search: "desktop"}
	if !query.IsGlobalQuery() {
		t.Fatal("unscoped input without keyword should be global")
	}
	query.Scope = common.QueryScope{Plugins: []common.QueryScopePlugin{{PluginID: "explorer"}}}
	if query.IsGlobalQuery() {
		t.Fatal("scoped input must not be global")
	}
}

func TestPrimaryTriggerKeywordSkipsStar(t *testing.T) {
	instance := &Instance{Metadata: Metadata{TriggerKeywords: []string{"*", "selection"}}}
	if got := instance.PrimaryTriggerKeyword(); got != "selection" {
		t.Fatalf("PrimaryTriggerKeyword = %q, want selection", got)
	}
}

func TestQueryScopeIdentity(t *testing.T) {
	scope := common.QueryScope{Plugins: []common.QueryScopePlugin{
		{PluginID: "selection", Command: "preview"},
	}}
	if scope.Identity() != "selection|preview" {
		t.Fatalf("identity = %q", scope.Identity())
	}
}

func TestPlainQueryForHistoryPreservesScope(t *testing.T) {
	query := Query{
		Type:     QueryTypeInput,
		RawQuery: "desktop",
		Scope: common.QueryScope{Plugins: []common.QueryScopePlugin{{
			PluginID: "explorer",
			Command:  "browse",
		}}},
	}
	history := plainQueryForHistory(query)
	if history.QueryScope.Identity() != "explorer|browse" {
		t.Fatalf("history scope = %q, want explorer|browse", history.QueryScope.Identity())
	}
	query.Scope.Plugins[0].Command = "changed"
	if history.QueryScope.Identity() != "explorer|browse" {
		t.Fatal("history scope shares the query's plugin slice")
	}
}

func TestScopeOwnerPluginRequiresExactlyOneAllowlistEntry(t *testing.T) {
	query := Query{Scope: common.QueryScope{Plugins: []common.QueryScopePlugin{
		{PluginID: "available"},
		{PluginID: "missing"},
	}}}
	available := &Instance{Metadata: Metadata{Id: "available"}}
	owner := ScopeOwnerPlugin(query, func(pluginID string) *Instance {
		if pluginID == available.Metadata.Id {
			return available
		}
		return nil
	})
	if owner != nil {
		t.Fatalf("multi-plugin scope owner = %q, want none", owner.Metadata.Id)
	}
}
