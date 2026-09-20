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

func TestGlobalWebViewPreviewRequestsKeyboardFocus(t *testing.T) {
	app := New(false, nil)
	defer app.cancel()
	preview := `{"url":"https://gemini.google.com"}`
	app.results = []queryResult{{Preview: queryPreview{PreviewType: "webview", PreviewData: preview}}}
	app.selected = 0
	if app.activateWebViewPreview(preview) {
		t.Fatal("first activation should not reset a missing session")
	}
	if !app.webViewWantKeyboardFocus {
		t.Fatal("a result-owned webview preview should request page focus")
	}

	app.webViewWantKeyboardFocus = false
	if app.activateWebViewPreview(preview) {
		t.Fatal("same preview should not reset")
	}
	if app.webViewWantKeyboardFocus {
		t.Fatal("same session should not steal focus back")
	}

	if !app.activateWebViewPreview(`{"url":"https://x.com"}`) {
		t.Fatal("a new URL should replace the previous session")
	}
	if !app.webViewWantKeyboardFocus {
		t.Fatal("switching result-owned webview pages should request page focus again")
	}
}

func TestHotkeyShowKeepsQueryFocusOnGlobalWebViewPreview(t *testing.T) {
	app := New(false, nil)
	defer app.cancel()
	preview := `{"url":"https://gemini.google.com"}`
	app.results = []queryResult{{Preview: queryPreview{PreviewType: "webview", PreviewData: preview}}}
	app.selected = 0
	app.keepQueryFocusOnWebViewActivate = true
	if app.activateWebViewPreview(preview); app.webViewWantKeyboardFocus {
		t.Fatal("hotkey show should keep the selected query box focused")
	}

	app.keepQueryFocusOnWebViewActivate = false
	app.webViewPreviewData = ""
	if app.activateWebViewPreview(preview); !app.webViewWantKeyboardFocus {
		t.Fatal("a live webview query should still request page focus")
	}
}

func TestRestoreQueryFocusAfterShowCancelsWebViewFocus(t *testing.T) {
	app := New(false, nil)
	defer app.cancel()
	app.webViewWantKeyboardFocus = true
	if app.restoreQueryFocusAfterShow() {
		t.Fatal("query restore should no-op without a host")
	}
	if app.webViewWantKeyboardFocus {
		t.Fatal("hotkey show must cancel pending webview page focus")
	}
}

func TestFileWebViewPreviewKeepsQueryFocus(t *testing.T) {
	app := New(false, nil)
	defer app.cancel()
	app.results = []queryResult{{Preview: queryPreview{PreviewType: "file", PreviewData: "notes.md"}}}
	app.selected = 0
	if app.activateWebViewPreview(`{"url":"https://example.com"}`); app.webViewWantKeyboardFocus {
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
	app.results = []queryResult{
		{ID: "first", Preview: queryPreview{PreviewType: "webview", PreviewData: `{"html":"<p>first</p>"}`}},
		{ID: "second", Preview: queryPreview{PreviewType: "webview", PreviewData: `{"html":"<p>second</p>"}`}},
	}
	for _, selected := range []int{0, 1, 0} {
		app.selected = selected
		app.activateWebViewPreview(app.results[selected].Preview.PreviewData)
		if !host.HasFocus(launcherview.LauncherQueryInputKey) || app.webViewWantKeyboardFocus {
			t.Fatalf("HTML result %d stole query focus: key=%q pending=%t", selected, host.FocusedKey(), app.webViewWantKeyboardFocus)
		}
	}
	app.activateWebViewPreview(`{"html":"<p>updated</p>"}`)
	if !host.HasFocus(launcherview.LauncherQueryInputKey) || app.webViewWantKeyboardFocus {
		t.Fatal("updating HTML must keep query focus")
	}
	preview := `{"url":"https://gemini.google.com"}`
	app.results = []queryResult{{Preview: queryPreview{PreviewType: "webview", PreviewData: preview}}}
	app.selected = 0
	app.activateWebViewPreview(preview)
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
