package launcher

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

type webViewPreviewData struct {
	URL           string `json:"url"`
	HTML          string `json:"html"`
	InjectCSS     string `json:"injectCss"`
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
	return woxui.WebViewContent{URL: d.URL, HTML: d.HTML, InjectCSS: d.InjectCSS, CacheDisabled: d.CacheDisabled, CacheKey: cacheKey}
}

func (a *App) buildWebViewPreview(previewData string, palette uiPalette, width, height, maxRight float32) woxwidget.Widget {
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
		return previewview.WebViewPreviewMessage("Loading WebView preview…", theme.PreviewText, theme, width, height)
	}
	content := data.content()
	return previewview.WebViewPreview(previewview.WebViewPreviewProps{Width: width, Height: height, Theme: theme, OnBounds: func(bounds woxui.Rect) {
		if a.webViewPreviewData != previewData || a.webViewPreviewError != "" {
			return
		}
		bounds, visible := webViewPreviewVisibleBounds(bounds, maxRight)
		if !visible {
			a.hideWebView()
			return
		}
		if err := a.window.ShowWebView(content, bounds); err != nil {
			a.setWebViewPreviewError(err)
		}
	}})
}

func webViewPreviewVisibleBounds(bounds woxui.Rect, maxRight float32) (woxui.Rect, bool) {
	if maxRight > 0 && bounds.X+bounds.Width > maxRight {
		bounds.Width = max(float32(0), maxRight-bounds.X)
	}
	return bounds, bounds.Width > 0 && bounds.Height > 0
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
