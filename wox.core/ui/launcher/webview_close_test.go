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
			app := &App{
				visible: true, query: newInputQuery("files example"),
				queryContext: queryContext{PluginID: common.FileSearchPluginID},
				editor:       woxui.NewTextEditor("files example"), selected: 0,
				results:      []queryResult{{ID: "example", Preview: preview}},
				filePreviews: map[string]filePreviewContent{"example.html": {Kind: "webview", WebViewData: data}},
			}
			app.resultsQueryID = app.query.QueryID
			app.reconcileSelectedPreview()
			if app.webViewPreviewData != data {
				t.Fatal("fixture did not activate its native browser preview")
			}
			query := app.query
			editing := app.editor.State()
			app.requestLauncherShortcutClose()
			app.reconcileSelectedPreview()
			if app.webViewPreviewData != "" || !app.visible || !reflect.DeepEqual(app.query, query) || !reflect.DeepEqual(app.editor.State(), editing) {
				t.Fatal("closing a preview changed its query/editor or reconciliation reopened it")
			}
			if !reflect.DeepEqual(app.results[0].Preview, preview) || launcherPreviewVisible(app.selectedPreviewLayout(), preview) {
				t.Fatal("close must hide the pane without changing the result payload")
			}
			if !app.toggleSelectedPreview() || app.webViewPreviewData != data {
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
	app.query = newInputQuery("g wox")
	app.editor = woxui.NewTextEditor("g wox")
	app.queryContext = queryContext{PluginID: common.WebSearchPluginID}
	app.resultsQueryID = app.query.QueryID
	original := queryPreview{PreviewType: "text", PreviewData: "Search details"}
	app.results = []queryResult{{ID: "search", Title: "Wox", Preview: original}}
	app.selected = 0
	app.enterWebViewPreviewMode(0, map[string]string{webViewPreviewURLContextKey: "https://example.com/search?q=wox"})
	if !app.webViewFullscreen || app.webViewPreviewData == "" {
		t.Fatal("fixture did not activate a fullscreen search browser")
	}
	query := app.query
	app.requestLauncherShortcutClose()
	app.reconcileSelectedPreview()
	if app.webViewFullscreen || app.webViewFullscreenResultID != "" || app.webViewPreviewWidth != 0 || app.webViewPreviewHeight != 0 || app.webViewPreviewData != "" {
		t.Fatal("closing fullscreen search left browser state active")
	}
	if !app.visible || !reflect.DeepEqual(app.query, query) || app.editor.State().Text != "g wox" || !reflect.DeepEqual(app.results[0].Preview, original) {
		t.Fatal("closing fullscreen search did not restore its original query and preview")
	}
}
