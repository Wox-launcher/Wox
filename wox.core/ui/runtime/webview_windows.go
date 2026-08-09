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
)

type windowsWebView struct {
	handle *C.WoxWindowsWebView
}

// newWindowsWebView starts the asynchronous WebView2 environment on the window UI thread.
func newWindowsWebView(owner uintptr) (*windowsWebView, error) {
	var handle *C.WoxWindowsWebView
	result := C.wox_windows_webview_create(C.uintptr_t(owner), &handle)
	if result < 0 {
		code := uint32(result)
		if code == 0x8007007E || code == 0x8007007F {
			return nil, fmt.Errorf("%w: WebView2Loader.dll is missing; place it beside the executable or set WOX_WEBVIEW2_LOADER_PATH", ErrWebViewUnavailable)
		}
		return nil, webViewHRESULT("initialize WebView2", result)
	}
	return &windowsWebView{handle: handle}, nil
}

func (w *windowsWebView) show(content WebViewContent, bounds Rect, scale float32) error {
	if w == nil || w.handle == nil {
		return ErrWebViewUnavailable
	}
	if scale <= 0 {
		scale = 1
	}
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

func (w *windowsWebView) hide() error {
	if w == nil || w.handle == nil {
		return nil
	}
	result := C.wox_windows_webview_hide(w.handle)
	if result < 0 {
		return webViewHRESULT("hide WebView2", result)
	}
	return nil
}

func (w *windowsWebView) goBack() error {
	if w == nil || w.handle == nil {
		return ErrWebViewUnavailable
	}
	result := C.wox_windows_webview_go_back(w.handle)
	if result < 0 {
		return webViewHRESULT("webview go back", result)
	}
	return nil
}

func (w *windowsWebView) goForward() error {
	if w == nil || w.handle == nil {
		return ErrWebViewUnavailable
	}
	result := C.wox_windows_webview_go_forward(w.handle)
	if result < 0 {
		return webViewHRESULT("webview go forward", result)
	}
	return nil
}

func (w *windowsWebView) reload() error {
	if w == nil || w.handle == nil {
		return ErrWebViewUnavailable
	}
	result := C.wox_windows_webview_reload(w.handle)
	if result < 0 {
		return webViewHRESULT("webview reload", result)
	}
	return nil
}

func (w *windowsWebView) openInBrowser() error {
	if w == nil || w.handle == nil {
		return ErrWebViewUnavailable
	}
	result := C.wox_windows_webview_open_in_browser(w.handle)
	if result < 0 {
		return webViewHRESULT("webview open in browser", result)
	}
	return nil
}

func (w *windowsWebView) navigationState() (WebViewNavigationState, error) {
	if w == nil || w.handle == nil {
		return WebViewNavigationState{}, ErrWebViewUnavailable
	}
	var url *C.char
	var canGoBack, canGoForward C.int32_t
	result := C.wox_windows_webview_navigation_state(w.handle, &url, &canGoBack, &canGoForward)
	if result < 0 {
		return WebViewNavigationState{}, webViewHRESULT("webview navigation state", result)
	}
	state := WebViewNavigationState{CanGoBack: canGoBack != 0, CanGoForward: canGoForward != 0}
	if url != nil {
		state.URL = C.GoString(url)
		C.wox_windows_free_string(url)
	}
	return state, nil
}

func (w *windowsWebView) destroy() {
	if w != nil && w.handle != nil {
		C.wox_windows_webview_destroy(w.handle)
		w.handle = nil
	}
}

func webViewHRESULT(operation string, result C.int32_t) error {
	return fmt.Errorf("woxui: %s failed with HRESULT 0x%08X", operation, uint32(result))
}

// woxGoWindowsWebViewEscape forwards only WebView escape fallback, leaving normal browser input native.
//
//export woxGoWindowsWebViewEscape
func woxGoWindowsWebViewEscape(owner C.uintptr_t) C.int32_t {
	value, ok := nativeWindows.Load(uintptr(owner))
	if !ok {
		return 0
	}
	window := value.(*platformWindow)
	if window.options.OnKey != nil && window.options.OnKey(KeyEvent{Key: KeyEscape, Down: true}) {
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
