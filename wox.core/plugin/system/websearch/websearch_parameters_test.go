package websearch

import (
	"context"
	"encoding/json"
	"net/url"
	"slices"
	"strings"
	"testing"
	"wox/common"
	"wox/plugin"
	"wox/util/selection"
)

type webSearchParametersTestAPI struct {
	webSearchTriggerTestAPI
	settings map[string]string
	writes   int
	changed  common.PlainQuery
}

func (a *webSearchParametersTestAPI) GetSetting(_ context.Context, key string) string {
	return a.settings[key]
}
func (a *webSearchParametersTestAPI) SaveSetting(_ context.Context, key, value string, _ bool) {
	a.settings[key] = value
	a.writes++
}
func (a *webSearchParametersTestAPI) ChangeQuery(_ context.Context, query common.PlainQuery) {
	a.changed = query
}

// TestWebSearchParameters exercises multi-slot input, paste recovery, encoding and compatibility at plugin boundaries.
func TestWebSearchParameters(t *testing.T) {
	ctx := context.Background()
	search := webSearch{Keyword: "tr", Enabled: true, Title: "{wox:parameter?name=text} → {wox:parameter?name=language}", Urls: []string{
		"https://example.com/?text={wox:parameter?name=text}&lang={wox:parameter?name=language}",
		"https://example.org/{wox:parameter?name=language}?q={wox:parameter?name=text}",
	}}
	names, err := search.parameters()
	if err != nil || !slices.Equal(names, []string{"text", "language"}) {
		t.Fatalf("parameters=%v err=%v", names, err)
	}
	api := &webSearchParametersTestAPI{}
	p := &WebSearchPlugin{api: api, webSearches: []webSearch{search}}
	p.registerTriggerKeywords(ctx)
	query := plugin.Query{Type: plugin.QueryTypeInput, TriggerKeyword: "tr", RawQuery: "tr hello world"}
	response := p.Query(ctx, query)
	if len(response.Results) != 1 || response.Results[0].Title != "i18n:plugin_websearch_fill_parameters" {
		t.Fatal("plain multi-input query was executed")
	}
	response.Results[0].Actions[0].Action(ctx, plugin.ActionContext{})
	query.QueryHint = api.changed.QueryHint
	if query.QueryHint.Argument("parameter:text") != "hello world" {
		t.Fatal("paste was split or lost")
	}
	query.QueryHint.Elements[3].Value = "zh CN"
	query.RawQuery = query.QueryHint.PlainText()
	response = p.Query(ctx, query)
	if response.Results[0].Title != "hello world → zh CN" {
		t.Fatalf("title=%s", response.Results[0].Title)
	}
	values, complete := search.parameterValues(query, names)
	if !complete {
		t.Fatal("complete named inputs rejected")
	}
	resolved := renderWebSearchTemplate(search.Urls[0], values, true)
	parsed, _ := url.Parse(resolved)
	if parsed.Query().Get("text") != "hello world" || parsed.Query().Get("lang") != "zh CN" {
		t.Fatal(resolved)
	}
	values = map[string]string{plugin.ParameterQueryVariable("query"): "Ä &/?#% {wox:clipboard_text}", plugin.QueryVariableClipboardText: "SECRET"}
	for _, token := range []string{plugin.ParameterQueryVariable("query"), "{wox:parameter?name=query&case=lower}", "{wox:parameter?name=query&case=upper}"} {
		resolved := renderWebSearchTemplate("https://example.com/?q="+token, values, true)
		parsed, _ := url.Parse(resolved)
		want := values[plugin.ParameterQueryVariable("query")]
		if token == "{wox:parameter?name=query&case=lower}" {
			want = strings.ToLower(want)
		}
		if token == "{wox:parameter?name=query&case=upper}" {
			want = strings.ToUpper(want)
		}
		if parsed.Query().Get("q") != want || strings.Contains(resolved, "SECRET") {
			t.Fatal(resolved)
		}
	}
	path := renderWebSearchTemplate("https://example.com/"+plugin.ParameterQueryVariable("query"), values, true)
	if !strings.Contains(path, "%2F") || strings.Contains(path, "+") {
		t.Fatal(path)
	}
	for _, template := range []string{
		"https://{wox:parameter?name=host}/",
		"https://example.com/{wox:paramter?name=q}",
		"https://example.com/{wox:parameter}",
		"https://example.com/{wox:parameter?name=q",
		"https://example.com/{wox:parameter:query}",
		"https://example.com/{query}",
		"https://example.com/{wox:parameter?name= query}",
		"https://example.com/{wox:parameter?name=query }",
	} {
		if _, err := (webSearch{Urls: []string{template}}).parameters(); err == nil {
			t.Fatalf("accepted invalid template %q", template)
		}
	}
	spaced := webSearch{Urls: []string{"https://www.google.com/search?q={wox:parameter?name=query 123}&t={wox:parameter?name=time}"}}
	if names, err := spaced.parameters(); err != nil || !slices.Equal(names, []string{"query 123", "time"}) {
		t.Fatalf("spaced parameter names=%v err=%v", names, err)
	}
	chinese := webSearch{
		Title: "搜索 {wox:parameter?name=查询内容}",
		Urls:  []string{"https://www.google.com/search?q={wox:parameter?name=查询内容}"},
	}
	if names, err := chinese.parameters(); err != nil || !slices.Equal(names, []string{"查询内容"}) {
		t.Fatalf("chinese parameter names=%v err=%v", names, err)
	}
	if _, err := (webSearch{Urls: []string{"https://example.com/?q={wox:parameter?name=1query}"}}).parameters(); err == nil {
		t.Fatal("names that start with a digit should stay invalid")
	}
	if got := ValidateSettingFields("", []string{"https://example.com/?q={wox:parameter?name=1query}"}); got["Urls"] != "i18n:plugin_websearch_error_invalid_parameter_name" {
		t.Fatalf("invalid name fields = %#v", got)
	}
	mismatch := webSearch{
		Title: "Search Google for {wox:parameter?name=query}",
		Urls:  []string{"https://www.google.com/search?q={wox:parameter?name=query123}&t={wox:parameter?name=time}"},
	}
	if _, err := mismatch.parameters(); err == nil {
		t.Fatal("title parameter missing from URLs should fail")
	}
	if got := ValidateSettingFields(mismatch.Title, mismatch.Urls); got["Title"] != "i18n:plugin_websearch_error_unknown_title_variable" {
		t.Fatalf("validate fields = %#v", got)
	}
}

