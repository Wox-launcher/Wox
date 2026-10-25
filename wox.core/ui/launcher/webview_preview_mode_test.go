package launcher

import (
	"testing"
	"wox/plugin"

	launcherview "wox/ui/launcher/view"
	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestActivateOpenWebViewPreviewActionEntersFullscreen(t *testing.T) {
	app := New(false, nil)
	app.uiCall = nil
	app.visible = true
	app.querySession.query = newInputQuery("g wox")
	defer app.cancel()

	app.resultList.results = []queryResult{{
		ID: "search",
		Actions: []resultAction{{
			ID:                     openWebViewPreviewActionID,
			PreventHideAfterAction: true,
			ContextData:            map[string]string{webViewPreviewURLContextKey: "https://www.google.com/search?q=wox"},
		}},
	}}
	app.resultList.selected = 0
	app.activateAction(0, 0)
	if !app.webView.webViewFullscreen || !app.webView.webViewWantKeyboardFocus || !app.queryCanFocus() || app.resultList.results[0].Preview.PreviewType != "webview" {
		t.Fatalf("webview mode = fullscreen:%v pageFocusPending:%v queryFocus:%v preview:%+v", app.webView.webViewFullscreen, app.webView.webViewWantKeyboardFocus, app.queryCanFocus(), app.resultList.results[0].Preview)
	}
	data, err := decodeWebViewPreview(app.resultList.results[0].Preview.PreviewData)
	if err != nil || data.URL != "https://www.google.com/search?q=wox" || !data.CacheDisabled {
		t.Fatalf("preview data = %+v err=%v", data, err)
	}
	if content := data.content(); !content.CacheDisabled || content.CacheKey != "" || content.InjectCSS != "" {
		t.Fatalf("fullscreen preview must not reuse a cached session: %+v", content)
	}
	if got := launcherPreviewRatio(app.resultList.layout, app.isPreviewFullscreen()); got != 0 {
		t.Fatalf("fullscreen preview ratio = %v, want 0", got)
	}

	app.webView.handleWebViewFallbackEscape()
	if app.webView.webViewFullscreen || app.webView.webViewWantKeyboardFocus || app.resultList.results[0].Preview.PreviewType != "" || !app.queryCanFocus() {
		t.Fatalf("escape restored fullscreen:%v pageFocusPending:%v preview:%+v queryFocus:%v", app.webView.webViewFullscreen, app.webView.webViewWantKeyboardFocus, app.resultList.results[0].Preview, app.queryCanFocus())
	}
}

func TestActivateOpenWebViewPreviewActionAppliesPerSearchChrome(t *testing.T) {
	app := New(false, nil)
	app.uiCall = nil
	app.visible = true
	app.show.WindowWidth = 800
	app.show.MaxResultCount = 8
	defer app.cancel()

	app.resultList.results = []queryResult{{
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
	data, err := decodeWebViewPreview(app.resultList.results[0].Preview.PreviewData)
	if err != nil || data.InjectCSS != "header{display:none}" || !data.CacheDisabled {
		t.Fatalf("preview data = %+v err=%v", data, err)
	}
	if app.webView.webViewPreviewWidth != 640 || app.webView.webViewPreviewHeight != 720 {
		t.Fatalf("preview size = %dx%d", app.webView.webViewPreviewWidth, app.webView.webViewPreviewHeight)
	}
	app.webView.handleWebViewFallbackEscape()
	if app.webView.webViewPreviewWidth != 0 || app.webView.webViewPreviewHeight != 0 {
		t.Fatalf("escape should restore the previous window size, got %dx%d", app.webView.webViewPreviewWidth, app.webView.webViewPreviewHeight)
	}
}

func TestOpenWebViewPreviewActionIgnoresInvalidURL(t *testing.T) {
	app := New(false, nil)
	app.uiCall = nil
	app.visible = true
	defer app.cancel()

	app.resultList.results = []queryResult{{
		ID: "search",
		Actions: []resultAction{{
			ID:          openWebViewPreviewActionID,
			ContextData: map[string]string{webViewPreviewURLContextKey: "not-a-url"},
		}},
	}}
	app.activateAction(0, 0)
	if app.webView.webViewFullscreen {
		t.Fatal("invalid URL entered webview fullscreen")
	}
}

func TestApplyResultsExitsWebViewPreviewMode(t *testing.T) {
	app := New(false, nil)
	app.uiCall = nil
	app.visible = true
	app.querySession.query = newInputQuery("g wox")
	defer app.cancel()

	app.resultList.results = []queryResult{{ID: "search", Actions: []resultAction{{
		ID:          openWebViewPreviewActionID,
		ContextData: map[string]string{webViewPreviewURLContextKey: "https://example.com/?q=wox"},
	}}}}
	app.resultList.selected = 0
	app.activateAction(0, 0)
	if !app.webView.webViewFullscreen {
		t.Fatal("expected webview fullscreen before results refresh")
	}

	app.applyResults(app.querySession.query.QueryID, []queryResult{{ID: "next", Title: "Search"}}, &queryLayout{}, nil, nil, 0, true)
	if app.webView.webViewFullscreen {
		t.Fatal("new results should leave webview fullscreen")
	}
}

func TestWebViewFullscreenKeepsQueryBox(t *testing.T) {
	app := New(false, nil)
	defer app.cancel()
	app.webView.webViewFullscreen = true
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

	app.resultList.results = []queryResult{{ID: "search"}, {ID: "chat"}}
	app.resultList.selected = 0
	app.webView.webViewFullscreen = true
	app.webView.webViewFullscreenResultID = "search"
	if !app.onKey(woxui.KeyEvent{Key: woxui.KeyArrowDown, Down: true}) || !app.webView.webViewFullscreen || app.resultList.selected != 0 {
		t.Fatalf("arrow keys dismissed webview: fullscreen=%v selected=%d", app.webView.webViewFullscreen, app.resultList.selected)
	}
}

func TestOnWebViewPreviewModeKeyLeavesOnEscape(t *testing.T) {
	app := New(false, nil)
	app.uiCall = nil
	app.visible = true
	defer app.cancel()

	app.resultList.results = []queryResult{{ID: "search", Preview: queryPreview{PreviewType: "text"}}}
	app.webView.webViewFullscreen = true
	app.webView.webViewFullscreenResultID = "search"
	app.webView.webViewFullscreenRestore = app.resultList.results[0].Preview
	if !app.webView.onWebViewPreviewModeKey(woxui.KeyEvent{Key: woxui.KeyArrowDown, Down: true}) || !app.webView.webViewFullscreen {
		t.Fatal("launcher keys should stay consumed while webview fullscreen is active")
	}
	if !app.webView.onWebViewPreviewModeKey(woxui.KeyEvent{Key: woxui.KeyEscape, Down: true}) || app.webView.webViewFullscreen {
		t.Fatal("escape should consume the key and exit webview fullscreen")
	}
}

func TestGlobalWebViewPreviewRequestsKeyboardFocus(t *testing.T) {
	app := New(false, nil)
	defer app.cancel()
	preview := `{"url":"https://gemini.google.com"}`
	app.resultList.results = []queryResult{{Preview: queryPreview{PreviewType: "webview", PreviewData: preview}}}
	app.resultList.selected = 0
	if app.webView.activateWebViewPreview(preview) {
		t.Fatal("first activation should not reset a missing session")
	}
	if !app.webView.webViewWantKeyboardFocus {
		t.Fatal("a result-owned webview preview should request page focus")
	}

	app.webView.webViewWantKeyboardFocus = false
	if app.webView.activateWebViewPreview(preview) {
		t.Fatal("same preview should not reset")
	}
	if app.webView.webViewWantKeyboardFocus {
		t.Fatal("same session should not steal focus back")
	}

	if !app.webView.activateWebViewPreview(`{"url":"https://x.com"}`) {
		t.Fatal("a new URL should replace the previous session")
	}
	if !app.webView.webViewWantKeyboardFocus {
		t.Fatal("switching result-owned webview pages should request page focus again")
	}
}

func TestHotkeyShowKeepsQueryFocusOnGlobalWebViewPreview(t *testing.T) {
	app := New(false, nil)
	defer app.cancel()
	preview := `{"url":"https://gemini.google.com"}`
	app.resultList.results = []queryResult{{Preview: queryPreview{PreviewType: "webview", PreviewData: preview}}}
	app.resultList.selected = 0
	app.webView.keepQueryFocusOnWebViewActivate = true
	if app.webView.activateWebViewPreview(preview); app.webView.webViewWantKeyboardFocus {
		t.Fatal("hotkey show should keep the selected query box focused")
	}

	app.webView.keepQueryFocusOnWebViewActivate = false
	app.webView.webViewPreviewData = ""
	if app.webView.activateWebViewPreview(preview); !app.webView.webViewWantKeyboardFocus {
		t.Fatal("a live webview query should still request page focus")
	}
}

func TestRestoreQueryFocusAfterShowCancelsWebViewFocus(t *testing.T) {
	app := New(false, nil)
	defer app.cancel()
	app.webView.webViewWantKeyboardFocus = true
	if app.restoreQueryFocusAfterShow() {
		t.Fatal("query restore should no-op without a host")
	}
	if app.webView.webViewWantKeyboardFocus {
		t.Fatal("hotkey show must cancel pending webview page focus")
	}
}

func TestFileWebViewPreviewKeepsQueryFocus(t *testing.T) {
	app := New(false, nil)
	defer app.cancel()
	app.resultList.results = []queryResult{{Preview: queryPreview{PreviewType: "file", PreviewData: "notes.md"}}}
	app.resultList.selected = 0
	if app.webView.activateWebViewPreview(`{"url":"https://example.com"}`); app.webView.webViewWantKeyboardFocus {
		t.Fatal("a file preview that happens to use a webview should leave the query box focused")
	}
}

func TestWebViewPreviewActivationHostFocus(t *testing.T) {
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.EditableText{Key: launcherview.LauncherQueryInputKey, Autofocus: true, Child: woxwidget.Container{Width: 100, Height: 30}},
			woxwidget.Focusable{Key: previewview.WebViewPreviewFocusKey, Child: woxwidget.Container{Width: 100, Height: 30}},
		}}
	})
	host.AttachServices(formTableHostServices{})
	defer host.Dispose()
	host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 200, Height: 30}, PixelSize: woxui.PixelSize{Width: 200, Height: 30}, Scale: 1})
	if !host.HasFocus(launcherview.LauncherQueryInputKey) {
		t.Fatal("query input should start focused")
	}

	app := New(false, nil)
	defer app.cancel()
	app.host = host
	// Both replacing HTML in one result and selecting another HTML result must
	// keep keyboard navigation on the query input.
	app.resultList.results = []queryResult{
		{ID: "first", Preview: queryPreview{PreviewType: "webview", PreviewData: `{"html":"<p>first</p>"}`}},
		{ID: "second", Preview: queryPreview{PreviewType: "webview", PreviewData: `{"html":"<p>second</p>"}`}},
	}
	for _, selected := range []int{0, 1, 0} {
		app.resultList.selected = selected
		app.webView.activateWebViewPreview(app.resultList.results[selected].Preview.PreviewData)
		if !host.HasFocus(launcherview.LauncherQueryInputKey) || app.webView.webViewWantKeyboardFocus {
			t.Fatalf("HTML result %d stole query focus: key=%q pending=%t", selected, host.FocusedKey(), app.webView.webViewWantKeyboardFocus)
		}
	}
	app.webView.activateWebViewPreview(`{"html":"<p>updated</p>"}`)
	if !host.HasFocus(launcherview.LauncherQueryInputKey) || app.webView.webViewWantKeyboardFocus {
		t.Fatal("updating HTML must keep query focus")
	}
	preview := `{"url":"https://gemini.google.com"}`
	app.resultList.results = []queryResult{{Preview: queryPreview{PreviewType: "webview", PreviewData: preview}}}
	app.resultList.selected = 0
	app.webView.activateWebViewPreview(preview)
	if !host.HasFocus(previewview.WebViewPreviewFocusKey) {
		t.Fatalf("focused key = %q, want the webview preview so the query caret stops blinking", host.FocusedKey())
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
