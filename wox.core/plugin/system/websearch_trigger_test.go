package system

import (
	"context"
	"slices"
	"testing"
	"wox/plugin"
)

type webSearchTriggerTestAPI struct {
	plugin.API
	keywords []string
}

func (a *webSearchTriggerTestAPI) RegisterTriggerKeyword(_ context.Context, keyword string) bool {
	if keyword == "occupied" {
		return false
	}
	if !slices.Contains(a.keywords, keyword) {
		a.keywords = append(a.keywords, keyword)
	}
	return true
}

func (a *webSearchTriggerTestAPI) UnregisterTriggerKeyword(_ context.Context, keyword string) {
	a.keywords = slices.DeleteFunc(a.keywords, func(value string) bool { return value == keyword })
}

// TestWebSearchTriggerKeywords ensures disabling and removing searches release their routing keywords.
func TestWebSearchTriggerKeywords(t *testing.T) {
	api := &webSearchTriggerTestAPI{}
	search := &WebSearchPlugin{api: api, webSearches: []webSearch{
		{Keyword: "g", Enabled: true},
		{Keyword: "b", Enabled: false},
		{Keyword: "occupied", Enabled: true},
	}}
	search.registerTriggerKeywords(context.Background())
	if !slices.Equal(api.keywords, []string{"g"}) {
		t.Fatalf("keywords = %v", api.keywords)
	}
	if search.webSearches[2].triggerRegistered {
		t.Fatal("failed registration marked successful")
	}
	for _, query := range []plugin.Query{
		{RawQuery: "occupied test"},
		{RawQuery: "occupied test", TriggerKeyword: "occupied"},
	} {
		if len(search.Query(context.Background(), query).Results) != 0 {
			t.Fatal("failed registration bypassed keyword ownership")
		}
	}
	if len(search.Query(context.Background(), plugin.Query{RawQuery: "g test", TriggerKeyword: "g"}).Results) != 1 {
		t.Fatal("successful registration did not produce a search result")
	}
	search.webSearches[0].Enabled = false
	search.webSearches[1].Enabled = true
	search.webSearches[1].Keyword = "bing"
	search.registerTriggerKeywords(context.Background())
	if !slices.Equal(api.keywords, []string{"bing"}) {
		t.Fatalf("updated keywords = %v", api.keywords)
	}
	search.webSearches = nil
	search.registerTriggerKeywords(context.Background())
	if len(api.keywords) != 0 {
		t.Fatalf("removed keywords = %v", api.keywords)
	}
}
