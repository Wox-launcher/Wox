package woxui

import (
	"errors"
	"strings"
)

// ErrWebViewUnavailable reports that the current desktop is missing its system WebView runtime.
var ErrWebViewUnavailable = errors.New("woxui: system WebView is unavailable")

// WebViewContent describes one embedded browser document while Rect is controlled separately by layout.
type WebViewContent struct {
	URL           string
	HTML          string
	InjectCSS     string
	CacheDisabled bool
	CacheKey      string
}

// ShowWebView attaches or updates the window's system WebView in logical client coordinates.
func (w *Window) ShowWebView(content WebViewContent, bounds Rect) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	content.URL = strings.TrimSpace(content.URL)
	content.CacheKey = strings.TrimSpace(content.CacheKey)
	if content.URL == "" && content.HTML == "" {
		return errors.New("webview content requires a URL or HTML")
	}
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return errors.New("webview bounds must have a positive size")
	}
	return w.native.showWebView(content, bounds)
}

// HideWebView removes the embedded browser from the visible focus domain without discarding cached state.
func (w *Window) HideWebView() error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.hideWebView()
}
