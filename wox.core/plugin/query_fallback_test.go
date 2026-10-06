package plugin

import (
	"context"
	"testing"
)

type recordingFallbackPlugin struct {
	calls int
}

func (p *recordingFallbackPlugin) Init(context.Context, InitParams) {}

func (p *recordingFallbackPlugin) Query(context.Context, Query) QueryResponse {
	return QueryResponse{}
}

func (p *recordingFallbackPlugin) QueryFallback(context.Context, Query) []QueryResult {
	p.calls++
	return nil
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
	if !manager.appendPluginInstance(disabled) {
		t.Fatal("failed to register the disabled fallback plugin")
	}

	enabled := &recordingFallbackPlugin{}
	if !manager.appendPluginInstance(&Instance{
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
