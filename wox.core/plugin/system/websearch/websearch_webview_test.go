package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"wox/common"
	"wox/plugin"
	"wox/setting/definition"
	"wox/util"
	"wox/util/selection"
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

// TestWebSearchDefaultWebView covers each source of search results with the option enabled and disabled.
func TestWebSearchDefaultWebView(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, source := range []string{"input", "selection", "fallback", "mru"} {
			t.Run(fmt.Sprintf("%t/%s", enabled, source), func(t *testing.T) {
				search := webSearch{Keyword: "g", Title: "Search {wox:parameter?name=query}",
					Urls: []string{"https://example.com/?q={wox:parameter?name=query}"}, IsFallback: true, OpenInWebView: enabled,
					WebViewWidth: 640, WebViewHeight: 720}
				p := &WebSearchPlugin{api: &webSearchTriggerTestAPI{}, webSearches: []webSearch{search}}
				p.registerTriggerKeywords(context.Background())
				var result plugin.QueryResult
				var response plugin.QueryResponse
				switch source {
				case "input":
					response = p.Query(context.Background(), plugin.Query{Type: plugin.QueryTypeInput, TriggerKeyword: "g", RawQuery: "g a & b"})
					result = response.Results[0]
				case "selection":
					response = p.Query(context.Background(), plugin.Query{Type: plugin.QueryTypeSelection, Selection: selection.Selection{Type: selection.SelectionTypeText, Text: "a & b"}})
					result = response.Results[0]
				case "fallback":
					result = p.QueryFallback(context.Background(), plugin.Query{RawQuery: "a & b"})[0]
				case "mru":
					restored, err := p.handleMRURestore(context.Background(), plugin.MRUData{ContextData: webSearchMRUContext(search, map[string]string{plugin.ParameterQueryVariable("query"): "a & b"})})
					if err != nil {
						t.Fatal(err)
					}
					result = *restored
				}
				if !result.Actions[0].IsDefault || (result.Actions[0].Id == webSearchOpenWebViewActionID) != enabled {
					t.Fatalf("default action = %+v, want webview=%v", result.Actions[0], enabled)
				}
				for index, action := range result.Actions {
					if index > 0 && action.IsDefault {
						t.Fatal("multiple default actions")
					}
					expectedHotkey := util.PrimaryHotkey("enter")
					if enabled {
						expectedHotkey = "enter"
					}
					if action.Id == webSearchOpenWebViewActionID && (action.Hotkey != expectedHotkey || action.ContextData[webSearchOpenWebViewURLKey] != "https://example.com/?q=a+%26+b") {
						t.Fatalf("webview action lost its shortcut or encoded URL: %+v", action)
					}
				}
				if enabled && (len(result.Actions) != 2 || result.Actions[1].Name != "i18n:plugin_websearch_open_in_browser") {
					t.Fatal("explicit browser action is missing")
				}
				if enabled {
					if result.Actions[0].Name != "i18n:plugin_websearch_focus_webview" {
						t.Fatal("automatic preview must expose focus as its default action")
					}
					var content plugin.WoxPreviewWebviewData
					if err := json.Unmarshal([]byte(result.Preview.PreviewData), &content); err != nil || content.Url != "https://example.com/?q=a+%26+b" || !content.CacheDisabled {
						t.Fatalf("invalid automatic preview: %+v err=%v", content, err)
					}
				} else if !result.Preview.IsEmpty() {
					t.Fatal("browser searches should retain action-only previews")
				}
				if source == "input" || source == "selection" {
					ratio := response.Layout.ResultPreviewWidthRatio
					if enabled && (ratio == nil || *ratio != 0) {
						t.Fatal("WebView response must include the full-preview layout")
					}
					if !enabled && ratio != nil {
						t.Fatal("browser response must keep the ordinary result layout")
					}
					if enabled && (response.Layout.WindowWidth != 640 || response.Layout.WindowHeight != 720) {
						t.Fatalf("automatic preview lost its window size: %+v", response.Layout)
					}
					if !enabled && (response.Layout.WindowWidth != 0 || response.Layout.WindowHeight != 0) {
						t.Fatal("browser response must not override the window size")
					}
				}
			})
		}
	}
}

