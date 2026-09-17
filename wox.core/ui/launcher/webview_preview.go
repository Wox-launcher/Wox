package launcher

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	launcherview "wox/ui/launcher/view"
	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

const (
	openWebViewPreviewActionID        = "__wox_internal_open_webview_preview__"
	webViewPreviewURLContextKey       = "url"
	webViewPreviewInjectCSSContextKey = "injectCss"
	webViewPreviewWidthContextKey     = "width"
	webViewPreviewHeightContextKey    = "height"
	webViewPreviewAutomationID        = "launcher.preview.webview"
)

type webViewPreviewData struct {
	URL           string `json:"url"`
	HTML          string `json:"html"`
	InjectCSS     string `json:"injectCss"`
	UserAgent     string `json:"userAgent"`
	CacheDisabled bool   `json:"cacheDisabled"`
	CacheKey      string `json:"cacheKey"`
}

// decodeWebViewPreview preserves compatibility with plugins that still send a plain URL.
func decodeWebViewPreview(previewData string) (webViewPreviewData, error) {
	trimmed := strings.TrimSpace(previewData)
	if trimmed == "" {
		return webViewPreviewData{}, errors.New("preview data is empty")
	}
	if strings.HasPrefix(trimmed, "{") {
		var data webViewPreviewData
		if err := json.Unmarshal([]byte(trimmed), &data); err != nil {
			return webViewPreviewData{}, err
		}
		if strings.TrimSpace(data.URL) == "" && data.HTML == "" {
			return webViewPreviewData{}, errors.New("preview requires a URL or HTML")
		}
		return data, nil
	}
	return webViewPreviewData{URL: trimmed}, nil
}

func (d webViewPreviewData) content() woxui.WebViewContent {
	cacheKey := strings.TrimSpace(d.CacheKey)
	if !d.CacheDisabled && cacheKey == "" {
		cacheKey = strings.TrimSpace(d.URL)
		if cacheKey == "" {
			cacheKey = strings.TrimSpace(d.HTML)
		}
	}
	return woxui.WebViewContent{URL: d.URL, HTML: d.HTML, InjectCSS: d.InjectCSS, UserAgent: d.UserAgent, CacheDisabled: d.CacheDisabled, CacheKey: cacheKey}
}

func (a *App) buildWebViewPreview(previewData string, palette uiPalette, width, height float32) woxwidget.Widget {
	theme := palette.componentTheme()
	data, err := decodeWebViewPreview(previewData)
	if err != nil {
		return webViewPreviewSemantics("error: "+err.Error(), previewview.WebViewPreviewMessage(fmt.Sprintf("Invalid WebView preview: %v", err), theme.ErrorText, theme, width, height))
	}
	active := a.webViewPreviewData == previewData
	webViewError := ""
	if active {
		webViewError = a.webViewPreviewError
	}
	if webViewError != "" {
		return webViewPreviewSemantics("error: "+webViewError, previewview.WebViewPreviewMessage(webViewError, theme.ErrorText, theme, width, height))
	}
	if !active {
		return webViewPreviewSemantics("loading", previewview.WebViewPreviewLoading(theme, width, height))
	}
	content := data.content()
	content.CornerRadius = previewview.WebViewPreviewCornerRadius
	return webViewPreviewSemantics(webViewPreviewReadyValue(data), previewview.WebViewPreview(previewview.WebViewPreviewProps{Width: width, Height: height, Theme: theme, OnPointer: a.window.ForwardEmbeddedSurfacePointer, OnEscape: a.handleWebViewFallbackEscape, OnBounds: func(bounds woxui.Rect) {
		if a.webViewPreviewData != previewData || a.webViewPreviewError != "" {
			return
		}
		a.syncWebViewActionHotkey()
		if err := a.window.ShowWebView(content, bounds); err != nil {
			a.setWebViewPreviewError(err)
			return
		}
		a.requestWebViewKeyboardFocus()
	}}))
}

