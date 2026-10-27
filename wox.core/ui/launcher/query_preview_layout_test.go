package launcher

import (
	"testing"
	"wox/common"
	woxwidget "wox/ui/widget"
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

// TestHideResultsOverridesPluginLayout for every combination of the other display options.
func TestHideResultsOverridesPluginLayout(t *testing.T) {
	app := New(false, nil)
	defer app.cancel()
	app.resultList.selected = 0
	app.resultList.results = []queryResult{{Preview: queryPreview{PreviewType: "file", PreviewData: "main.go", DefaultHidden: true}}}
	for _, hideQuery := range []bool{false, true} {
		for _, hideToolbar := range []bool{false, true} {
			for _, grid := range []bool{false, true} {
				ratio := 0.4
				app.show = showAppParams{HideResults: true, HideQueryBox: hideQuery, HideToolbar: hideToolbar, ShowPreviewTitleBar: hideQuery && hideToolbar}
				app.resultList.layout = queryLayout{ResultPreviewWidthRatio: &ratio}
				if grid {
					app.resultList.layout.GridLayout = &gridLayout{Columns: 4}
				}
				layout := app.previews.selectedPreviewLayout()
				if launcherPreviewRatio(layout, false) != 0 || layout.GridLayout != nil || !launcherPreviewVisible(layout, app.resultList.results[0].Preview) {
					t.Fatalf("query hidden %t toolbar hidden %t grid %t: layout = %#v", hideQuery, hideToolbar, grid, layout)
				}
				if *app.resultList.layout.ResultPreviewWidthRatio != ratio || (app.resultList.layout.GridLayout != nil) != grid {
					t.Fatal("HideResults modified the plugin layout")
				}
				snapshot := viewSnapshot{show: app.show, results: app.resultList.results, selected: 0, layout: layout}
				if launcherPreviewTitleBarVisible(snapshot) != (hideQuery && hideToolbar) {
					t.Fatalf("unexpected title bar visibility for query %t toolbar %t", hideQuery, hideToolbar)
				}
			}
		}
	}
	app.show = showAppParams{HideQueryBox: true, HideToolbar: true, ShowPreviewTitleBar: true}
	ratio := 0.4
	app.resultList.layout = queryLayout{ResultPreviewWidthRatio: &ratio}
	app.resultList.results[0].Preview.DefaultHidden = false
	if launcherPreviewRatio(app.previews.selectedPreviewLayout(), false) != 0.4 {
		t.Fatal("old settings without HideResults must retain the split layout")
	}
}

func TestHideResultsDoesNotShowListWithoutPreview(t *testing.T) {
	app := New(false, nil)
	defer app.cancel()
	result := queryResult{ID: "plain"}
	snapshot := viewSnapshot{show: showAppParams{HideResults: true}, results: []queryResult{result}, selected: 0, layout: queryLayout{}}
	view := app.buildContent(snapshot, 400, 300, 1, 0).(woxwidget.Semantics)
	blank, ok := view.Child.(woxwidget.Container)
	if !ok || blank.Child != nil || blank.Width != 400 || blank.Height != 300 {
		t.Fatalf("hidden results should render an empty content area, got %#v", view.Child)
	}
}
