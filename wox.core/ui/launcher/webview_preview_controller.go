package launcher

import (
	"context"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// webViewPreviewController owns the native document, navigation, and focus handoff state,
// renders the WebView preview, and drives its fullscreen mode and page commands (close,
// hide, open in browser). Query, window and result-list changes go through launcher
// collaborators injected by bindWebViewPreview.
type webViewPreviewController struct {
	sessionID                        string
	lifecycleCtx                     context.Context
	language                         *languageCatalog
	resultList                       *resultController
	querySession                     *queryController
	previews                         *previewController
	actions                          *actionPanelController
	show                             *showAppParams
	isDestroyed                      func() bool
	window                           func() *woxui.Window
	host                             func() *woxwidget.Host
	restoreQueryTextInput            func()
	restoreQueryFocusAfterShow       func() bool
	queryCanFocus                    func() bool
	applyWindowBounds                func() error
	hideWindow                       func(notify bool) error
	hasCacheableWebViewPreviewLocked func() bool
	actionPanelHotkey                func() string
	webViewPreviewData               string
	webViewPreviewError              string
	webViewNavigation                woxui.WebViewNavigationState
	webViewFullscreen                bool
	webViewFullscreenResultID        string
	webViewFullscreenRestore         queryPreview
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
	a.webView.sessionID = a.sessionID
	a.webView.lifecycleCtx = a.lifecycleCtx
	a.webView.language = &a.language
	a.webView.resultList = &a.resultList
	a.webView.querySession = &a.querySession
	a.webView.previews = &a.previews
	a.webView.actions = &a.actions
	a.webView.show = &a.show
	a.webView.restoreQueryTextInput = a.restoreQueryTextInput
	a.webView.restoreQueryFocusAfterShow = a.restoreQueryFocusAfterShow
	a.webView.queryCanFocus = a.queryCanFocus
	a.webView.applyWindowBounds = a.applyWindowBounds
	a.webView.hideWindow = a.hideWindow
	a.webView.hasCacheableWebViewPreviewLocked = a.hasCacheableWebViewPreviewLocked
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
