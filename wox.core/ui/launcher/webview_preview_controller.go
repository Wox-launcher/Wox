package launcher

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// webViewPreviewController owns the native document, navigation, and focus handoff state.
// Query transitions and result replacement remain explicit coordinator operations.
type webViewPreviewController struct {
	isDestroyed                       func() bool
	window                            func() *woxui.Window
	host                              func() *woxwidget.Host
	isGlobalWebViewPreview            func() bool
	restoreQueryTextInput             func()
	restoreWebViewPreviewResultLocked func()
	actionPanelHotkey                 func() string
	webViewPreviewData                string
	webViewPreviewError               string
	webViewNavigation                 woxui.WebViewNavigationState
	webViewFullscreen                 bool
	webViewFullscreenResultID         string
	webViewFullscreenRestore          queryPreview
	// webViewPreviewWidth and webViewPreviewHeight temporarily size the launcher
	// window in logical pixels for an in-launcher WebView preview.
	webViewPreviewWidth  int
	webViewPreviewHeight int
	// webViewWantKeyboardFocus retries native page focus until the embedded surface exists.
	webViewWantKeyboardFocus bool
	// keepQueryFocusOnWebViewActivate skips page focus when a hotkey show restores
	// an existing WebView result and has already selected the query box.
	keepQueryFocusOnWebViewActivate bool
}

// bindWebViewPreview wires host capabilities and cross-domain focus commands.
func (a *App) bindWebViewPreview() {
	a.webView.isDestroyed = a.isDestroyed
	a.webView.window = func() *woxui.Window { return a.window }
	a.webView.host = func() *woxwidget.Host { return a.host }
	a.webView.isGlobalWebViewPreview = a.isGlobalWebViewPreview
	a.webView.restoreQueryTextInput = a.restoreQueryTextInput
	a.webView.restoreWebViewPreviewResultLocked = a.restoreWebViewPreviewResultLocked
	a.webView.actionPanelHotkey = a.actionPanelHotkey
}

// dispose forgets presentation state; the runtime window owns native WebView destruction.
func (c *webViewPreviewController) dispose() {
	c.deactivateWebViewPreview()
	c.webViewFullscreen = false
	c.webViewFullscreenResultID = ""
	c.webViewFullscreenRestore = queryPreview{}
	c.webViewPreviewWidth = 0
	c.webViewPreviewHeight = 0
	c.webViewWantKeyboardFocus = false
	c.keepQueryFocusOnWebViewActivate = false
}

// beginMode retains the original result preview until the temporary page is closed.
func (c *webViewPreviewController) beginMode(result queryResult, width, height int) {
	if c.webViewFullscreenResultID != result.ID {
		c.webViewFullscreenRestore = result.Preview
		c.webViewFullscreenResultID = result.ID
	}
	c.webViewFullscreen = true
	c.webViewPreviewWidth = width
	c.webViewPreviewHeight = height
}