// webViewPreviewSemantics publishes preview readiness without exposing the native page DOM.
func webViewPreviewSemantics(value string, child woxwidget.Widget) woxwidget.Widget {
	return woxwidget.Semantics{
		AutomationID: webViewPreviewAutomationID,
		Role:         woxui.AccessibilityRoleText,
		Label:        "WebView preview",
		Value:        value,
		LiveRegion:   woxui.AccessibilityLiveRegionPolite,
		Child:        child,
	}
}

func webViewPreviewReadyValue(data webViewPreviewData) string {
	if url := strings.TrimSpace(data.URL); url != "" {
		return url
	}
	return "ready"
}

// handleWebViewFallbackEscape leaves browser focus before applying the launcher's outer Escape behavior.
func (a *App) handleWebViewFallbackEscape() {
	if a.webViewFullscreen {
		a.exitWebViewPreviewMode()
		return
	}
	a.webViewWantKeyboardFocus = false
	queryVisible := !a.show.HideQueryBox
	queryCanFocus := a.queryCanFocus()
	focusedKeyBefore := woxwidget.Key("")
	if a.host != nil {
		focusedKeyBefore = a.host.FocusedKey()
	}
	webViewFocusedBefore := a.host != nil && a.host.HasFocus(previewview.WebViewPreviewFocusKey)
	queryFocusedBefore := a.host != nil && a.host.HasFocus(launcherview.LauncherQueryInputKey)
	requested := queryVisible && a.host != nil && a.host.RequestFocus(launcherview.LauncherQueryInputKey)
	focusedKeyAfter := woxwidget.Key("")
	if a.host != nil {
		focusedKeyAfter = a.host.FocusedKey()
	}
	queryFocusedAfter := a.host != nil && a.host.HasFocus(launcherview.LauncherQueryInputKey)
	util.GetLogger().Info(a.lifecycleCtx, fmt.Sprintf(
		"webview escape launcher focus transfer: queryVisible=%t queryCanFocus=%t focusedKeyBefore=%q webViewFocusedBefore=%t queryFocusedBefore=%t requestFocus=%t focusedKeyAfter=%q queryFocusedAfter=%t",
		queryVisible,
		queryCanFocus,
		focusedKeyBefore,
		webViewFocusedBefore,
		queryFocusedBefore,
		requested,
		focusedKeyAfter,
		queryFocusedAfter,
	))
	if requested && queryFocusedAfter {
		return
	}
	util.Go(a.lifecycleCtx, "hide launcher from webview escape", func() {
		if err := a.hideWindow(true); err != nil {
			util.GetLogger().Error(a.lifecycleCtx, "hide launcher from webview escape: "+err.Error())
		}
	})
}

func (a *App) setWebViewPreviewError(err error) {
	if a.webViewPreviewError == err.Error() {
		return
	}
	a.webViewPreviewError = err.Error()
	a.hideWebView()
	_ = a.window.Invalidate()
}

// activateWebViewPreview prepares controller state and reports whether the active URL changed.
func (a *App) activateWebViewPreview(previewData string) bool {
	changed := a.webViewPreviewData != previewData
	urlChanged := changed && webViewPreviewURLChanged(a.webViewPreviewData, previewData)
	if changed {
		a.webViewPreviewData = previewData
		a.webViewPreviewError = ""
		a.webViewNavigation = woxui.WebViewNavigationState{}
		if a.isGlobalWebViewPreview() && !a.keepQueryFocusOnWebViewActivate {
			a.beginWebViewKeyboardFocus()
		}
	}
	if strings.TrimSpace(a.webViewNavigation.URL) == "" {
		if data, err := decodeWebViewPreview(previewData); err == nil {
			a.webViewNavigation.URL = strings.TrimSpace(data.URL)
		}
	}
	return urlChanged
}

func webViewPreviewURLChanged(previousData, nextData string) bool {
	if strings.TrimSpace(previousData) == "" {
		return false
	}
	previous, previousErr := decodeWebViewPreview(previousData)
	next, nextErr := decodeWebViewPreview(nextData)
	if previousErr != nil || nextErr != nil {
		return previousData != nextData
	}
	return strings.TrimSpace(previous.URL) != strings.TrimSpace(next.URL)
}

