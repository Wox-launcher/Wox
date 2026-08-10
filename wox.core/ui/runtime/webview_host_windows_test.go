//go:build windows

package woxui

import (
	"testing"

	"github.com/lxn/win"

	webviewruntime "wox/ui/runtime/internal/webview"
)

type webViewNavigationDriver struct {
	backCalls    int
	forwardCalls int
}

func (*webViewNavigationDriver) Show(webviewruntime.Content, webviewruntime.Rect, float32) error {
	return nil
}
func (*webViewNavigationDriver) Hide() error  { return nil }
func (*webViewNavigationDriver) Reset() error { return nil }
func (d *webViewNavigationDriver) GoBack() error {
	d.backCalls++
	return nil
}
func (d *webViewNavigationDriver) GoForward() error {
	d.forwardCalls++
	return nil
}
func (*webViewNavigationDriver) Reload() error        { return nil }
func (*webViewNavigationDriver) OpenDevTools() error  { return nil }
func (*webViewNavigationDriver) OpenInBrowser() error { return nil }
func (*webViewNavigationDriver) NavigationState() (webviewruntime.NavigationState, error) {
	return webviewruntime.NavigationState{}, nil
}
func (*webViewNavigationDriver) Pointer(webviewruntime.PointerEvent) bool { return true }
func (*webViewNavigationDriver) Close()                                   {}

func TestWebViewCursorOverridesHostOnlyWhilePointerIsOverSurface(t *testing.T) {
	const webViewCursor = win.HCURSOR(123)
	window := &platformWindow{
		pointerCursor:      PointerCursorText,
		webViewCursor:      webViewCursor,
		webViewCursorKnown: true,
		webViewPointerOver: true,
	}

	if actual := window.resolvedPointerCursor(); actual != webViewCursor {
		t.Fatalf("WebView cursor = %v, want %v", actual, webViewCursor)
	}
	window.webViewCursor = 0
	if actual := window.resolvedPointerCursor(); actual != 0 {
		t.Fatalf("CSS cursor:none = %v, want no cursor", actual)
	}
	window.webViewCursor = webViewCursor

	window.webViewPointerOver = false
	if actual := window.resolvedPointerCursor(); actual == webViewCursor {
		t.Fatal("WebView cursor remained active after the pointer left its surface")
	}

	window.webViewPointerOver = true
	window.clearWebViewPointerState()
	if actual := window.resolvedPointerCursor(); actual == webViewCursor {
		t.Fatal("WebView cursor remained active after clearing the embedded surface state")
	}
}

func TestWebViewXButtonsNavigateOnlyWhilePointerIsOverSurface(t *testing.T) {
	driver := &webViewNavigationDriver{}
	window := &platformWindow{webView: webviewruntime.New(driver)}

	if window.handleWebViewXButton(win.XBUTTON1, true) {
		t.Fatal("XButton1 was handled while the pointer was outside the WebView")
	}
	window.webViewPointerOver = true
	if !window.handleWebViewXButton(win.XBUTTON1, true) || driver.backCalls != 1 {
		t.Fatalf("XButton1 navigation = handled with %d back calls, want one", driver.backCalls)
	}
	if !window.handleWebViewXButton(win.XBUTTON2, true) || driver.forwardCalls != 1 {
		t.Fatalf("XButton2 navigation = handled with %d forward calls, want one", driver.forwardCalls)
	}
	if !window.handleWebViewXButton(win.XBUTTON1, false) || driver.backCalls != 1 {
		t.Fatalf("XButton1 release changed navigation calls to %d", driver.backCalls)
	}
	if window.handleWebViewXButton(0, true) {
		t.Fatal("unknown XButton was handled")
	}
}
