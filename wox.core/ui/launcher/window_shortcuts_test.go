package launcher

import (
	"runtime"
	"testing"
	"wox/common"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TestWindowShortcutPrecedesFocusedControls models a control or modal swallowing all feature keys.
func TestWindowShortcutPrecedesFocusedControls(t *testing.T) {
	for _, modal := range []bool{false, true} {
		controlCalls, closeCalls, fallbackCalls := 0, 0, 0
		host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
			return woxwidget.FocusScope{Key: "scope", Modal: modal, OnKey: func(woxui.KeyEvent) bool {
				controlCalls++
				return true
			}, Child: woxwidget.Focusable{Key: "control", Autofocus: true, OnKey: func(woxui.KeyEvent) bool {
				controlCalls++
				return true
			}, Child: woxwidget.Semantics{AutomationID: "control", Role: woxui.AccessibilityRoleButton, Label: "Focused control", Child: woxwidget.Text{Value: "Focused control"}}}}
		})
		host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 200, Height: 100}, Scale: 1})
		app := bindTestApp(&App{})
		primary := woxui.KeyModifierControl
		if runtime.GOOS == "darwin" {
			primary = woxui.KeyModifierMeta
		}
		event := woxui.KeyEvent{Key: "w", Modifiers: primary, Down: true}
		fallback := func(woxui.KeyEvent) bool { fallbackCalls++; return true }
		closeRequested := func() { closeCalls++ }
		if !app.dispatchWindowKey(event, host, "", closeRequested, fallback) {
			t.Fatal("window close was not handled")
		}
		event.Repeat = true
		if !app.dispatchWindowKey(event, host, "", closeRequested, fallback) || closeCalls != 1 || controlCalls != 0 || fallbackCalls != 0 {
			t.Fatalf("modal=%t close=%d control=%d fallback=%d", modal, closeCalls, controlCalls, fallbackCalls)
		}
	}
}

// TestWindowShortcutLeavesCompositionAndUnrelatedKeysToTheOwner protects recording and text input boundaries.
func TestWindowShortcutLeavesCompositionAndUnrelatedKeysToTheOwner(t *testing.T) {
	primary := woxui.KeyModifierControl
	if runtime.GOOS == "darwin" {
		primary = woxui.KeyModifierMeta
	}
	for _, event := range []woxui.KeyEvent{
		{Key: "w", Modifiers: primary},
		{Key: "w", Modifiers: primary, Down: true, Composing: true},
		{Key: "w", Modifiers: primary | woxui.KeyModifierShift, Down: true},
		{Key: "w", Down: true},
		{Key: "q", Modifiers: primary, Down: true},
	} {
		app := bindTestApp(&App{})
		called := false
		if app.onWindowShortcut(event, "", func() { called = true }) || called {
			t.Fatalf("window command claimed unrelated event: %+v", event)
		}
	}
}

// TestWindowCloseShortcutClosesWebViewBeforeLauncher preserves page-first dismissal.
func TestWindowCloseShortcutClosesWebViewBeforeLauncher(t *testing.T) {
	app := bindTestApp(&App{
		visible: true,
		querySession: queryController{
			editor:       woxui.NewTextEditor("example "),
			query:        newInputQuery("example "),
			queryContext: queryContext{PluginID: common.WebSearchPluginID},
		},
		webView: webViewPreviewController{webViewPreviewData: `{"url":"https://example.com","cacheDisabled":false}`},
	})
	primary := woxui.KeyModifierControl
	if runtime.GOOS == "darwin" {
		primary = woxui.KeyModifierMeta
	}
	fallbackCalled := false
	handled := app.dispatchWindowKey(woxui.KeyEvent{Key: "w", Modifiers: primary, Down: true}, nil, "", app.requestLauncherShortcutClose, func(woxui.KeyEvent) bool {
		fallbackCalled = true
		return true
	})
	if !handled || fallbackCalled || !app.visible || app.webView.webViewPreviewData != "" || app.querySession.query.QueryText != "example " || app.querySession.editor.State().Text != "example " {
		t.Fatal("WebView close shortcut did not dismiss the page before the launcher or custom action")
	}
}

// TestWindowCloseShortcutDefersWebViewTeardown models a native callback that still owns its browser.
func TestWindowCloseShortcutDefersWebViewTeardown(t *testing.T) {
	app := bindTestApp(&App{
		visible: true,
		querySession: queryController{
			editor:       woxui.NewTextEditor("example "),
			query:        newInputQuery("example "),
			queryContext: queryContext{PluginID: common.WebSearchPluginID},
		},
		webView: webViewPreviewController{webViewPreviewData: `{"url":"https://example.com"}`},
	})
	var pending func()
	app.uiPost = func(fn func()) error { pending = fn; return nil }
	app.requestLauncherShortcutClose()
	if pending == nil || app.webView.webViewPreviewData == "" || app.querySession.query.QueryText != "example " {
		t.Fatal("browser close ran before the native key callback returned")
	}
	pending()
	if app.webView.webViewPreviewData != "" || app.querySession.query.QueryText != "example " || !app.visible {
		t.Fatal("queued close did not dismiss the plugin page")
	}
}

// TestWindowCloseShortcutIgnoresDestroyedOwner covers a window destroyed before its queued callback runs.
func TestWindowCloseShortcutIgnoresDestroyedOwner(t *testing.T) {
	app := bindTestApp(&App{webView: webViewPreviewController{webViewPreviewData: `{"url":"https://example.com"}`}})
	var pending func()
	app.uiPost = func(fn func()) error { pending = fn; return nil }
	app.requestLauncherShortcutClose()
	app.destroyed.Store(true)
	pending()
	if app.webView.webViewPreviewData == "" {
		t.Fatal("queued close touched a destroyed window")
	}
}

// TestWindowCloseShortcutIgnoresReplacementPage protects previews opened after the close was queued.
func TestWindowCloseShortcutIgnoresReplacementPage(t *testing.T) {
	app := bindTestApp(&App{
		querySession: queryController{query: newInputQuery("old query")},
		webView:      webViewPreviewController{webViewPreviewData: `{"url":"https://example.com/old"}`},
	})
	var pending func()
	app.uiPost = func(fn func()) error { pending = fn; return nil }
	app.requestLauncherShortcutClose()
	app.querySession.query = newInputQuery("new query")
	app.webView.webViewPreviewData = `{"url":"https://example.com/new"}`
	pending()
	if app.querySession.query.QueryText != "new query" || app.webView.webViewPreviewData == "" {
		t.Fatal("queued close dismissed a replacement page")
	}
}
