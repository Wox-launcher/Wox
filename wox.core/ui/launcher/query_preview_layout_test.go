package launcher

import (
	"testing"
	"wox/common"
)

// TestQueryFullPreviewLayout applies content and layout together, even before query completion.
func TestQueryFullPreviewLayout(t *testing.T) {
	for _, kind := range []string{"webview", "text", "markdown"} {
		t.Run(kind, func(t *testing.T) {
			app := New(false, nil)
			app.uiCall = nil
			app.visible = true
			app.querySession.query = newInputQuery("g wox")
			defer app.cancel()
			data := "Preview content"
			if kind == "webview" {
				data = `{"url":"https://example.com/?q=wox","cacheDisabled":true}`
			}
			result := queryResult{ID: "search", Preview: queryPreview{PreviewType: kind, PreviewData: data}}
			ratio := 0.0
			layout := queryLayout{ResultPreviewWidthRatio: &ratio}
			for _, complete := range []bool{false, true} {
				app.applyResults(app.querySession.query.QueryID, []queryResult{result}, &layout, nil, nil, 0, complete)
				_, preview, visible := app.previews.selectedPreviewForLifecycle()
				if !visible || preview.PreviewData != data || launcherPreviewRatio(app.previews.selectedPreviewLayout(), false) != 0 {
					t.Fatalf("complete=%v: response exposed the result list instead of the preview", complete)
				}
				if app.webView.webViewFullscreen || app.webView.webViewWantKeyboardFocus || !app.queryCanFocus() {
					t.Fatal("query layout must not execute the manual WebView action or take query focus")
				}
			}
			app.applyResults(app.querySession.query.QueryID, []queryResult{{ID: "plain"}}, &queryLayout{}, nil, nil, 0, true)
			if launcherPreviewRatio(app.previews.selectedPreviewLayout(), false) == 0 {
				t.Fatal("ordinary response retained the old full-preview layout")
			}
		})
	}
}

// TestQueryFullPreviewPanelFocus preserves pending page focus across partial/final delivery and Show.
func TestQueryFullPreviewPanelFocus(t *testing.T) {
	app := New(false, nil)
	app.uiCall = nil
	app.visible = true
	app.show = showAppParams{HideQueryBox: true, HideToolbar: true, ShowPreviewTitleBar: true}
	app.querySession.query = newInputQuery("g wox")
	defer app.cancel()
	ratio := 0.0
	layout := queryLayout{ResultPreviewWidthRatio: &ratio}
	result := queryResult{ID: "search", Preview: queryPreview{PreviewType: "webview", PreviewData: `{"url":"https://example.com"}`}}
	for _, complete := range []bool{false, true} {
		app.applyResults(app.querySession.query.QueryID, []queryResult{result}, &layout, nil, nil, 0, complete)
		if !app.webView.webViewWantKeyboardFocus {
			t.Fatalf("complete=%v: preview-only window lost pending page focus", complete)
		}
	}
	if app.restoreQueryFocusAfterShow() || !app.webView.webViewWantKeyboardFocus {
		t.Fatal("Show canceled the pending page focus")
	}
}

// TestQueryFullPreviewSiteFocus distinguishes a fixed site from a search with input slots.
func TestQueryFullPreviewSiteFocus(t *testing.T) {
	app := New(false, nil)
	app.uiCall = nil
	app.visible = true
	app.querySession.query = newInputQuery("example ")
	defer app.cancel()
	ratio := 0.0
	layout := queryLayout{ResultPreviewWidthRatio: &ratio}
	result := queryResult{ID: "page", Preview: queryPreview{PreviewType: "webview", PreviewData: `{"url":"https://example.com"}`}}
	app.applyResults(app.querySession.query.QueryID, []queryResult{result}, &layout, nil, &queryContext{PluginID: common.WebSearchPluginID}, 0, true)
	if !app.webView.webViewWantKeyboardFocus {
		t.Fatal("fixed sites must focus their page")
	}
	app.webView.webViewWantKeyboardFocus = false
	app.querySession.query.QueryHint = &common.QueryHint{Elements: []common.QueryElement{{Kind: common.QueryElementArgument, Value: "wox"}}}
	result.Preview.PreviewData = `{"url":"https://example.com/?q=wox"}`
	app.applyResults(app.querySession.query.QueryID, []queryResult{result}, &layout, nil, nil, 0, true)
	if app.webView.webViewWantKeyboardFocus {
		t.Fatal("parameterized searches must keep query focus")
	}
}

// TestQueryPreviewReplacesManualContent prevents a reused result ID from restoring an obsolete page.
func TestQueryPreviewReplacesManualContent(t *testing.T) {
	app := New(false, nil)
	app.uiCall = nil
	app.visible = true
	app.querySession.query = newInputQuery("g wox")
	defer app.cancel()
	app.resultList.publishRows(app.querySession.query.QueryID, []queryResult{{ID: "search"}})
	app.webView.enterWebViewPreviewMode(0, map[string]string{webViewPreviewURLContextKey: "https://example.com/old"})
	ratio := 0.0
	layout := queryLayout{ResultPreviewWidthRatio: &ratio}
	result := queryResult{ID: "search", Preview: queryPreview{PreviewType: "webview", PreviewData: `{"url":"https://example.com/new"}`}}
	app.applyResults(app.querySession.query.QueryID, []queryResult{result}, &layout, nil, nil, 0, false)
	if app.resultList.results[0].Preview.PreviewData != result.Preview.PreviewData || app.webView.webViewFullscreen {
		t.Fatal("restoring the manual preview overwrote the incoming response")
	}
}