// TestWebSearchMultiplePreviewsKeepList requires a choice when one keyword matches several engines.
func TestWebSearchMultiplePreviewsKeepList(t *testing.T) {
	p := &WebSearchPlugin{api: &webSearchTriggerTestAPI{}, webSearches: []webSearch{
		{Keyword: "s", Title: "First", OpenInWebView: true, WebViewWidth: 640, WebViewHeight: 720, IsFallback: true, Urls: []string{"https://example.com/?q={wox:parameter?name=query}"}},
		{Keyword: "s", Title: "Second", OpenInWebView: true, WebViewWidth: 900, WebViewHeight: 800, IsFallback: true, Urls: []string{"https://example.org/?q={wox:parameter?name=query}"}},
	}}
	p.registerTriggerKeywords(context.Background())
	for _, query := range []plugin.Query{
		{Type: plugin.QueryTypeInput, TriggerKeyword: "s", RawQuery: "s wox"},
		{Type: plugin.QueryTypeSelection, Selection: selection.Selection{Type: selection.SelectionTypeText, Text: "wox"}},
	} {
		response := p.Query(context.Background(), query)
		if len(response.Results) != 2 || response.Layout.ResultPreviewWidthRatio != nil || response.Layout.WindowWidth != 0 || response.Layout.WindowHeight != 0 {
			t.Fatalf("ambiguous search must keep its result list and window size: %+v", response)
		}
	}
}

// TestWebSearchPreviewSizeDefaults keeps unset and invalid dimensions on the normal sizing path.
func TestWebSearchPreviewSizeDefaults(t *testing.T) {
	for _, size := range []int{0, -10} {
		search := webSearch{WebViewWidth: size, WebViewHeight: size}
		response := webSearchResponse([]plugin.QueryResult{{Preview: webSearchAutomaticPreview(search, "https://example.com")}}, search)
		if response.Layout.WindowWidth != 0 || response.Layout.WindowHeight != 0 {
			t.Fatalf("size %d must retain default dimensions: %+v", size, response.Layout)
		}
	}
}

// TestWebSearchPreviewHandoff keeps captured values intact when a headless action opens a preview panel.
func TestWebSearchPreviewHandoff(t *testing.T) {
	search := webSearch{Keyword: "g", Title: "Search", OpenInWebView: true, WebViewWidth: 640, WebViewHeight: 720,
		InjectCSS: "header{display:none}", Urls: []string{"https://example.com/?q={wox:selected_text}&clip={wox:clipboard_text}"}}
	values := map[string]string{plugin.QueryVariableSelectedText: "selected & text", plugin.QueryVariableClipboardText: "clipboard"}
	request := webSearchPreviewRequest(search, values)
	if request.Query.QueryId == "" || request.Role != common.WoxInstanceRoleSecondary || !request.ShowApp.HideQueryBox || !request.ShowApp.HideToolbar || !request.ShowApp.ShowPreviewTitleBar || request.ShowApp.WindowWidth != 640 || request.ShowApp.ShowSource != common.ShowSourceQueryHotkey {
		t.Fatalf("preview request = %+v", request)
	}
	values[plugin.QueryVariableSelectedText] = "changed after activation"
	p := &WebSearchPlugin{api: &webSearchTriggerTestAPI{}, webSearches: []webSearch{search}}
	p.registerTriggerKeywords(context.Background())
	query := plugin.Query{Type: plugin.QueryTypeInput, TriggerKeyword: "g", RawQuery: request.Query.QueryText, ContextData: request.Query.ContextData,
		QueryVariables: map[string]string{plugin.QueryVariableSelectedText: "new selection", plugin.QueryVariableClipboardText: "new clipboard"}}
	response := p.Query(context.Background(), query)
	if len(response.Results) != 1 || response.Results[0].Actions[0].Id != webSearchOpenWebViewActionID {
		t.Fatalf("handoff response = %+v", response)
	}
	data := response.Results[0].Actions[0].ContextData
	if ratio := response.Layout.ResultPreviewWidthRatio; ratio == nil || *ratio != 0 {
		t.Fatal("handoff lost full-preview layout")
	}
	if response.Layout.WindowWidth != 640 || response.Layout.WindowHeight != 720 {
		t.Fatalf("handoff lost window size: %+v", response.Layout)
	}
	if data[webSearchOpenWebViewURLKey] != "https://example.com/?q=selected+%26+text&clip=clipboard" || data[webSearchOpenWebViewHeightKey] != "720" || data[webSearchOpenWebViewInjectCSSKey] != search.InjectCSS {
		t.Fatalf("handoff lost inputs or chrome: %+v", data)
	}
	query.ContextData[webSearchMRUValuesKey] = "invalid JSON"
	if len(p.Query(context.Background(), query).Results) != 0 {
		t.Fatal("invalid handoff should not open a page")
	}
}

