package websearch

import (
	"context"
	"encoding/json"
	"testing"
	"wox/plugin"
	"wox/setting/definition"
)

// TestWebSearchSitePreviewPreservesBrowserOptions checks automatic, manual and headless entry points.
func TestWebSearchSitePreviewPreservesBrowserOptions(t *testing.T) {
	for _, keep := range []bool{false, true} {
		search := webSearch{Keyword: "site", Title: "Site", Urls: []string{"https://example.com"},
			OpenInWebView: true, KeepInBackground: keep, UserAgent: webviewUserAgentMobileSafari, InjectCSS: "header{display:none}"}
		p := &WebSearchPlugin{api: &webSearchTriggerTestAPI{}, webSearches: []webSearch{search}}
		p.registerTriggerKeywords(context.Background())
		response := p.Query(context.Background(), plugin.Query{Type: plugin.QueryTypeInput, TriggerKeyword: "site", RawQuery: "site "})
		if len(response.Results) != 1 || response.Layout.ResultPreviewWidthRatio == nil || *response.Layout.ResultPreviewWidthRatio != 0 {
			t.Fatalf("fixed site response = %+v", response)
		}
		result := response.Results[0]
		var data plugin.WoxPreviewWebviewData
		if err := json.Unmarshal([]byte(result.Preview.PreviewData), &data); err != nil {
			t.Fatal(err)
		}
		if data.UserAgent != webviewMobileSafariUserAgent || data.CacheDisabled == keep || data.InjectCss != search.InjectCSS {
			t.Fatalf("preview options = %+v", data)
		}
		action := result.Actions[0]
		if action.ContextData[webSearchOpenWebViewUserAgentKey] != data.UserAgent ||
			(action.ContextData[webSearchOpenWebViewKeepKey] == "true") != keep {
			t.Fatal("manual action lost browser options")
		}
		request := webSearchPreviewRequest(search, map[string]string{})
		headless := p.queryWebViewPreview(context.Background(), search, request.Query.ContextData)
		if len(headless.Results) != 1 || headless.Results[0].Preview.PreviewData != result.Preview.PreviewData {
			t.Fatal("headless site lost browser options")
		}
		restored, err := p.handleMRURestore(context.Background(), plugin.MRUData{ContextData: action.ContextData})
		if err != nil || restored.Preview.PreviewData != result.Preview.PreviewData {
			t.Fatalf("fixed-site history failed: %+v %v", restored, err)
		}
	}
}

// TestWebSearchMRURestoreRequiresParameters allows zero-input sites without accepting incomplete searches.
func TestWebSearchMRURestoreRequiresParameters(t *testing.T) {
	search := webSearch{Keyword: "s", Urls: []string{"https://example.com/?q={wox:parameter?name=query}"}}
	p := &WebSearchPlugin{webSearches: []webSearch{search}}
	if _, err := p.handleMRURestore(context.Background(), plugin.MRUData{ContextData: webSearchMRUContext(search, nil)}); err == nil {
		t.Fatal("parameterized search accepted empty history")
	}
	search = webSearch{Keyword: "clip", Urls: []string{"https://example.com/?q={wox:clipboard_text}"}, OpenInWebView: true}
	p.webSearches = []webSearch{search}
	data := webSearchMRUContext(search, map[string]string{plugin.QueryVariableClipboardText: "captured"})
	if result, err := p.handleMRURestore(context.Background(), plugin.MRUData{ContextData: data}); err != nil || result.Preview.IsEmpty() {
		t.Fatalf("captured history context was lost: %+v %v", result, err)
	}
}

// TestResolveWebviewUserAgent covers site browser presets and custom values.
func TestResolveWebviewUserAgent(t *testing.T) {
	for _, test := range []struct{ value, want string }{
		{"", ""}, {"auto", ""}, {webviewUserAgentDesktopEdge, ""}, {webviewUserAgentDesktopSafari, ""},
		{webviewUserAgentDesktopChrome, webviewDesktopChromeUserAgent},
		{webviewUserAgentMobileSafari, webviewMobileSafariUserAgent},
		{webviewUserAgentMobileChrome, webviewMobileChromeUserAgent},
		{" custom-agent ", "custom-agent"},
	} {
		if got := resolveWebviewUserAgent(test.value); got != test.want {
			t.Fatalf("User-Agent %q = %q, want %q", test.value, got, test.want)
		}
	}
}

// TestWebSearchSiteSettings keeps migrated options in the existing collapsed WebView group.
func TestWebSearchSiteSettings(t *testing.T) {
	found := 0
	for _, setting := range (&WebSearchPlugin{}).GetMetadata().SettingDefinitions {
		table, ok := setting.Value.(*definition.PluginSettingValueTable)
		if !ok {
			continue
		}
		for _, column := range table.Columns {
			if column.Key != "UserAgent" && column.Key != "KeepInBackground" {
				continue
			}
			found++
			if !column.HideInTable || column.Group != webSearchTableGroupWebView {
				t.Fatalf("browser setting is not grouped: %+v", column)
			}
			if column.Key == "UserAgent" && (column.Type != definition.PluginSettingValueTableColumnTypeSelect || len(column.SelectOptions) < 3) {
				t.Fatal("User-Agent presets are missing")
			}
			if column.Key == "KeepInBackground" && column.Type != definition.PluginSettingValueTableColumnTypeCheckbox {
				t.Fatal("background setting must be a checkbox")
			}
		}
	}
	if found != 2 {
		t.Fatal("migrated browser settings are missing")
	}
}