// deactivateWebViewPreview clears controller ownership and reports whether native content was attached.
func (a *App) deactivateWebViewPreview() bool {
	wasActive := a.webViewPreviewData != "" || a.webViewPreviewError != ""
	a.webViewPreviewData = ""
	a.webViewPreviewError = ""
	a.webViewNavigation = woxui.WebViewNavigationState{}
	return wasActive
}

// hideWebView marshals native WebView detachment onto the UI thread.
func (a *App) hideWebView() {
	if a.window == nil {
		return
	}
	_ = woxui.Call(func() {
		_ = a.window.HideWebView()
	})
}

// resetWebView drops the native instance so changed URLs cannot reuse failed or stale state.
func (a *App) resetWebView() {
	if a.window == nil {
		return
	}
	_ = woxui.Call(func() {
		_ = a.window.ResetWebView()
	})
}

func (a *App) isPreviewFullscreen() bool {
	return a.chatFullscreen || a.webViewFullscreen || a.terminalFullscreen
}

// onWebViewPreviewModeKey keeps result-list navigation from tearing down the
// preview. Typing in the query box still reaches the editor; Escape leaves.
func (a *App) onWebViewPreviewModeKey(event woxui.KeyEvent) bool {
	if !a.webViewFullscreen {
		return false
	}
	if event.Down && !event.Repeat && event.Modifiers == 0 && event.Key == woxui.KeyEscape {
		// Native WebView2/WKWebView deliver reserved Escape here after releasing page focus.
		a.exitWebViewPreviewMode()
		return true
	}
	if webViewPreviewBlocksResultNavigation(event) {
		return true
	}
	if a.host != nil && a.host.HasFocus(launcherview.LauncherQueryInputKey) {
		return false
	}
	// Page focus swallows ordinary launcher keys, but reserved WebView
	// accelerators still need to run refresh/back/forward/open-in-browser.
	if a.onResultActionHotkey(event) {
		return true
	}
	return true
}

func webViewPreviewBlocksResultNavigation(event woxui.KeyEvent) bool {
	switch event.Key {
	case woxui.KeyArrowUp, woxui.KeyArrowDown, woxui.KeyPageUp, woxui.KeyPageDown:
		return true
	default:
		return false
	}
}

// enterWebViewPreviewMode replaces the result area with a WebView and leaves the query box visible.
func (a *App) enterWebViewPreviewMode(resultIndex int, contextData map[string]string) {
	previewURL := strings.TrimSpace(contextData[webViewPreviewURLContextKey])
	if resultIndex < 0 || resultIndex >= len(a.results) || !isWebViewPreviewURL(previewURL) {
		return
	}
	// Escape destroys this surface; caching by URL would restore the old scroll position.
	payload, err := json.Marshal(webViewPreviewData{
		URL: previewURL, InjectCSS: strings.TrimSpace(contextData[webViewPreviewInjectCSSContextKey]), CacheDisabled: true,
	})
	if err != nil {
		util.GetLogger().Error(a.lifecycleCtx, fmt.Sprintf("marshal webview preview: %v", err))
		return
	}
	result := &a.results[resultIndex]
	if a.webViewFullscreenResultID != result.ID {
		a.webViewFullscreenRestore = result.Preview
		a.webViewFullscreenResultID = result.ID
	}
	result.Preview.PreviewType = "webview"
	result.Preview.PreviewData = string(payload)
	a.selected = resultIndex
	a.webViewFullscreen = true
	a.webViewPreviewWidth = parseOptionalPositiveInt(contextData[webViewPreviewWidthContextKey])
	a.webViewPreviewHeight = parseOptionalPositiveInt(contextData[webViewPreviewHeightContextKey])
	a.restoreQueryTextInput()
	a.reconcileSelectedPreview()
	_ = a.applyWindowBounds()
	a.beginWebViewKeyboardFocus()
	if a.window != nil {
		_ = a.window.Invalidate()
	}
}

// isGlobalWebViewPreview reports a result-owned WebView page, such as `webview g`
// or Web Search's in-launcher preview. File previews that happen to render as a
// WebView stay on the query box.
func (a *App) isGlobalWebViewPreview() bool {
	if a.selected < 0 || a.selected >= len(a.results) {
		return false
	}
	return a.results[a.selected].Preview.PreviewType == "webview"
}

