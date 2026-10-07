package woxui

import (
	window "wox/ui/runtime/internal/window"
)

var ErrWebViewUnavailable = window.ErrWebViewUnavailable

// WebViewContent describes one embedded browser document while Rect is controlled separately by layout.
// HTML ignores cache options and uses one temporary instance, released after 10 seconds out of view.
type WebViewContent = window.WebViewContent

// WebViewNavigationState mirrors the live browser chrome for an attached WebView.
type WebViewNavigationState = window.WebViewNavigationState

// WebViewTooltipEvent reports native toolbar hover in virtual desktop coordinates.
// Deprecated: floating WebView toolbars are replaced by the Go UI title bar.
type WebViewTooltipEvent = window.WebViewTooltipEvent
