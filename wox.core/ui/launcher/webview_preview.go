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

func (a *App) buildWebViewPreview(previewData string, palette uiPalette, width, height float32) woxwidget.Widget {
	theme := palette.componentTheme()
	data, err := decodeWebViewPreview(previewData)
	if err != nil {
		return previewview.WebViewPreviewMessage(fmt.Sprintf("Invalid WebView preview: %v", err), theme.ErrorText, theme, width, height)
	}
	a.mu.RLock()
	active := a.webViewPreviewData == previewData
	webViewError := ""
	if active {
		webViewError = a.webViewPreviewError
	}
	a.mu.RUnlock()
	if webViewError != "" {
		return previewview.WebViewPreviewMessage(webViewError, theme.ErrorText, theme, width, height)
	}
	if !active {
		return previewview.WebViewPreviewMessage("Loading WebView preview…", theme.PreviewText, theme, width, height)
	}
	content := data.content()
	return previewview.WebViewPreview(previewview.WebViewPreviewProps{Width: width, Height: height, Theme: theme, OnBounds: func(bounds woxui.Rect) {
		a.mu.RLock()
		current := a.webViewPreviewData == previewData && a.webViewPreviewError == ""
		a.mu.RUnlock()
		if !current {
			return
		}
		if err := a.window.ShowWebView(content, bounds); err != nil {
			a.setWebViewPreviewError(err)
		}
	}})
}

func (a *App) setWebViewPreviewError(err error) {
	a.mu.Lock()
	if a.webViewPreviewError == err.Error() {
		a.mu.Unlock()
		return
	}
	a.webViewPreviewError = err.Error()
	a.mu.Unlock()
	a.hideWebView()
	_ = a.window.Invalidate()
}

// activateWebViewPreview prepares controller state and reports whether native content is stale.
func (a *App) activateWebViewPreview(previewData string) bool {
	a.mu.Lock()
	changed := a.webViewPreviewData != previewData
	if changed {
		a.webViewPreviewData = previewData
		a.webViewPreviewError = ""
	}
	a.mu.Unlock()
	return changed
}

// deactivateWebViewPreview clears controller ownership and reports whether native content was attached.
func (a *App) deactivateWebViewPreview() bool {
	a.mu.Lock()
	wasActive := a.webViewPreviewData != "" || a.webViewPreviewError != ""
	a.webViewPreviewData = ""
	a.webViewPreviewError = ""
	a.mu.Unlock()
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
