//go:build darwin

package woxui

/*
#include <stdlib.h>
#include "native_darwin.h"
*/
import "C"

import (
	"errors"
	"runtime/cgo"
	"unsafe"

	webviewruntime "wox/ui/runtime/internal/webview"
)

// darwinWebViewDriver adapts the AppKit window-owned WKWebView to the portable controller.
type darwinWebViewDriver struct {
	window *platformWindow
}

var _ webviewruntime.Driver = (*darwinWebViewDriver)(nil)

func (d *darwinWebViewDriver) Show(content webviewruntime.Content, bounds webviewruntime.Rect, scale float32) error {
	native, err := d.window.openNative()
	if err != nil {
		return err
	}
	url := C.CString(content.URL)
	html := C.CString(content.HTML)
	css := C.CString(content.InjectCSS)
	userAgent := C.CString(content.UserAgent)
	cacheKey := C.CString(content.CacheKey)
	defer C.free(unsafe.Pointer(url))
	defer C.free(unsafe.Pointer(html))
	defer C.free(unsafe.Pointer(css))
	defer C.free(unsafe.Pointer(userAgent))
	defer C.free(unsafe.Pointer(cacheKey))
	cacheDisabled := C.int32_t(0)
	if content.CacheDisabled {
		cacheDisabled = 1
	}
	if C.wox_darwin_window_show_webview(native, url, html, css, userAgent, cacheDisabled, cacheKey, C.float(bounds.X), C.float(bounds.Y), C.float(bounds.Width), C.float(bounds.Height)) != 0 {
		return errors.New("woxui: failed to show macOS WebView")
	}
	return nil
}

func (d *darwinWebViewDriver) Hide() error {
	native, err := d.window.openNative()
	if err != nil {
		return err
	}
	if C.wox_darwin_window_hide_webview(native) != 0 {
		return errors.New("woxui: failed to hide macOS WebView")
	}
	return nil
}

func (d *darwinWebViewDriver) Reset() error {
	native, err := d.window.openNative()
	if err != nil {
		return err
	}
	if C.wox_darwin_window_reset_webview(native) != 0 {
		return errors.New("woxui: failed to reset macOS WebView")
	}
	return nil
}

func (d *darwinWebViewDriver) GoBack() error {
	native, err := d.window.openNative()
	if err != nil {
		return err
	}
	if C.wox_darwin_window_webview_go_back(native) != 0 {
		return errors.New("woxui: failed to go back in macOS WebView")
	}
	return nil
}

func (d *darwinWebViewDriver) GoForward() error {
	native, err := d.window.openNative()
	if err != nil {
		return err
	}
	if C.wox_darwin_window_webview_go_forward(native) != 0 {
		return errors.New("woxui: failed to go forward in macOS WebView")
	}
	return nil
}

func (d *darwinWebViewDriver) Reload() error {
	native, err := d.window.openNative()
	if err != nil {
		return err
	}
	if C.wox_darwin_window_webview_reload(native) != 0 {
		return errors.New("woxui: failed to reload macOS WebView")
	}
	return nil
}

func (d *darwinWebViewDriver) OpenDevTools() error {
	native, err := d.window.openNative()
	if err != nil {
		return err
	}
	if C.wox_darwin_window_webview_open_dev_tools(native) != 0 {
		return errors.New("woxui: failed to open macOS WebView developer tools")
	}
	return nil
}

func (d *darwinWebViewDriver) OpenInBrowser() error {
	native, err := d.window.openNative()
	if err != nil {
		return err
	}
	if C.wox_darwin_window_webview_open_in_browser(native) != 0 {
		return errors.New("woxui: failed to open macOS WebView in browser")
	}
	return nil
}

func (d *darwinWebViewDriver) NavigationState() (webviewruntime.NavigationState, error) {
	native, err := d.window.openNative()
	if err != nil {
		return webviewruntime.NavigationState{}, err
	}
	var url *C.char
	var canGoBack, canGoForward C.int32_t
	if C.wox_darwin_window_webview_navigation_state(native, &url, &canGoBack, &canGoForward) != 0 {
		return webviewruntime.NavigationState{}, errors.New("woxui: failed to read macOS WebView navigation state")
	}
	state := webviewruntime.NavigationState{CanGoBack: canGoBack != 0, CanGoForward: canGoForward != 0}
	if url != nil {
		state.URL = C.GoString(url)
		C.free(unsafe.Pointer(url))
	}
	return state, nil
}

func (d *darwinWebViewDriver) Pointer(event webviewruntime.PointerEvent) bool {
	native, err := d.window.openNative()
	return err == nil && C.wox_darwin_window_forward_embedded_surface_pointer(native, C.uint8_t(event.Kind)) == 0
}

func (*darwinWebViewDriver) Close() {}

// woxGoDarwinWebViewEscapeDiagnostic records the page decision and native focus handoff.
//
//export woxGoDarwinWebViewEscapeDiagnostic
func woxGoDarwinWebViewEscapeDiagnostic(context C.uintptr_t, detail *C.char) {
	webviewruntime.LogEscapeDiagnostic("darwin", uintptr(context), C.GoString(detail))
}

//export woxGoDarwinWebViewHideRequested
func woxGoDarwinWebViewHideRequested(context C.uintptr_t) {
	window := cgo.Handle(context).Value().(*platformWindow)
	if window.options.OnWebViewHideRequested != nil {
		window.options.OnWebViewHideRequested()
	}
}

//export woxGoDarwinWebViewTooltip
func woxGoDarwinWebViewTooltip(context C.uintptr_t, visible C.int32_t, tooltip *C.char, x C.float, y C.float, width C.float, height C.float) {
	window := cgo.Handle(context).Value().(*platformWindow)
	if window.options.OnWebViewTooltip == nil {
		return
	}
	window.options.OnWebViewTooltip(WebViewTooltipEvent{
		Visible: visible != 0,
		Text:    C.GoString(tooltip),
		Bounds:  Rect{X: float32(x), Y: float32(y), Width: float32(width), Height: float32(height)},
	})
}

//export woxGoDarwinWebViewNavigationChanged
func woxGoDarwinWebViewNavigationChanged(context C.uintptr_t, url *C.char, canGoBack, canGoForward C.int32_t) {
	window := cgo.Handle(context).Value().(*platformWindow)
	if window.options.OnWebViewNavigationChanged == nil {
		return
	}
	state := WebViewNavigationState{CanGoBack: canGoBack != 0, CanGoForward: canGoForward != 0}
	if url != nil {
		state.URL = C.GoString(url)
	}
	window.options.OnWebViewNavigationChanged(state)
}
