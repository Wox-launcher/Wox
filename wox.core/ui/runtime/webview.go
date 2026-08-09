package woxui

import (
	"errors"
	"fmt"
	"net/url"
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

// WebViewNavigationState mirrors the live browser chrome for an attached WebView.
type WebViewNavigationState struct {
	URL          string
	CanGoBack    bool
	CanGoForward bool
}

// WebViewTooltipEvent reports native toolbar hover in virtual desktop coordinates.
// Deprecated: floating WebView toolbars are replaced by the Go UI title bar.
type WebViewTooltipEvent struct {
	Visible bool
	Text    string
	Bounds  Rect
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
	if content.HTML == "" && !isAbsoluteWebViewURL(content.URL) {
		return fmt.Errorf("webview URL must be an absolute http(s) URL: %q", content.URL)
	}
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return errors.New("webview bounds must have a positive size")
	}
	return w.native.showWebView(content, bounds)
}

func isAbsoluteWebViewURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Scheme, "http") || strings.EqualFold(parsed.Scheme, "https")
}

// HideWebView removes the embedded browser from the visible focus domain without discarding cached state.
func (w *Window) HideWebView() error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.hideWebView()
}

// ResetWebView destroys the current native browser and its cached sessions.
func (w *Window) ResetWebView() error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.resetWebView()
}

// WebViewGoBack navigates the active WebView to the previous history entry when available.
func (w *Window) WebViewGoBack() error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.webViewGoBack()
}

// WebViewGoForward navigates the active WebView to the next history entry when available.
func (w *Window) WebViewGoForward() error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.webViewGoForward()
}

// WebViewReload reloads the active WebView document.
func (w *Window) WebViewReload() error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.webViewReload()
}

// WebViewOpenInBrowser opens the active WebView's http(s) URL in the system browser.
func (w *Window) WebViewOpenInBrowser() error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.webViewOpenInBrowser()
}

// WebViewNavigationState returns the latest known navigation chrome for the active WebView.
func (w *Window) WebViewNavigationState() (WebViewNavigationState, error) {
	if w == nil || w.native == nil {
		return WebViewNavigationState{}, errors.New("window is not initialized")
	}
	return w.native.webViewNavigationState()
}

// ForwardEmbeddedSurfacePointer routes host-tested, surface-local input to the active platform composition surface.
func (w *Window) ForwardEmbeddedSurfacePointer(event PointerEvent) bool {
	if w == nil || w.native == nil {
		return false
	}
	return w.native.forwardEmbeddedSurfacePointer(event)
}
