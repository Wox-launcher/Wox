//go:build windows

package woxui

import (
	"fmt"
	"os"
	"testing"

	"github.com/lxn/win"

	webviewruntime "wox/ui/runtime/internal/webview"
)

// TestWebViewRetainsHiddenRenderer exercises trimming with a real native renderer and cached controller.
func TestWebViewRetainsHiddenRenderer(t *testing.T) {
	if os.Getenv("WOX_WINDOWS_WEBVIEW_TRIM_INTEGRATION") != "1" {
		t.Skip("set WOX_WINDOWS_WEBVIEW_TRIM_INTEGRATION=1 to run the native renderer lifetime test")
	}
	err := Run(func() error {
		window, err := Open(WindowOptions{Title: "Wox WebView renderer lifetime test", Size: Size{Width: 320, Height: 240}})
		if err != nil {
			return err
		}
		defer window.Close()
		native := window.native
		original := native.renderer.handle
		if original == nil || native.focus.visible {
			return fmt.Errorf("expected an initialized hidden renderer")
		}
		native.webView = webviewruntime.New(&webViewNavigationDriver{})
		if result := native.executeCommand(windowCommand{kind: windowCommandTrimRenderer}); result.err != nil {
			return result.err
		}
		if native.renderer.handle != original {
			return fmt.Errorf("trim destroyed the renderer retained by a cached WebView")
		}
		native.webView.Close()
		native.webView = nil
		if result := native.executeCommand(windowCommand{kind: windowCommandTrimRenderer}); result.err != nil {
			return result.err
		}
		if native.renderer.handle != nil {
			return fmt.Errorf("trim retained the renderer after the WebView was released")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

type webViewNavigationDriver struct {
	backCalls    int
	forwardCalls int
	focusCalled  bool
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
func (d *webViewNavigationDriver) Focus() error {
	d.focusCalled = true
	return nil
}
func (*webViewNavigationDriver) Close() {}

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

func TestFocusWebViewQueuesUntilControllerExists(t *testing.T) {
	window := &platformWindow{}
	result, handled := window.executeWebViewCommand(windowCommand{kind: windowCommandFocusWebView})
	if !handled || result.err != nil || !window.webViewFocusPending {
		t.Fatalf("queued focus = handled %t err %v pending %t", handled, result.err, window.webViewFocusPending)
	}

	driver := &webViewNavigationDriver{}
	window.webView = webviewruntime.New(driver)
	result, handled = window.executeWebViewCommand(windowCommand{
		kind:          windowCommandShowWebView,
		webView:       WebViewContent{URL: "https://example.com"},
		webViewBounds: Rect{Width: 100, Height: 80},
	})
	if !handled || result.err != nil || !driver.focusCalled || window.webViewFocusPending {
		t.Fatalf("show applied focus = handled %t err %v focusCalled %t pending %t", handled, result.err, driver.focusCalled, window.webViewFocusPending)
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
