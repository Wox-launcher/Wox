//go:build windows

package woxui

/*
#cgo CXXFLAGS: -std=c++17 -DUNICODE -D_UNICODE
#include <stdlib.h>
#include "native_windows.h"
*/
import "C"

import (
	"fmt"
	"unsafe"

	webviewruntime "wox/ui/runtime/internal/webview"
)

func (w *platformWindow) showWebView(content WebViewContent, bounds Rect) error {
	return w.call(windowCommand{kind: windowCommandShowWebView, webView: content, webViewBounds: bounds}).err
}

func (w *platformWindow) hideWebView() error {
	return w.call(windowCommand{kind: windowCommandHideWebView}).err
}

func (w *platformWindow) resetWebView() error {
	return w.call(windowCommand{kind: windowCommandResetWebView}).err
}

func (w *platformWindow) webViewGoBack() error {
	return w.call(windowCommand{kind: windowCommandWebViewGoBack}).err
}

func (w *platformWindow) webViewGoForward() error {
	return w.call(windowCommand{kind: windowCommandWebViewGoForward}).err
}

func (w *platformWindow) webViewReload() error {
	return w.call(windowCommand{kind: windowCommandWebViewReload}).err
}

func (w *platformWindow) webViewOpenInBrowser() error {
	return w.call(windowCommand{kind: windowCommandWebViewOpenInBrowser}).err
}

func (w *platformWindow) webViewNavigationState() (WebViewNavigationState, error) {
	result := w.call(windowCommand{kind: windowCommandWebViewNavigationState})
	return result.navigation, result.err
}

func (w *platformWindow) forwardEmbeddedSurfacePointer(event PointerEvent) bool {
	return w.webView != nil && w.webView.Pointer(toWebViewPointerEvent(event))
}

// executeWebViewCommand keeps WebView lifecycle out of the general Win32 command switch.
func (w *platformWindow) executeWebViewCommand(command windowCommand) (windowCommandResult, bool) {
	switch command.kind {
	case windowCommandShowWebView:
		if w.webView == nil {
			driver, err := newWindowsWebViewDriver(uintptr(w.hwnd), w.renderer)
			if err != nil {
				return windowCommandResult{err: err}, true
			}
			w.webView = webviewruntime.New(driver)
		}
		return windowCommandResult{err: w.webView.Show(toWebViewContent(command.webView), toWebViewRect(command.webViewBounds), w.scale)}, true
	case windowCommandHideWebView:
		if w.webView == nil {
			return windowCommandResult{}, true
		}
		return windowCommandResult{err: w.webView.Hide()}, true
	case windowCommandResetWebView:
		if w.webView == nil {
			return windowCommandResult{}, true
		}
		return windowCommandResult{err: w.webView.Reset()}, true
	case windowCommandWebViewGoBack:
		if w.webView == nil {
			return windowCommandResult{err: ErrWebViewUnavailable}, true
		}
		return windowCommandResult{err: w.webView.GoBack()}, true
	case windowCommandWebViewGoForward:
		if w.webView == nil {
			return windowCommandResult{err: ErrWebViewUnavailable}, true
		}
		return windowCommandResult{err: w.webView.GoForward()}, true
	case windowCommandWebViewReload:
		if w.webView == nil {
			return windowCommandResult{err: ErrWebViewUnavailable}, true
		}
		return windowCommandResult{err: w.webView.Reload()}, true
	case windowCommandWebViewOpenInBrowser:
		if w.webView == nil {
			return windowCommandResult{err: ErrWebViewUnavailable}, true
		}
		return windowCommandResult{err: w.webView.OpenInBrowser()}, true
	case windowCommandWebViewNavigationState:
		if w.webView == nil {
			return windowCommandResult{err: ErrWebViewUnavailable}, true
		}
		state, err := w.webView.NavigationState()
		return windowCommandResult{navigation: fromWebViewNavigationState(state), err: err}, true
	default:
		return windowCommandResult{}, false
	}
}

type windowsWebViewDriver struct {
	handle   *C.WoxWindowsWebView
	owner    uintptr
	renderer *nativeRenderer
	scale    float32
}

var _ webviewruntime.Driver = (*windowsWebViewDriver)(nil)

