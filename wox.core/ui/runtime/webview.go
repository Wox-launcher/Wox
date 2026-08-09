package woxui

import (
	"errors"

	webviewruntime "wox/ui/runtime/internal/webview"
)

// ErrWebViewUnavailable reports that the current desktop is missing its system WebView runtime.
var ErrWebViewUnavailable = webviewruntime.ErrUnavailable

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
	normalized, err := webviewruntime.Normalize(toWebViewContent(content), toWebViewRect(bounds))
	if err != nil {
		return err
	}
	return w.native.showWebView(fromWebViewContent(normalized), bounds)
}

func isAbsoluteWebViewURL(rawURL string) bool {
	return webviewruntime.IsAbsoluteURL(rawURL)
}

func toWebViewRect(rect Rect) webviewruntime.Rect {
	return webviewruntime.Rect{X: rect.X, Y: rect.Y, Width: rect.Width, Height: rect.Height}
}

func toWebViewContent(content WebViewContent) webviewruntime.Content {
	return webviewruntime.Content{
		URL: content.URL, HTML: content.HTML, InjectCSS: content.InjectCSS,
		CacheDisabled: content.CacheDisabled, CacheKey: content.CacheKey,
	}
}

func fromWebViewContent(content webviewruntime.Content) WebViewContent {
	return WebViewContent{
		URL: content.URL, HTML: content.HTML, InjectCSS: content.InjectCSS,
		CacheDisabled: content.CacheDisabled, CacheKey: content.CacheKey,
	}
}

func fromWebViewNavigationState(state webviewruntime.NavigationState) WebViewNavigationState {
	return WebViewNavigationState{URL: state.URL, CanGoBack: state.CanGoBack, CanGoForward: state.CanGoForward}
}

func toWebViewPointerEvent(event PointerEvent) webviewruntime.PointerEvent {
	return webviewruntime.PointerEvent{
		Kind: uint8(event.Kind), Position: webviewruntime.Point{X: event.Position.X, Y: event.Position.Y}, Button: uint8(event.Button),
		Scroll: webviewruntime.Point{X: event.Scroll.X, Y: event.Scroll.Y}, Modifiers: uint8(event.Modifiers),
	}
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

// WebViewOpenDevTools opens the developer tools for the active WebView document.
func (w *Window) WebViewOpenDevTools() error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.webViewOpenDevTools()
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
