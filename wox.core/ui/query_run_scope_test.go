package ui

import (
	"context"
	"testing"

	"wox/common"
	"wox/plugin"
	"wox/ui/contract"
)

func TestQueryRunKeepsCanonicalContextForMultiPluginScope(t *testing.T) {
	query := plugin.Query{
		Id:   "query",
		Type: plugin.QueryTypeInput,
		Scope: common.QueryScope{Plugins: []common.QueryScopePlugin{
			{PluginID: "one"},
			{PluginID: "two"},
		}},
	}
	run := newQueryRun(context.Background(), contract.QueryRequest{}, nil, query, nil)
	run.addResponse(plugin.QueryResponseUI{
		Context: plugin.QueryContext{PluginId: "one"},
	})
	if run.latestResponse.Context.IsGlobalQuery || run.latestResponse.Context.PluginId != "" {
		t.Fatalf("multi-scope context = %#v, want non-global without owner", run.latestResponse.Context)
	}
}
