package launcher

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	woxui "github.com/Wox-launcher/wox.ui.go"
	woxwidget "github.com/Wox-launcher/wox.ui.go/widget"
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
	a.mu.Lock()
	previewChanged := a.webViewPreviewData != previewData
	if previewChanged {
		a.webViewPreviewData = previewData
		a.webViewPreviewError = ""
	}
	a.mu.Unlock()
	if previewChanged {
		_ = a.window.HideWebView()
	}
	data, err := decodeWebViewPreview(previewData)
	if err != nil {
		_ = a.window.HideWebView()
		return a.webViewPreviewMessage(fmt.Sprintf("Invalid WebView preview: %v", err), woxui.Color{R: 232, G: 95, B: 95, A: 255}, palette, width, height)
	}
	a.mu.RLock()
	webViewError := a.webViewPreviewError
	a.mu.RUnlock()
	if webViewError != "" {
		_ = a.window.HideWebView()
		return a.webViewPreviewMessage(webViewError, woxui.Color{R: 232, G: 95, B: 95, A: 255}, palette, width, height)
	}
	content := data.content()
	return woxwidget.Painter{Width: width, Height: height, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		displayList.FillRoundedRect(bounds, 10, palette.queryBackground)
		if err := a.window.ShowWebView(content, bounds); err != nil {
			a.setWebViewPreviewError(err)
		}
	}}
}

func (a *App) webViewPreviewMessage(message string, color woxui.Color, palette uiPalette, width, height float32) woxwidget.Widget {
	return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: palette.queryBackground, Padding: woxwidget.UniformInsets(14), Child: woxwidget.TextBlock{Value: message, Width: max(float32(0), width-28), Height: max(float32(0), height-28), Style: woxui.TextStyle{Size: 13}, Color: color}}
}

func (a *App) setWebViewPreviewError(err error) {
	a.mu.Lock()
	if a.webViewPreviewError == err.Error() {
		a.mu.Unlock()
		return
	}
	a.webViewPreviewError = err.Error()
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) deactivateWebViewPreview() {
	_ = a.window.HideWebView()
	a.mu.Lock()
	a.webViewPreviewData = ""
	a.webViewPreviewError = ""
	a.mu.Unlock()
}
