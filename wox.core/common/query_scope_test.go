package common

import "testing"

func TestQueryScopeDeduplicateKeepsFirst(t *testing.T) {
	scope := QueryScope{Plugins: []QueryScopePlugin{
		{PluginID: "a", Command: "one"},
		{PluginID: "", Command: "ignored"},
		{PluginID: "a", Command: "two"},
		{PluginID: "b"},
	}}.Deduplicate()
	if len(scope.Plugins) != 2 {
		t.Fatalf("plugins = %d, want 2", len(scope.Plugins))
	}
	if scope.Plugins[0].PluginID != "a" || scope.Plugins[0].Command != "one" {
		t.Fatalf("first plugin = %+v, want a/one", scope.Plugins[0])
	}
	if scope.Identity() != "a|one,b" {
		t.Fatalf("identity = %q", scope.Identity())
	}
}

func TestPlainQueryIsEmptyIncludesScope(t *testing.T) {
	empty := PlainQuery{}
	if !empty.IsEmpty() {
		t.Fatal("empty plain query should be empty")
	}
	scoped := PlainQuery{QueryScope: QueryScope{Plugins: []QueryScopePlugin{{PluginID: "explorer"}}}}
	if scoped.IsEmpty() {
		t.Fatal("scoped plain query should not be empty")
	}
	if scoped.String() == "" {
		t.Fatal("scoped empty-text query needs a display string")
	}
}

func TestQueryScopeNormalizeForRoutingDoesNotBroadenInvalidScope(t *testing.T) {
	scope := QueryScope{Plugins: []QueryScopePlugin{{PluginID: "   "}}}.NormalizeForRouting()
	if scope.IsEmpty() {
		t.Fatal("explicit invalid scope was normalized into an unscoped query")
	}
	if !scope.Deduplicate().IsEmpty() {
		t.Fatalf("invalid scope unexpectedly produced a routable plugin: %#v", scope)
	}
}