// newWindowsWebViewDriver starts the asynchronous WebView2 environment on the window UI thread.
func newWindowsWebViewDriver(owner uintptr, renderer *nativeRenderer) (*windowsWebViewDriver, error) {
	driver := &windowsWebViewDriver{owner: owner, renderer: renderer}
	if err := driver.create(); err != nil {
		return nil, err
	}
	return driver, nil
}

// create allocates a fresh native controller after construction or Reset.
func (w *windowsWebViewDriver) create() error {
	if w == nil || w.renderer == nil || w.renderer.handle == nil {
		return webviewruntime.ErrUnavailable
	}
	result := C.wox_windows_webview_create(C.uintptr_t(w.owner), (*C.WoxRenderer)(w.renderer.handle), &w.handle)
	if result < 0 {
		code := uint32(result)
		if code == 0x8007007E || code == 0x8007007F {
			return fmt.Errorf("%w: WebView2Loader.dll is missing; place it beside the executable or set WOX_WEBVIEW2_LOADER_PATH", webviewruntime.ErrUnavailable)
		}
		return webViewHRESULT("initialize WebView2", result)
	}
	return nil
}

func (w *windowsWebViewDriver) Show(content webviewruntime.Content, bounds webviewruntime.Rect, scale float32) error {
	if w == nil {
		return webviewruntime.ErrUnavailable
	}
	if w.handle == nil {
		if err := w.create(); err != nil {
			return err
		}
	}
	if scale <= 0 {
		scale = 1
	}
	w.scale = scale
	url := C.CString(content.URL)
	html := C.CString(content.HTML)
	css := C.CString(content.InjectCSS)
	cacheKey := C.CString(content.CacheKey)
	defer C.free(unsafe.Pointer(url))
	defer C.free(unsafe.Pointer(html))
	defer C.free(unsafe.Pointer(css))
	defer C.free(unsafe.Pointer(cacheKey))
	cacheDisabled := C.int32_t(0)
	if content.CacheDisabled {
		cacheDisabled = 1
	}
	result := C.wox_windows_webview_show(
		w.handle,
		url,
		html,
		css,
		cacheDisabled,
		cacheKey,
		C.int32_t(bounds.X*scale+0.5),
		C.int32_t(bounds.Y*scale+0.5),
		C.int32_t(bounds.Width*scale+0.5),
		C.int32_t(bounds.Height*scale+0.5),
	)
	if result < 0 {
		return webViewHRESULT("show WebView2", result)
	}
	return nil
}

func (w *windowsWebViewDriver) Pointer(event webviewruntime.PointerEvent) bool {
	if w == nil || w.handle == nil {
		return false
	}
	scale := w.scale
	if scale <= 0 {
		scale = 1
	}
	result := C.wox_windows_webview_pointer(
		w.handle,
		C.int32_t(event.Kind),
		C.int32_t(event.Position.X*scale+0.5),
		C.int32_t(event.Position.Y*scale+0.5),
		C.int32_t(event.Button),
		C.int32_t(event.Scroll.X/pointerScrollLine*120),
		C.int32_t(event.Scroll.Y/pointerScrollLine*120),
		C.uint32_t(event.Modifiers),
	)
	return result >= 0
}

func (w *windowsWebViewDriver) Hide() error {
	if w == nil || w.handle == nil {
		return nil
	}
	result := C.wox_windows_webview_hide(w.handle)
	if result < 0 {
		return webViewHRESULT("hide WebView2", result)
	}
	return nil
}

func (w *windowsWebViewDriver) GoBack() error {
	if w == nil || w.handle == nil {
		return webviewruntime.ErrUnavailable
	}
	result := C.wox_windows_webview_go_back(w.handle)
	if result < 0 {
		return webViewHRESULT("webview go back", result)
	}
	return nil
}

func (w *windowsWebViewDriver) GoForward() error {
	if w == nil || w.handle == nil {
		return webviewruntime.ErrUnavailable
	}
	result := C.wox_windows_webview_go_forward(w.handle)
	if result < 0 {
		return webViewHRESULT("webview go forward", result)
	}
	return nil
}

func (w *windowsWebViewDriver) Reload() error {
	if w == nil || w.handle == nil {
		return webviewruntime.ErrUnavailable
	}
	result := C.wox_windows_webview_reload(w.handle)
	if result < 0 {
		return webViewHRESULT("webview reload", result)
	}
	return nil
}

