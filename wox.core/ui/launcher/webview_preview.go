package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"wox/ui/contract"
	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
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
	content.ToolbarLabels = woxui.WebViewToolbarLabels{
		GoBack:        a.translate("i18n:ui_action_webview_go_back"),
		Refresh:       a.translate("i18n:ui_action_webview_refresh"),
		GoForward:     a.translate("i18n:ui_action_webview_go_forward"),
		OpenInBrowser: a.translate("i18n:ui_action_webview_open_in_browser"),
		HideWox:       a.translate("i18n:ui_action_webview_hide_wox"),
	}
	return previewview.WebViewPreview(previewview.WebViewPreviewProps{Width: width, Height: height, Theme: theme, OnBounds: func(bounds woxui.Rect) {
		current := a.webViewPreviewData == previewData && a.webViewPreviewError == ""
		if !current {
			return
		}
		if err := a.window.ShowWebView(content, bounds); err != nil {
			a.setWebViewPreviewError(err)
		}
	}})
}

func (a *App) setWebViewPreviewError(err error) {
	if a.webViewPreviewError == err.Error() {
		return
	}
	a.webViewPreviewError = err.Error()
	a.hideWebView()
	_ = a.window.Invalidate()
}

// activateWebViewPreview prepares controller state and reports whether native content is stale.
func (a *App) activateWebViewPreview(previewData string) bool {
	changed := a.webViewPreviewData != previewData
	if changed {
		a.webViewPreviewData = previewData
		a.webViewPreviewError = ""
	}
	return changed
}

// deactivateWebViewPreview clears controller ownership and reports whether native content was attached.
func (a *App) deactivateWebViewPreview() bool {
	wasActive := a.webViewPreviewData != "" || a.webViewPreviewError != ""
	a.webViewPreviewData = ""
	a.webViewPreviewError = ""
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

// setWebViewToolbarTooltip keeps native WebView buttons on the shared Wox tooltip path.
func (a *App) setWebViewToolbarTooltip(event woxui.WebViewTooltipEvent) {
	revision := a.webViewTooltipRevision.Add(1)
	util.Go(a.lifecycleCtx, "update webview toolbar tooltip", func() {
		a.tooltipMu.Lock()
		defer a.tooltipMu.Unlock()
		if revision != a.webViewTooltipRevision.Load() {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		name := "go-ui-webview-toolbar-" + a.sessionID
		if !event.Visible {
			if err := a.services.HideTooltip(ctx, a.sessionID, name); err != nil {
				log.Printf("hide webview toolbar tooltip: %v", err)
			}
			return
		}
		if err := a.services.ShowTooltip(ctx, a.sessionID, contract.TooltipOptions{
			Name: name, Text: event.Text, Side: "top",
			AnchorX: float64(event.Bounds.X), AnchorY: float64(event.Bounds.Y),
			AnchorWidth: float64(event.Bounds.Width), AnchorHeight: float64(event.Bounds.Height),
		}); err != nil {
			log.Printf("show webview toolbar tooltip: %v", err)
		}
	})
}
