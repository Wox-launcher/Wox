package launcher

import (
	"reflect"
	"testing"

	"wox/common"
	woxui "wox/ui/runtime"
)

// TestCloseWebViewPreviewKeepsOwningQuery covers direct plugin HTML and file previews using the same native surface.
func TestCloseWebViewPreviewKeepsOwningQuery(t *testing.T) {
	const data = `{"html":"<p>preview</p>"}`
	for _, kind := range []string{"webview", "file"} {
		t.Run(kind, func(t *testing.T) {
			preview := queryPreview{PreviewType: kind, PreviewData: data}
			if kind == "file" {
				preview.PreviewData = "example.html"
			}
			app := bindTestApp(&App{
				visible: true,
				querySession: queryController{
					query:        newInputQuery("files example"),
					queryContext: queryContext{PluginID: common.FileSearchPluginID},
					editor:       woxui.NewTextEditor("files example"),
				},
				resultList: resultController{
					selected: 0,
					results:  []queryResult{{ID: "example", Preview: preview}},
				},
				filePreview: filePreviewController{filePreviews: map[string]filePreviewContent{"example.html": {Kind: "webview", WebViewData: data}}},
			})
			app.resultList.resultsQueryID = app.querySession.query.QueryID
			app.previews.reconcileSelectedPreview()
			if app.webView.webViewPreviewData != data {
				t.Fatal("fixture did not activate its native browser preview")
			}
			query := app.querySession.query
			editing := app.querySession.editor.State()
			app.requestLauncherShortcutClose()
			app.previews.reconcileSelectedPreview()
			if app.webView.webViewPreviewData != "" || !app.visible || !reflect.DeepEqual(app.querySession.query, query) || !reflect.DeepEqual(app.querySession.editor.State(), editing) {
				t.Fatal("closing a preview changed its query/editor or reconciliation reopened it")
			}
			if !reflect.DeepEqual(app.resultList.results[0].Preview, preview) || launcherPreviewVisible(app.previews.selectedPreviewLayout(), preview) {
				t.Fatal("close must hide the pane without changing the result payload")
			}
			if !app.toggleSelectedPreview() || app.webView.webViewPreviewData != data {
				t.Fatal("closed preview could not be reopened manually")
			}
		})
	}
}

// TestCloseFullscreenWebSearchRestoresSearch covers Ctrl/Cmd+W on a search URL with an original non-browser preview.
func TestCloseFullscreenWebSearchRestoresSearch(t *testing.T) {
	app := New(false, nil)
	app.uiCall, app.uiPost = nil, nil
	defer app.cancel()
	app.visible = true
	app.querySession.query = newInputQuery("g wox")
	app.querySession.editor = woxui.NewTextEditor("g wox")
	app.querySession.queryContext = queryContext{PluginID: common.WebSearchPluginID}
	app.resultList.resultsQueryID = app.querySession.query.QueryID
	original := queryPreview{PreviewType: "text", PreviewData: "Search details"}
	app.resultList.results = []queryResult{{ID: "search", Title: "Wox", Preview: original}}
	app.resultList.selected = 0
	app.webView.enterWebViewPreviewMode(0, map[string]string{webViewPreviewURLContextKey: "https://example.com/search?q=wox"})
	if !app.webView.webViewFullscreen || app.webView.webViewPreviewData == "" {
		t.Fatal("fixture did not activate a fullscreen search browser")
	}
	query := app.querySession.query
	app.requestLauncherShortcutClose()
	app.previews.reconcileSelectedPreview()
	if app.webView.webViewFullscreen || app.webView.webViewFullscreenResultID != "" || app.webView.webViewPreviewWidth != 0 || app.webView.webViewPreviewHeight != 0 || app.webView.webViewPreviewData != "" {
		t.Fatal("closing fullscreen search left browser state active")
	}
	if !app.visible || !reflect.DeepEqual(app.querySession.query, query) || app.querySession.editor.State().Text != "g wox" || !reflect.DeepEqual(app.resultList.results[0].Preview, original) {
		t.Fatal("closing fullscreen search did not restore its original query and preview")
	}
}
