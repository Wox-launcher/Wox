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

func TestQueryRunRetainsPluginPreviewOnlyForSingleOwner(t *testing.T) {
	preview := &plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeText, PreviewData: "player"}
	query := plugin.Query{Id: "query", Type: plugin.QueryTypeInput}
	global := newQueryRun(context.Background(), contract.QueryRequest{}, nil, query, nil)
	global.addResponse(plugin.QueryResponseUI{PluginPreview: preview})
	if global.latestResponse.PluginPreview != nil {
		t.Fatal("global query accepted plugin preview")
	}
	scoped := newQueryRun(context.Background(), contract.QueryRequest{}, nil, query, &plugin.Instance{})
	scoped.addResponse(plugin.QueryResponseUI{PluginPreview: preview})
	scoped.addResponse(plugin.QueryResponseUI{})
	if scoped.latestResponse.PluginPreview != preview {
		t.Fatal("scoped query lost plugin preview before flush")
	}
}
