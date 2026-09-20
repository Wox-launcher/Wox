package websearch

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"wox/plugin"
)

type webSearchTriggerTestAPI struct {
	plugin.API
	keywords []string
}

func (a *webSearchTriggerTestAPI) RegisterTriggerKeyword(_ context.Context, option plugin.RegisterTriggerKeywordOption) plugin.RegisterTriggerKeywordResult {
	keyword := option.Keyword
	if keyword == "occupied" {
		return plugin.RegisterTriggerKeywordResult{Success: false}
	}
	if !slices.Contains(a.keywords, keyword) {
		a.keywords = append(a.keywords, keyword)
	}
	return plugin.RegisterTriggerKeywordResult{Success: true}
}

func (a *webSearchTriggerTestAPI) UnregisterTriggerKeyword(_ context.Context, option plugin.UnregisterTriggerKeywordOption) plugin.UnregisterTriggerKeywordResult {
	a.keywords = slices.DeleteFunc(a.keywords, func(value string) bool { return value == option.Keyword })
	return plugin.UnregisterTriggerKeywordResult{Success: true}
}

// TestWebSearchTriggerKeywords ensures disabling and removing searches release their routing keywords.
func TestWebSearchTriggerKeywords(t *testing.T) {
	api := &webSearchTriggerTestAPI{}
	search := &WebSearchPlugin{api: api, webSearches: []webSearch{
		{Keyword: "g"},
		{Keyword: "b", Disabled: true},
		{Keyword: "occupied"},
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
	search.webSearches[0].Disabled = true
	search.webSearches[1].Disabled = false
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

func TestWebSearchMRURestoreRebuildsResult(t *testing.T) {
	search := webSearch{
		Keyword: "g",
		Title:   "Search {wox:parameter?name=query}",
		Urls:    []string{"https://www.google.com/search?q={wox:parameter?name=query}"},
	}
	p := &WebSearchPlugin{api: &webSearchTriggerTestAPI{}, webSearches: []webSearch{search}}
	encoded, err := json.Marshal(map[string]string{plugin.ParameterQueryVariable("query"): "wox"})
	if err != nil {
		t.Fatal(err)
	}
	restored, err := p.handleMRURestore(context.Background(), plugin.MRUData{
		ContextData: map[string]string{webSearchMRUKeywordKey: "g", webSearchMRUValuesKey: string(encoded)},
	})
	if err != nil {
		t.Fatalf("restore web search: %v", err)
	}
	if restored.Actions[0].ContextData[webSearchMRUKeywordKey] != "g" {
		t.Fatalf("restored context = %#v", restored.Actions[0].ContextData)
	}
	if _, err := p.handleMRURestore(context.Background(), plugin.MRUData{}); err == nil {
		t.Fatal("empty context should fail restore")
	}
	p.webSearches[0].Disabled = true
	if _, err := p.handleMRURestore(context.Background(), plugin.MRUData{
		ContextData: map[string]string{webSearchMRUKeywordKey: "g", webSearchMRUValuesKey: string(encoded)},
	}); err == nil {
		t.Fatal("disabled search should fail restore")
	}
}