// TestWebSearchLegacyFallback keeps load from rewriting stored settings; placeholder upgrades belong to migrations.
func TestWebSearchLegacyFallback(t *testing.T) {
	ctx := context.Background()
	old, _ := json.Marshal([]webSearch{{Keyword: "g", Enabled: true, IsFallback: true, Browser: "default", Title: "{wox:parameter?name=query}", Urls: []string{"https://example.com/?q={wox:parameter?name=query}&lower={wox:parameter?name=query&case=lower}"}}})
	api := &webSearchParametersTestAPI{settings: map[string]string{webSearchesSettingKey: string(old)}}
	p := &WebSearchPlugin{api: api}
	p.webSearches = p.loadWebSearches(ctx)
	if api.writes != 0 || api.settings[webSearchesSettingKey] != string(old) {
		t.Fatal("loading settings rewrote legacy data")
	}
	p.webSearches = p.loadWebSearches(ctx)
	if api.writes != 0 {
		t.Fatal("reloading settings wrote data")
	}
	p.webSearches = append(p.webSearches,
		webSearch{Enabled: true, IsFallback: true, Urls: []string{"https://example.com/"}},
		webSearch{Enabled: true, IsFallback: true, Urls: []string{"https://example.com/?a={wox:parameter?name=a}&b={wox:parameter?name=b}"}},
	)
	query := plugin.Query{RawQuery: "hello world", Type: plugin.QueryTypeSelection, Selection: selection.Selection{Type: selection.SelectionTypeText, Text: "chosen words"}}
	if got := p.QueryFallback(ctx, query); len(got) != 1 || got[0].Title != "hello world" {
		t.Fatalf("fallback=%v", got)
	}
	if got := p.querySelection(ctx, query); len(got) != 1 || got[0].Title != "chosen words" {
		t.Fatalf("selection=%v", got)
	}
	query.Selection.Type = selection.SelectionTypeFile
	if len(p.querySelection(ctx, query)) != 0 {
		t.Fatal("file selection accepted")
	}
	search := webSearch{Title: "{wox:clipboard_text} {wox:selected_text}", Urls: []string{"https://example.com/?q={wox:parameter?name=q}"}}
	environment := map[string]string{plugin.QueryVariableClipboardText: "clip", plugin.QueryVariableSelectedText: "selected"}
	result := p.searchResult(ctx, search, map[string]string{}, environment, nil)
	environment[plugin.QueryVariableClipboardText] = "changed"
	if result.Title != "clip selected" || len(result.Actions) != 1 {
		t.Fatal("environment not bound to result")
	}
	if got := p.searchResult(ctx, search, map[string]string{}, nil, nil); len(got.Actions) != 0 {
		t.Fatal("missing context can open URL")
	}
	encoded, _ := json.Marshal(plugin.RegisterTriggerKeywordOption{Keyword: "g", QueryVariables: []string{plugin.QueryVariableSelectedText}})
	if strings.Contains(string(encoded), "QueryVariables") {
		t.Fatal("core context requirement leaked into public API")
	}
}