func (w *windowsWebViewDriver) OpenInBrowser() error {
	if w == nil || w.handle == nil {
		return webviewruntime.ErrUnavailable
	}
	result := C.wox_windows_webview_open_in_browser(w.handle)
	if result < 0 {
		return webViewHRESULT("webview open in browser", result)
	}
	return nil
}

func (w *windowsWebViewDriver) NavigationState() (webviewruntime.NavigationState, error) {
	if w == nil || w.handle == nil {
		return webviewruntime.NavigationState{}, webviewruntime.ErrUnavailable
	}
	var url *C.char
	var canGoBack, canGoForward C.int32_t
	result := C.wox_windows_webview_navigation_state(w.handle, &url, &canGoBack, &canGoForward)
	if result < 0 {
		return webviewruntime.NavigationState{}, webViewHRESULT("webview navigation state", result)
	}
	state := webviewruntime.NavigationState{CanGoBack: canGoBack != 0, CanGoForward: canGoForward != 0}
	if url != nil {
		state.URL = C.GoString(url)
		C.wox_windows_free_string(url)
	}
	return state, nil
}

func (w *windowsWebViewDriver) Reset() error {
	w.destroy()
	return nil
}

func (w *windowsWebViewDriver) Close() {
	w.destroy()
}

func (w *windowsWebViewDriver) destroy() {
	if w != nil && w.handle != nil {
		C.wox_windows_webview_destroy(w.handle)
		w.handle = nil
	}
}

func webViewHRESULT(operation string, result C.int32_t) error {
	return fmt.Errorf("woxui: %s failed with HRESULT 0x%08X", operation, uint32(result))
}

// woxGoWindowsWebViewEscapeDiagnostic records the page decision and native focus handoff.
//
//export woxGoWindowsWebViewEscapeDiagnostic
func woxGoWindowsWebViewEscapeDiagnostic(owner C.uintptr_t, detail *C.char) {
	webviewruntime.LogEscapeDiagnostic("windows", uintptr(owner), C.GoString(detail))
}

// woxGoWindowsWebViewEscape forwards only WebView escape fallback, leaving normal browser input native.
//
//export woxGoWindowsWebViewEscape
func woxGoWindowsWebViewEscape(owner C.uintptr_t) C.int32_t {
	value, ok := nativeWindows.Load(uintptr(owner))
	if !ok {
		webviewruntime.LogEscapeDiagnostic("windows", uintptr(owner), "host-dispatch owner-not-found")
		return 0
	}
	window := value.(*platformWindow)
	hasCallback := window.options.OnKey != nil
	handled := hasCallback && window.options.OnKey(KeyEvent{Key: KeyEscape, Down: true})
	webviewruntime.LogEscapeDiagnostic("windows", uintptr(owner), fmt.Sprintf("host-dispatch callback=%t handled=%t", hasCallback, handled))
	if handled {
		return 1
	}
	return 0
}

// woxGoWindowsWebViewActionPanel forwards the launcher-reserved shortcut from focused WebView content.
//
//export woxGoWindowsWebViewActionPanel
func woxGoWindowsWebViewActionPanel(owner C.uintptr_t) C.int32_t {
	value, ok := nativeWindows.Load(uintptr(owner))
	if !ok {
		return 0
	}
	window := value.(*platformWindow)
	if window.options.OnKey != nil && window.options.OnKey(KeyEvent{Key: Key("j"), Modifiers: KeyModifierControl, Down: true}) {
		return 1
	}
	return 0
}

// woxGoWindowsWebViewNavigationChanged pushes live browser chrome into the Go UI title bar.
//
//export woxGoWindowsWebViewNavigationChanged
func woxGoWindowsWebViewNavigationChanged(owner C.uintptr_t, url *C.char, canGoBack, canGoForward C.int32_t) {
	value, ok := nativeWindows.Load(uintptr(owner))
	if !ok {
		return
	}
	window := value.(*platformWindow)
	if window.options.OnWebViewNavigationChanged == nil {
		return
	}
	state := WebViewNavigationState{CanGoBack: canGoBack != 0, CanGoForward: canGoForward != 0}
	if url != nil {
		state.URL = C.GoString(url)
	}
	window.options.OnWebViewNavigationChanged(state)
}
