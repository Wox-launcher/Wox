package launcher

import (
	"testing"
	"wox/plugin"

	woxui "wox/ui/runtime"
)

func TestActivateOpenWebViewPreviewActionEntersFullscreen(t *testing.T) {
	app := New(false, nil)
	app.uiCall = nil
	app.visible = true
	app.query = newInputQuery("g wox")
	defer app.cancel()

	app.results = []queryResult{{
		ID: "search",
		Actions: []resultAction{{
			ID:                     openWebViewPreviewActionID,
			PreventHideAfterAction: true,
			ContextData:            map[string]string{webViewPreviewURLContextKey: "https://www.google.com/search?q=wox"},
		}},
	}}
	app.selected = 0
	app.activateAction(0, 0)
	if !app.webViewFullscreen || !app.webViewWantKeyboardFocus || !app.queryCanFocus() || app.results[0].Preview.PreviewType != "webview" {
		t.Fatalf("webview mode = fullscreen:%v pageFocusPending:%v queryFocus:%v preview:%+v", app.webViewFullscreen, app.webViewWantKeyboardFocus, app.queryCanFocus(), app.results[0].Preview)
	}
	data, err := decodeWebViewPreview(app.results[0].Preview.PreviewData)
	if err != nil || data.URL != "https://www.google.com/search?q=wox" || !data.CacheDisabled {
		t.Fatalf("preview data = %+v err=%v", data, err)
	}
	if content := data.content(); !content.CacheDisabled || content.CacheKey != "" || content.InjectCSS != "" {
		t.Fatalf("fullscreen preview must not reuse a cached session: %+v", content)
	}
	if got := launcherPreviewRatio(app.layout, app.isPreviewFullscreen()); got != 0 {
		t.Fatalf("fullscreen preview ratio = %v, want 0", got)
	}

	app.handleWebViewFallbackEscape()
	if app.webViewFullscreen || app.webViewWantKeyboardFocus || app.results[0].Preview.PreviewType != "" || !app.queryCanFocus() {
		t.Fatalf("escape restored fullscreen:%v pageFocusPending:%v preview:%+v queryFocus:%v", app.webViewFullscreen, app.webViewWantKeyboardFocus, app.results[0].Preview, app.queryCanFocus())
	}
}

func TestActivateOpenWebViewPreviewActionAppliesPerSearchChrome(t *testing.T) {
	app := New(false, nil)
	app.uiCall = nil
	app.visible = true
	app.show.WindowWidth = 800
	app.show.MaxResultCount = 8
	defer app.cancel()

	app.results = []queryResult{{
		ID: "search",
		Actions: []resultAction{{
			ID: openWebViewPreviewActionID,
			ContextData: map[string]string{
				webViewPreviewURLContextKey:       "https://example.com/search?q=wox",
				webViewPreviewInjectCSSContextKey: "header{display:none}",
				webViewPreviewWidthContextKey:     "640",
				webViewPreviewHeightContextKey:    "720",
			},
		}},
	}}
	app.activateAction(0, 0)
	data, err := decodeWebViewPreview(app.results[0].Preview.PreviewData)
	if err != nil || data.InjectCSS != "header{display:none}" || !data.CacheDisabled {
		t.Fatalf("preview data = %+v err=%v", data, err)
	}
	if app.webViewPreviewWidth != 640 || app.webViewPreviewHeight != 720 {
		t.Fatalf("preview size = %dx%d", app.webViewPreviewWidth, app.webViewPreviewHeight)
	}
	app.handleWebViewFallbackEscape()
	if app.webViewPreviewWidth != 0 || app.webViewPreviewHeight != 0 {
		t.Fatalf("escape should restore the previous window size, got %dx%d", app.webViewPreviewWidth, app.webViewPreviewHeight)
	}
}

