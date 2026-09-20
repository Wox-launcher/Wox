//go:build windows

package woxui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/lxn/win"

	webviewruntime "wox/ui/runtime/internal/webview"
)

// TestWindowsHTMLCacheEviction exercises the native cache and environment release without showing a window.
func TestWindowsHTMLCacheEviction(t *testing.T) {
	if os.Getenv("WOX_WINDOWS_WEBVIEW_INTEGRATION") != "1" {
		t.Skip("set WOX_WINDOWS_WEBVIEW_INTEGRATION=1 to run native WebView cache eviction")
	}
	loader, err := filepath.Abs("../../resource/others/webview/WebView2Loader.dll")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("WOX_WEBVIEW2_LOADER_PATH", loader)
	err = Run(func() error {
		window, err := Open(WindowOptions{Title: "Wox HTML cache eviction test", Size: Size{Width: 320, Height: 240}})
		if err != nil {
			return err
		}
		defer window.Close()
		driver, err := newWindowsWebViewDriver(uintptr(window.native.hwnd), window.native.renderer)
		if err != nil {
			return err
		}
		window.native.webView = webviewruntime.New(driver, Call)
		bounds := Rect{Width: 100, Height: 80}
		if err := window.ShowWebView(WebViewContent{HTML: "<p>first</p>", CacheKey: "plugin-one"}, bounds); err != nil {
			return err
		}
		if err := window.ShowWebView(WebViewContent{HTML: "<p>second</p>", CacheKey: "plugin-two", CacheDisabled: true}, bounds); err != nil {
			return err
		}
		if err := driver.Evict("html"); err == nil {
			return fmt.Errorf("native eviction accepted the active HTML slot")
		}
		if err := window.ShowWebView(WebViewContent{URL: "https://example.com", CacheKey: "site"}, bounds); err != nil {
			return err
		}
		if err := driver.Evict("html"); err != nil {
			return err
		}
		if driver.handle == nil {
			return fmt.Errorf("HTML eviction released the active URL environment")
		}
		if err := driver.Evict("url|site"); err == nil {
			return fmt.Errorf("native eviction accepted the active URL session")
		}
		if err := window.HideWebView(); err != nil {
			return err
		}
		if err := driver.Evict("url|site"); err != nil {
			return err
		}
		if driver.handle != nil {
			return fmt.Errorf("last cache eviction retained the native environment or old plugin keys")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

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
		native.webView = webviewruntime.New(&webViewNavigationDriver{}, Call)
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
func (*webViewNavigationDriver) Hide() error        { return nil }
func (*webViewNavigationDriver) Evict(string) error { return nil }
func (*webViewNavigationDriver) Reset() error       { return nil }
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

func TestWindowsWebViewActionHotkeyMatchesConfiguredKeyOnly(t *testing.T) {
	window := &platformWindow{webViewActionHotkey: "ctrl+k"}
	if !window.matchesWebViewActionHotkey('K', KeyModifierControl) {
		t.Fatal("configured ctrl+k should match")
	}
	if window.matchesWebViewActionHotkey('J', KeyModifierControl) {
		t.Fatal("WebView must not reserve J when Action Hotkey is K")
	}
}

func TestFocusWebViewQueuesUntilControllerExists(t *testing.T) {
	window := &platformWindow{}
	result, handled := window.executeWebViewCommand(windowCommand{kind: windowCommandFocusWebView})
	if !handled || result.err != nil || !window.webViewFocusPending {
		t.Fatalf("queued focus = handled %t err %v pending %t", handled, result.err, window.webViewFocusPending)
	}

	driver := &webViewNavigationDriver{}
	window.webView = webviewruntime.New(driver, func(fn func()) error { fn(); return nil })
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
	window := &platformWindow{webView: webviewruntime.New(driver, func(fn func()) error { fn(); return nil })}

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
