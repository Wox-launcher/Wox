package launcher

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"wox/common"
	"wox/network"

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
	webViewPreviewUserAgentContextKey = "userAgent"
	webViewPreviewKeepContextKey      = "keepInBackground"
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
		var fields map[string]json.RawMessage
		_ = json.Unmarshal([]byte(trimmed), &fields)
		_, hasMode := fields["updateMode"]
		_, hasScript := fields["script"]
		if hasMode || hasScript {
			return webViewPreviewData{}, errors.New("script updates are only valid in UpdateResult")
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
	}
	return woxui.WebViewContent{URL: d.URL, HTML: d.HTML, InjectCSS: d.InjectCSS, UserAgent: d.UserAgent, CacheDisabled: d.CacheDisabled, CacheKey: cacheKey}
}

func (a *webViewPreviewController) buildWebViewPreview(previewData string, palette uiPalette, width, height float32, fullBleed bool) woxwidget.Widget {
	theme := palette.componentTheme()
	if network.IsOffline() {
		return webViewPreviewSemantics("offline", previewview.WebViewPreviewMessage(a.language.translate("i18n:ui_offline_unavailable"), theme.PreviewText, theme, width, height))
	}
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
	content.DocumentKey = a.webViewDocumentKey()
	content.CornerRadius = previewview.WebViewPreviewCornerRadius
	if fullBleed {
		content.CornerRadius = 0
	}
	return webViewPreviewSemantics(webViewPreviewReadyValue(data), previewview.WebViewPreview(previewview.WebViewPreviewProps{Width: width, Height: height, Theme: theme, CornerRadius: content.CornerRadius, OnPointer: a.window().ForwardEmbeddedSurfacePointer, OnEscape: a.handleWebViewFallbackEscape, OnBounds: func(bounds woxui.Rect) {
		if a.isDestroyed() || network.IsOffline() || a.webViewPreviewData != previewData || a.webViewPreviewError != "" || content.DocumentKey != a.webViewDocumentKey() {
			return
		}
		a.syncWebViewActionHotkey()
		if err := a.window().ShowWebView(content, bounds); err != nil {
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
func (a *webViewPreviewController) handleWebViewFallbackEscape() {
	if a.webViewFullscreen {
		a.exitWebViewPreviewMode()
		return
	}
	a.webViewWantKeyboardFocus = false
	queryVisible := !a.show.HideQueryBox
	queryCanFocus := a.queryCanFocus()
	focusedKeyBefore := woxwidget.Key("")
	if a.host() != nil {
		focusedKeyBefore = a.host().FocusedKey()
	}
	webViewFocusedBefore := a.host() != nil && a.host().HasFocus(previewview.WebViewPreviewFocusKey)
	queryFocusedBefore := a.host() != nil && a.host().HasFocus(launcherview.LauncherQueryInputKey)
	requested := queryVisible && a.host() != nil && a.host().RequestFocus(launcherview.LauncherQueryInputKey)
	focusedKeyAfter := woxwidget.Key("")
	if a.host() != nil {
		focusedKeyAfter = a.host().FocusedKey()
	}
	queryFocusedAfter := a.host() != nil && a.host().HasFocus(launcherview.LauncherQueryInputKey)
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

func (a *webViewPreviewController) setWebViewPreviewError(err error) {
	if a.isDestroyed() || a.webViewPreviewError == err.Error() {
		return
	}
	a.webViewPreviewError = err.Error()
	a.hideWebView()
	_ = a.window().Invalidate()
}

// activateWebViewPreview prepares controller state and reports whether the active URL changed.
func (a *webViewPreviewController) activateWebViewPreview(previewData string) bool {
	if a.isDestroyed() {
		return false
	}
	changed := a.webViewPreviewData != previewData
	urlChanged := changed && webViewPreviewURLChanged(a.webViewPreviewData, previewData)
	if changed {
		a.webViewPreviewData = previewData
		a.webViewPreviewError = ""
		a.webViewNavigation = woxui.WebViewNavigationState{}
		// Query-owned full previews keep typing in the visible query box.
		// Fixed WebSearch URLs are page launchers; searches with argument slots keep query focus.
		keepQueryFocus := launcherPreviewRatio(a.resultList.layout, false) == 0 &&
			!a.show.HideQueryBox && !a.webViewFullscreen
		if a.querySession.queryContext.PluginID == common.WebSearchPluginID {
			hasArguments := false
			if hint := a.querySession.query.QueryHint; hint != nil {
				for _, element := range hint.Elements {
					hasArguments = hasArguments || element.Kind == common.QueryElementArgument
				}
			}
			keepQueryFocus = keepQueryFocus && hasArguments
		}
		if a.isGlobalWebViewPreview() && !a.keepQueryFocusOnWebViewActivate && !keepQueryFocus {
			// HTML previews change while typing or selecting results; taking focus
			// here would send subsequent arrow keys to the page instead of the list.
			if data, err := decodeWebViewPreview(previewData); err == nil && data.HTML == "" {
				// URL replacement detaches the old native page after this call.
				// Defer its focus handoff until the new page is attached by OnBounds.
				a.webViewWantKeyboardFocus = true
				if !urlChanged {
					a.requestWebViewKeyboardFocus()
				}
			}
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
	// HTML has a temporary shared slot. Switching to or from it must not reset
	// that slot or destroy unrelated URL sessions before its idle deadline.
	if previous.HTML != "" || next.HTML != "" {
		return false
	}
	return strings.TrimSpace(previous.URL) != strings.TrimSpace(next.URL)
}

// deactivateWebViewPreview clears controller ownership and reports whether native content was attached.
func (a *webViewPreviewController) deactivateWebViewPreview() bool {
	wasActive := a.webViewPreviewData != "" || a.webViewPreviewError != ""
	a.webViewPreviewData = ""
	a.webViewPreviewError = ""
	a.webViewNavigation = woxui.WebViewNavigationState{}
	return wasActive
}

// hideWebView marshals native WebView detachment onto the UI thread.
func (a *webViewPreviewController) hideWebView() {
	if a.window() == nil {
		return
	}
	_ = woxui.Call(func() {
		_ = a.window().HideWebView()
	})
}

// resetWebView drops the native instance so changed URLs cannot reuse failed or stale state.
func (a *webViewPreviewController) resetWebView() {
	if a.window() == nil {
		return
	}
	_ = woxui.Call(func() {
		_ = a.window().ResetWebView()
	})
}

func (a *App) isPreviewFullscreen() bool {
	return a.chat.chatFullscreen || a.webView.webViewFullscreen || a.terminal.terminalFullscreen
}

// onWebViewPreviewModeKey keeps result-list navigation from tearing down the
// preview. Typing in the query box still reaches the editor; Escape leaves.
func (a *webViewPreviewController) onWebViewPreviewModeKey(event woxui.KeyEvent) bool {
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
	if a.host() != nil && a.host().HasFocus(launcherview.LauncherQueryInputKey) {
		return false
	}
	// Page focus swallows ordinary launcher keys, but reserved WebView
	// accelerators still need to run refresh/back/forward/open-in-browser.
	if a.actions.onResultActionHotkey(event) {
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
func (a *webViewPreviewController) enterWebViewPreviewMode(resultIndex int, contextData map[string]string) {
	previewURL := strings.TrimSpace(contextData[webViewPreviewURLContextKey])
	if resultIndex < 0 || resultIndex >= len(a.resultList.results) || !isWebViewPreviewURL(previewURL) {
		return
	}
	_, currentPreview, visible := a.previews.selectedPreviewForLifecycle()
	if visible && a.resultList.selected == resultIndex && currentPreview.PreviewType == "webview" && currentPreview.PreviewData == a.webViewPreviewData {
		if current, err := decodeWebViewPreview(currentPreview.PreviewData); err == nil && current.URL == previewURL {
			// Automatic previews already own the page; entering manual mode would replace
			// that ownership and could discard navigation or scroll state on Escape.
			a.beginWebViewKeyboardFocus()
			if a.window() != nil {
				_ = a.window().Invalidate()
			}
			return
		}
	}
	payload, err := json.Marshal(webViewPreviewData{
		URL: previewURL, InjectCSS: strings.TrimSpace(contextData[webViewPreviewInjectCSSContextKey]),
		UserAgent:     strings.TrimSpace(contextData[webViewPreviewUserAgentContextKey]),
		CacheDisabled: contextData[webViewPreviewKeepContextKey] != "true",
	})
	if err != nil {
		util.GetLogger().Error(a.lifecycleCtx, fmt.Sprintf("marshal webview preview: %v", err))
		return
	}
	result := &a.resultList.results[resultIndex]
	a.beginMode(*result, parseOptionalPositiveInt(contextData[webViewPreviewWidthContextKey]), parseOptionalPositiveInt(contextData[webViewPreviewHeightContextKey]))
	result.Preview.PreviewType = "webview"
	result.Preview.PreviewData = string(payload)
	a.resultList.selected = resultIndex
	a.restoreQueryTextInput()
	a.previews.reconcileSelectedPreview()
	_ = a.applyWindowBounds()
	a.beginWebViewKeyboardFocus()
	if a.window() != nil {
		_ = a.window().Invalidate()
	}
}

// isGlobalWebViewPreview reports a result-owned WebView page, such as a WebSearch site.
// File previews that happen to render as a
// WebView stay on the query box.
func (a *webViewPreviewController) isGlobalWebViewPreview() bool {
	if a.resultList.selected < 0 || a.resultList.selected >= len(a.resultList.results) {
		return false
	}
	return a.resultList.results[a.resultList.selected].Preview.PreviewType == "webview"
}

// beginWebViewKeyboardFocus asks the Host and the native page for keyboard input.
func (a *webViewPreviewController) beginWebViewKeyboardFocus() {
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
// the controller. OnBounds retries until both succeed.
func (a *webViewPreviewController) requestWebViewKeyboardFocus() {
	if !a.webViewWantKeyboardFocus {
		return
	}
	hostFocused := a.host() == nil
	if a.host() != nil {
		hostFocused = a.host().HasFocus(previewview.WebViewPreviewFocusKey)
		if !hostFocused {
			hostFocused = a.host().RequestFocus(previewview.WebViewPreviewFocusKey)
		}
		if hostFocused && a.window() != nil {
			a.restoreQueryTextInput()
		}
	}
	if a.window() == nil {
		return
	}
	a.syncWebViewActionHotkey()
	if err := a.window().FocusWebView(); err != nil {
		return
	}
	if hostFocused {
		a.webViewWantKeyboardFocus = false
	}
}

// exitWebViewPreviewMode restores the query box without destroying the current search.
func (a *webViewPreviewController) exitWebViewPreviewMode() {
	if !a.webViewFullscreen {
		return
	}
	if a.isWebViewFullPreviewWindow() {
		// A hotkey panel has no query box to return to after closing its page.
		if err := a.hideWindow(true); err != nil {
			util.GetLogger().Error(a.lifecycleCtx, fmt.Sprintf("close web search preview panel: %v", err))
		}
		return
	}
	keepInBackground := a.hasCacheableWebViewPreviewLocked()
	a.clearWebViewPreviewModeLocked()
	a.previews.reconcileSelectedPreview()
	if !keepInBackground {
		a.resetWebView()
	}
	a.restoreQueryTextInput()
	a.querySession.selectEntireQuery()
	_ = a.applyWindowBounds()
	if a.host() != nil {
		a.host().RequestFocus(launcherview.LauncherQueryInputKey)
	}
	if a.window() != nil {
		_ = a.window().Invalidate()
	}
}

// clearWebViewPreviewModeLocked drops fullscreen chrome and restores the result that opened the preview.
func (a *webViewPreviewController) clearWebViewPreviewModeLocked() {
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
func (a *webViewPreviewController) restoreWebViewPreviewResultLocked() {
	if a.webViewFullscreenResultID == "" {
		return
	}
	for index := range a.resultList.results {
		if a.resultList.results[index].ID == a.webViewFullscreenResultID {
			a.resultList.results[index].Preview = a.webViewFullscreenRestore
			return
		}
	}
}

// isWebViewFullPreviewWindow reports a query-hotkey preview panel with no query box.
func (a *webViewPreviewController) isWebViewFullPreviewWindow() bool {
	return a.show.ShowPreviewTitleBar && a.show.HideQueryBox && a.show.HideToolbar
}

// canHideWebViewPage reports a full-preview window that opted into Run in background.
func (a *webViewPreviewController) canHideWebViewPage() bool {
	return a.isWebViewFullPreviewWindow() && a.hasCacheableWebViewPreviewLocked()
}

// closeWebViewPage closes preview-only windows or hides the owning result's split preview.
func (a *webViewPreviewController) closeWebViewPage() {
	fullPreview := a.isWebViewFullPreviewWindow() || launcherPreviewRatio(a.resultList.layout, false) == 0
	if a.webViewFullscreen && !fullPreview {
		// Explicit close releases a cached page too; Escape may keep it in the background.
		a.resetWebView()
		a.exitWebViewPreviewMode()
		return
	}
	// Restoring a query-owned full preview would immediately reopen the page being closed.
	a.clearWebViewPreviewModeLocked()
	a.resetWebView()
	a.deactivateWebViewPreview()
	if fullPreview {
		if err := a.hideWindow(true); err != nil {
			util.GetLogger().Error(a.lifecycleCtx, fmt.Sprintf("close full-preview webview page: %v", err))
		}
		return
	}
	// Suppress the pane until manual reopening or the next query.
	a.previews.reconcilePreviewVisibility()
	visible := false
	a.previews.previewVisibility.visible = &visible
	a.previews.reconcileSelectedPreview()
	a.restoreQueryFocusAfterShow()
	a.restoreQueryTextInput()
	_ = a.applyWindowBounds()
	if a.window() != nil {
		_ = a.window().Invalidate()
	}
}

// hideWebViewPage hides a cacheable full-preview window without destroying the page.
func (a *webViewPreviewController) hideWebViewPage() {
	if err := a.hideWindow(true); err != nil {
		util.GetLogger().Error(a.lifecycleCtx, fmt.Sprintf("hide webview page: %v", err))
	}
}

// openWebViewInSystemBrowser opens the current page in the OS browser and hides Wox.
func (a *webViewPreviewController) openWebViewInSystemBrowser() {
	if a.window() == nil {
		return
	}
	if err := a.window().WebViewOpenInBrowser(); err != nil {
		util.GetLogger().Error(a.lifecycleCtx, fmt.Sprintf("open webview in browser: %v", err))
		a.actions.hideActionPanel()
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
func (a *webViewPreviewController) syncWebViewActionHotkey() {
	if a == nil || a.window() == nil {
		return
	}
	_ = a.window().SetWebViewActionHotkey(a.actionPanelHotkey())
}

func isWebViewPreviewURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Scheme, "http") || strings.EqualFold(parsed.Scheme, "https")
}

// webViewDocumentKey scopes native inline documents to one query result.
func (a *webViewPreviewController) webViewDocumentKey() string {
	if a.resultList.selected < 0 || a.resultList.selected >= len(a.resultList.results) {
		return ""
	}
	key, _ := json.Marshal([]string{a.sessionID, a.resultList.resultsQueryID, a.resultList.results[a.resultList.selected].ID})
	return string(key)
}