func TestOpenWebViewPreviewActionIgnoresInvalidURL(t *testing.T) {
	app := New(false, nil)
	app.uiCall = nil
	app.visible = true
	defer app.cancel()

	app.results = []queryResult{{
		ID: "search",
		Actions: []resultAction{{
			ID:          openWebViewPreviewActionID,
			ContextData: map[string]string{webViewPreviewURLContextKey: "not-a-url"},
		}},
	}}
	app.activateAction(0, 0)
	if app.webViewFullscreen {
		t.Fatal("invalid URL entered webview fullscreen")
	}
}

func TestApplyResultsExitsWebViewPreviewMode(t *testing.T) {
	app := New(false, nil)
	app.uiCall = nil
	app.visible = true
	app.query = newInputQuery("g wox")
	defer app.cancel()

	app.results = []queryResult{{ID: "search", Actions: []resultAction{{
		ID:          openWebViewPreviewActionID,
		ContextData: map[string]string{webViewPreviewURLContextKey: "https://example.com/?q=wox"},
	}}}}
	app.selected = 0
	app.activateAction(0, 0)
	if !app.webViewFullscreen {
		t.Fatal("expected webview fullscreen before results refresh")
	}

	app.applyResults(app.query.QueryID, []queryResult{{ID: "next", Title: "Search"}}, &queryLayout{}, nil, nil, 0, true)
	if app.webViewFullscreen {
		t.Fatal("new results should leave webview fullscreen")
	}
}

func TestWebViewFullscreenKeepsQueryBox(t *testing.T) {
	app := New(false, nil)
	defer app.cancel()
	app.webViewFullscreen = true
	if !app.queryCanFocus() {
		t.Fatal("query box should stay focusable during webview preview")
	}
	ratio := 0.0
	if launcherPreviewOnly(viewSnapshot{
		selected:          0,
		results:           []queryResult{{Preview: queryPreview{PreviewType: "webview", PreviewData: `{"url":"https://example.com"}`}}},
		layout:            queryLayout{ResultPreviewWidthRatio: &ratio},
		webViewFullscreen: true,
	}) {
		t.Fatal("webview preview must not hide the query box as a preview-only panel")
	}
}

func TestWebViewPreviewIgnoresQueryBoxArrowKeys(t *testing.T) {
	app := New(false, nil)
	app.uiCall = nil
	app.visible = true
	defer app.cancel()

	app.results = []queryResult{{ID: "search"}, {ID: "chat"}}
	app.selected = 0
	app.webViewFullscreen = true
	app.webViewFullscreenResultID = "search"
	if !app.onKey(woxui.KeyEvent{Key: woxui.KeyArrowDown, Down: true}) || !app.webViewFullscreen || app.selected != 0 {
		t.Fatalf("arrow keys dismissed webview: fullscreen=%v selected=%d", app.webViewFullscreen, app.selected)
	}
}

func TestOnWebViewPreviewModeKeyLeavesOnEscape(t *testing.T) {
	app := New(false, nil)
	app.uiCall = nil
	app.visible = true
	defer app.cancel()

	app.results = []queryResult{{ID: "search", Preview: queryPreview{PreviewType: "text"}}}
	app.webViewFullscreen = true
	app.webViewFullscreenResultID = "search"
	app.webViewFullscreenRestore = app.results[0].Preview
	if !app.onWebViewPreviewModeKey(woxui.KeyEvent{Key: woxui.KeyArrowDown, Down: true}) || !app.webViewFullscreen {
		t.Fatal("launcher keys should stay consumed while webview fullscreen is active")
	}
	if !app.onWebViewPreviewModeKey(woxui.KeyEvent{Key: woxui.KeyEscape, Down: true}) || app.webViewFullscreen {
		t.Fatal("escape should consume the key and exit webview fullscreen")
	}
}

func TestFromCoreResultActionCopiesContextData(t *testing.T) {
	action := fromCoreResultAction(plugin.QueryResultActionUI{
		Id:          openWebViewPreviewActionID,
		ContextData: map[string]string{webViewPreviewURLContextKey: "https://example.com"},
	})
	if action.ContextData[webViewPreviewURLContextKey] != "https://example.com" {
		t.Fatalf("context = %#v", action.ContextData)
	}
}
