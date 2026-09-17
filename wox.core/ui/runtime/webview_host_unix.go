//go:build darwin || linux

package woxui

import "strings"

func (w *platformWindow) showWebView(content WebViewContent, bounds Rect) error {
	if err := w.webView.Show(toWebViewContent(content), toWebViewRect(bounds), 1); err != nil {
		return err
	}
	return w.applyWebViewActionHotkey()
}

func (w *platformWindow) forwardEmbeddedSurfacePointer(event PointerEvent) bool {
	return w.webView.Pointer(toWebViewPointerEvent(event))
}

func (w *platformWindow) hideWebView() error {
	return w.webView.Hide()
}

func (w *platformWindow) resetWebView() error {
	return w.webView.Reset()
}

func (w *platformWindow) webViewGoBack() error {
	return w.webView.GoBack()
}

func (w *platformWindow) webViewGoForward() error {
	return w.webView.GoForward()
}

func (w *platformWindow) webViewReload() error {
	return w.webView.Reload()
}

func (w *platformWindow) webViewOpenDevTools() error {
	return w.webView.OpenDevTools()
}

func (w *platformWindow) webViewOpenInBrowser() error {
	return w.webView.OpenInBrowser()
}

func (w *platformWindow) webViewNavigationState() (WebViewNavigationState, error) {
	state, err := w.webView.NavigationState()
	return fromWebViewNavigationState(state), err
}

func (w *platformWindow) focusWebView() error {
	if w.webView == nil {
		return ErrWebViewUnavailable
	}
	if err := w.applyWebViewActionHotkey(); err != nil {
		return err
	}
	return w.webView.Focus()
}

func (w *platformWindow) setWebViewActionHotkey(hotkey string) error {
	w.webViewActionHotkey = strings.TrimSpace(hotkey)
	return w.applyWebViewActionHotkey()
}
