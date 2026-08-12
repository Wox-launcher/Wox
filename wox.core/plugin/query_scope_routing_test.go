package plugin

import (
	"testing"
	"wox/common"
)

func TestApplyScopeForPluginOverridesCommand(t *testing.T) {
	manager := &Manager{}
	instance := &Instance{
		Metadata: Metadata{
			Id:              "d9e557ed-89bd-4b8b-bd64-2a7632cf3483",
			TriggerKeywords: []string{"*", "selection"},
		},
	}
	query := Query{
		Type:  QueryTypeSelection,
		Scope: common.QueryScope{Plugins: []common.QueryScopePlugin{{PluginID: instance.Metadata.Id, Command: "preview"}}},
	}
	scoped := manager.applyScopeForPlugin(query, instance)
	if scoped.Command != "preview" {
		t.Fatalf("command = %q, want preview", scoped.Command)
	}
	if scoped.TriggerKeyword != "selection" {
		t.Fatalf("trigger = %q, want selection", scoped.TriggerKeyword)
	}
}

func TestHasScopeAndIsGlobalQuery(t *testing.T) {
	query := Query{
		Type:  QueryTypeInput,
		Scope: common.QueryScope{Plugins: []common.QueryScopePlugin{{PluginID: "explorer"}}},
	}
	if !query.HasScope() {
		t.Fatal("expected HasScope")
	}
	if query.IsGlobalQuery() {
		t.Fatal("scoped query must not be global")
	}
}