// TestWebSearchDefaultWebViewSetting keeps the opt-in inside the collapsed WebView group.
func TestWebSearchDefaultWebViewSetting(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{`{"Keyword":"g"}`, false},
		{`{"OpenInWebView":false}`, false},
		{`{"OpenInWebView":true}`, true},
	} {
		var search webSearch
		if err := json.Unmarshal([]byte(test.value), &search); err != nil || search.OpenInWebView != test.want {
			t.Fatalf("row %s = %+v, err=%v", test.value, search, err)
		}
	}
	for _, item := range (&WebSearchPlugin{}).GetMetadata().SettingDefinitions {
		table, ok := item.Value.(*definition.PluginSettingValueTable)
		if !ok {
			continue
		}
		for _, column := range table.Columns {
			if column.Key == "OpenInWebView" {
				if column.Type != definition.PluginSettingValueTableColumnTypeCheckbox || column.Group != webSearchTableGroupWebView || !column.HideInTable {
					t.Fatalf("default WebView column = %+v", column)
				}
				return
			}
		}
	}
	t.Fatal("default WebView setting is missing")
}

type webSearchPreviewTestUI struct {
	common.UI
	request common.OpenWoxInstanceRequest
}

func (u *webSearchPreviewTestUI) OpenWoxInstance(_ context.Context, request common.OpenWoxInstanceRequest) {
	u.request = request
}

// TestWebSearchHeadlessDefaultAction verifies global hotkeys open a window instead of executing a UI-only no-op.
func TestWebSearchHeadlessDefaultAction(t *testing.T) {
	manager := plugin.GetPluginManager()
	previousUI := manager.GetUI()
	ui := &webSearchPreviewTestUI{}
	manager.AttachUI(ui)
	t.Cleanup(func() { manager.AttachUI(previousUI) })
	search := webSearch{Keyword: "g", Title: "Search", OpenInWebView: true,
		Urls: []string{"https://example.com/?q={wox:parameter?name=query}"}}
	p := &WebSearchPlugin{}
	result := p.searchResult(context.Background(), search, map[string]string{plugin.ParameterQueryVariable("query"): "wox"}, nil, nil)
	result.Actions[0].Action(context.Background(), plugin.ActionContext{})
	if ui.request.Query.QueryText != "g " || ui.request.Query.ContextData[webSearchPreviewContextKey] != "true" {
		t.Fatalf("headless action did not open a preview: %+v", ui.request)
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
	response := p.Query(ctx, plugin.Query{Type: plugin.QueryTypeInput, TriggerKeyword: "tr", RawQuery: "tr hello"})
	result := response.Results[0]
	if response.Layout.ResultPreviewWidthRatio != nil {
		t.Fatal("incomplete search must keep the result list")
	}
	if result.Title != "i18n:plugin_websearch_fill_parameters" {
		t.Fatalf("title = %q", result.Title)
	}
	if len(result.Actions) != 1 || result.Actions[0].Id == webSearchOpenWebViewActionID {
		t.Fatalf("incomplete search exposed a webview action: %#v", result.Actions)
	}
}
