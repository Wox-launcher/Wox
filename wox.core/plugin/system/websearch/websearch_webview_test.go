package websearch

import (
	"context"
	"testing"
	"wox/plugin"
	"wox/util"
)

func TestFirstWebViewSearchURLUsesFirstHTTPTemplate(t *testing.T) {
	search := webSearch{Urls: []string{
		"https://www.google.com/search?q={wox:parameter?name=query}",
		"https://www.bing.com/search?q={wox:parameter?name=query}",
	}}
	values := map[string]string{plugin.ParameterQueryVariable("query"): "wox"}
	if got := firstWebViewSearchURL(search, values); got != "https://www.google.com/search?q=wox" {
		t.Fatalf("first URL = %q", got)
	}
	if firstWebViewSearchURL(webSearch{Urls: []string{"not-a-url"}}, values) != "" {
		t.Fatal("invalid URL was accepted")
	}
}

func TestWebSearchOpenInWebViewAction(t *testing.T) {
	ctx := context.Background()
	search := webSearch{
		Keyword: "g", Title: "Search {wox:parameter?name=query}",
		Urls: []string{
			"https://www.google.com/search?q={wox:parameter?name=query}",
			"https://www.bing.com/search?q={wox:parameter?name=query}",
		},
	}
	p := &WebSearchPlugin{api: &webSearchTriggerTestAPI{}, webSearches: []webSearch{search}}
	p.registerTriggerKeywords(ctx)
	result := p.Query(ctx, plugin.Query{Type: plugin.QueryTypeInput, TriggerKeyword: "g", RawQuery: "g wox"}).Results[0]
	if !supportsWebViewPreview() {
		if len(result.Actions) != 1 {
			t.Fatalf("actions = %d, want 1 without WebView preview", len(result.Actions))
		}
		return
	}
	if len(result.Actions) != 2 || !result.Actions[0].IsDefault || result.Actions[1].Id != webSearchOpenWebViewActionID {
		t.Fatalf("actions = %#v", result.Actions)
	}
	action := result.Actions[1]
	if action.Hotkey != util.PrimaryHotkey("enter") || !action.PreventHideAfterAction {
		t.Fatalf("webview action = %#v", action)
	}
	if action.ContextData[webSearchOpenWebViewURLKey] != "https://www.google.com/search?q=wox" {
		t.Fatalf("webview url = %q", action.ContextData[webSearchOpenWebViewURLKey])
	}
}

func TestWebViewPreviewContextDataIncludesPerSearchChrome(t *testing.T) {
	data := webViewPreviewContextData(webSearch{
		InjectCSS:     "body{display:none}",
		WebViewWidth:  640,
		WebViewHeight: 720,
	}, "https://example.com")
	if data[webSearchOpenWebViewURLKey] != "https://example.com" ||
		data[webSearchOpenWebViewInjectCSSKey] != "body{display:none}" ||
		data[webSearchOpenWebViewWidthKey] != "640" ||
		data[webSearchOpenWebViewHeightKey] != "720" {
		t.Fatalf("context = %#v", data)
	}
	if _, ok := webViewPreviewContextData(webSearch{}, "https://example.com")[webSearchOpenWebViewInjectCSSKey]; ok {
		t.Fatal("empty chrome should stay omitted")
	}
}

func TestWebSearchFillParametersOmitsWebViewAction(t *testing.T) {
	ctx := context.Background()
	search := webSearch{
		Keyword: "tr", Title: "{wox:parameter?name=text} → {wox:parameter?name=language}",
		Urls: []string{"https://example.com/?text={wox:parameter?name=text}&lang={wox:parameter?name=language}"},
	}
	p := &WebSearchPlugin{api: &webSearchTriggerTestAPI{}, webSearches: []webSearch{search}}
	p.registerTriggerKeywords(ctx)
	result := p.Query(ctx, plugin.Query{Type: plugin.QueryTypeInput, TriggerKeyword: "tr", RawQuery: "tr hello"}).Results[0]
	if result.Title != "i18n:plugin_websearch_fill_parameters" {
		t.Fatalf("title = %q", result.Title)
	}
	if len(result.Actions) != 1 || result.Actions[0].Id == webSearchOpenWebViewActionID {
		t.Fatalf("incomplete search exposed a webview action: %#v", result.Actions)
	}
}
