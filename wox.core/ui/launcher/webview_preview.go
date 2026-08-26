package launcher

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	launcherview "wox/ui/launcher/view"
	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
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
		return previewview.WebViewPreviewMessage(fmt.Sprintf("Invalid WebView preview: %v", err), theme.ErrorText, theme, width, height)
	}
	active := a.webViewPreviewData == previewData
	webViewError := ""
	if active {
		webViewError = a.webViewPreviewError
	}
	if webViewError != "" {
		return previewview.WebViewPreviewMessage(webViewError, theme.ErrorText, theme, width, height)
	}
	if !active {
		return previewview.WebViewPreviewLoading(theme, width, height)
	}
	content := data.content()
	content.CornerRadius = previewview.WebViewPreviewCornerRadius
	return previewview.WebViewPreview(previewview.WebViewPreviewProps{Width: width, Height: height, Theme: theme, OnPointer: a.window.ForwardEmbeddedSurfacePointer, OnEscape: a.handleWebViewFallbackEscape, OnBounds: func(bounds woxui.Rect) {
		if a.webViewPreviewData != previewData || a.webViewPreviewError != "" {
			return
		}
		if err := a.window.ShowWebView(content, bounds); err != nil {
			a.setWebViewPreviewError(err)
		}
	}})
}

// handleWebViewFallbackEscape leaves browser focus before applying the launcher's outer Escape behavior.
func (a *App) handleWebViewFallbackEscape() {
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