// beginWebViewKeyboardFocus asks the Host and the native page for keyboard input.
func (a *App) beginWebViewKeyboardFocus() {
	a.webViewWantKeyboardFocus = true
	a.requestWebViewKeyboardFocus()
}

func parseOptionalPositiveInt(value string) int {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// requestWebViewKeyboardFocus moves keyboard input onto the native page. Host
// RequestFocus takes the query-box caret; composition WebView2 and WKWebView
// still need a separate native handoff, and the first Show may still be creating
// the controller. OnBounds retries until both succeed so plugin pages such as
// `webview g` match Web Search's Ctrl+Enter focus.
func (a *App) requestWebViewKeyboardFocus() {
	if !a.webViewWantKeyboardFocus {
		return
	}
	hostFocused := a.host == nil
	if a.host != nil {
		hostFocused = a.host.HasFocus(previewview.WebViewPreviewFocusKey)
		if !hostFocused {
			hostFocused = a.host.RequestFocus(previewview.WebViewPreviewFocusKey)
		}
		if hostFocused && a.window != nil {
			a.restoreQueryTextInput()
		}
	}
	if a.window == nil {
		return
	}
	a.syncWebViewActionHotkey()
	if err := a.window.FocusWebView(); err != nil {
		return
	}
	if hostFocused {
		a.webViewWantKeyboardFocus = false
	}
}

// exitWebViewPreviewMode restores the query box without destroying the current search.
func (a *App) exitWebViewPreviewMode() {
	if !a.webViewFullscreen {
		return
	}
	a.clearWebViewPreviewModeLocked()
	a.reconcileSelectedPreview()
	// Fullscreen search preview is ephemeral: hide would keep a cached document at its
	// last scroll position, so the next Ctrl+Enter must not reuse that session.
	a.resetWebView()
	a.restoreQueryTextInput()
	a.selectEntireQuery()
	_ = a.applyWindowBounds()
	if a.host != nil {
		a.host.RequestFocus(launcherview.LauncherQueryInputKey)
	}
	if a.window != nil {
		_ = a.window.Invalidate()
	}
}

// clearWebViewPreviewModeLocked drops fullscreen chrome and restores the result that opened the preview.
func (a *App) clearWebViewPreviewModeLocked() {
	a.restoreWebViewPreviewResultLocked()
	a.webViewFullscreen = false
	a.webViewFullscreenResultID = ""
	a.webViewFullscreenRestore = queryPreview{}
	a.webViewWantKeyboardFocus = false
	a.webViewPreviewWidth = 0
	a.webViewPreviewHeight = 0
}

// restoreWebViewPreviewResultLocked puts back the result's original preview so leaving
// fullscreen does not leave a split WebView pane behind.
func (a *App) restoreWebViewPreviewResultLocked() {
	if a.webViewFullscreenResultID == "" {
		return
	}
	for index := range a.results {
		if a.results[index].ID == a.webViewFullscreenResultID {
			a.results[index].Preview = a.webViewFullscreenRestore
			return
		}
	}
}

// openWebViewInSystemBrowser opens the current page in the OS browser and hides Wox.
func (a *App) openWebViewInSystemBrowser() {
	if a.window == nil {
		return
	}
	if err := a.window.WebViewOpenInBrowser(); err != nil {
		util.GetLogger().Error(a.lifecycleCtx, fmt.Sprintf("open webview in browser: %v", err))
		a.hideActionPanel()
		return
	}
	// Opening the system browser leaves this preview; drop the session so the next
	// Ctrl+Enter cannot resume the same scroll position after Hide.
	a.resetWebView()
	if err := a.hideWindow(true); err != nil {
		util.GetLogger().Error(a.lifecycleCtx, fmt.Sprintf("hide launcher after open webview in browser: %v", err))
	}
}

// syncWebViewActionHotkey pushes the configured Action Hotkey into the embedded preview.
func (a *App) syncWebViewActionHotkey() {
	if a == nil || a.window == nil {
		return
	}
	_ = a.window.SetWebViewActionHotkey(a.actionPanelHotkey())
}

func isWebViewPreviewURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Scheme, "http") || strings.EqualFold(parsed.Scheme, "https")
}
