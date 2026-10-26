package plugin

import (
	"context"
	"testing"

	"wox/common"
)

type recordingFallbackPlugin struct {
	calls   int
	results []QueryResult
}

func (p *recordingFallbackPlugin) Init(context.Context, InitParams) {}

func (p *recordingFallbackPlugin) Query(context.Context, Query) QueryResponse {
	return QueryResponse{}
}

func (p *recordingFallbackPlugin) QueryFallback(context.Context, Query) []QueryResult {
	p.calls++
	return p.results
}

// TestQueryFallbackRanksBelowLateResults checks the cached snapshot used by later UI batches.
func TestQueryFallbackRanksBelowLateResults(t *testing.T) {
	initPluginManagerLoadTest(t)
	ctx := context.Background()
	query := Query{Id: "fallback-ranking", SessionId: "session", Type: QueryTypeInput, RawQuery: "unmatched"}
	manager, instance := newTestManagerWithCachedResult(query, QueryResult{Id: "ordinary", Title: "ordinary"})
	instance.Plugin = &recordingFallbackPlugin{results: []QueryResult{{
		Id: "fallback", Title: "fallback", Score: 100, RankAboveUsage: true,
		Icon: common.NewWoxImageEmoji("🔍"),
	}}}
	if !manager.lifecycle.appendPluginInstance(instance) {
		t.Fatal("failed to register fallback plugin")
	}
	response := manager.QueryFallback(ctx, query, nil)
	if len(response.Results) != 1 || response.Results[0].Score >= 0 {
		t.Fatalf("fallback response = %+v, want one negative-score result", response.Results)
	}
	snapshot := manager.BuildQueryResultsSnapshot(query.SessionId, query.Id)
	if len(snapshot) != 2 || snapshot[0].Id != "ordinary" || snapshot[1].Id != "fallback" || snapshot[1].Score != response.Results[0].Score {
		t.Fatalf("snapshot = %+v, want ordinary result before fallback with matching scores", snapshot)
	}
}

type panickingFallbackPlugin struct{}

func (panickingFallbackPlugin) Init(context.Context, InitParams) {}

func (panickingFallbackPlugin) Query(context.Context, Query) QueryResponse {
	return QueryResponse{}
}

func (panickingFallbackPlugin) QueryFallback(context.Context, Query) []QueryResult {
	panic("disabled plugin must not run fallback")
}

func TestQueryFallbackSkipsDisabledPlugins(t *testing.T) {
	manager := &Manager{}
	disabled := newDisabledPluginInstance(t, "disabled-fallback", false)
	disabled.Plugin = panickingFallbackPlugin{}
	disabled.Metadata.Name = "Disabled Fallback"
	if !manager.lifecycle.appendPluginInstance(disabled) {
		t.Fatal("failed to register the disabled fallback plugin")
	}

	enabled := &recordingFallbackPlugin{}
	if !manager.lifecycle.appendPluginInstance(&Instance{
		Metadata: Metadata{Id: "enabled-fallback", Name: "Enabled Fallback"},
		Plugin:   enabled,
	}) {
		t.Fatal("failed to register the enabled fallback plugin")
	}

	response := manager.QueryFallback(context.Background(), Query{
		Type:     QueryTypeInput,
		RawQuery: "the red fortune",
	}, nil)
	if enabled.calls != 1 {
		t.Fatalf("enabled fallback calls = %d, want 1", enabled.calls)
	}
	if len(response.Results) != 0 {
		t.Fatalf("fallback results = %+v", response.Results)
	}
}
