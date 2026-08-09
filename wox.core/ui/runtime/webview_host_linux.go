//go:build linux

package woxui

/*
#include <stdlib.h>
#include "native_linux.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"

	webviewruntime "wox/ui/runtime/internal/webview"
)

// linuxWebViewDriver adapts the GTK window-owned WebKit surface to the portable controller.
type linuxWebViewDriver struct {
	window *platformWindow
}

var _ webviewruntime.Driver = (*linuxWebViewDriver)(nil)

func (d *linuxWebViewDriver) Show(content webviewruntime.Content, bounds webviewruntime.Rect, scale float32) error {
	native, err := d.window.openNative()
	if err != nil {
		return err
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
	result := C.wox_linux_window_show_webview(native, url, html, css, cacheDisabled, cacheKey, C.float(bounds.X), C.float(bounds.Y), C.float(bounds.Width), C.float(bounds.Height))
	if result == -2 {
		return fmt.Errorf("%w: install WebKitGTK 4.1 or 4.0", webviewruntime.ErrUnavailable)
	}
	if result != 0 {
		return errors.New("woxui: failed to show Linux WebView")
	}
	return nil
}

func (d *linuxWebViewDriver) Hide() error {
	native, err := d.window.openNative()
	if err != nil {
		return err
	}
	if C.wox_linux_window_hide_webview(native) != 0 {
		return errors.New("woxui: failed to hide Linux WebView")
	}
	return nil
}

func (d *linuxWebViewDriver) Reset() error {
	native, err := d.window.openNative()
	if err != nil {
		return err
	}
	if C.wox_linux_window_reset_webview(native) != 0 {
		return errors.New("woxui: failed to reset Linux WebView")
	}
	return nil
}

func (*linuxWebViewDriver) GoBack() error {
	return errors.New("woxui: linux WebView navigation is not implemented")
}

func (*linuxWebViewDriver) GoForward() error {
	return errors.New("woxui: linux WebView navigation is not implemented")
}

func (*linuxWebViewDriver) Reload() error {
	return errors.New("woxui: linux WebView navigation is not implemented")
}

func (*linuxWebViewDriver) OpenDevTools() error {
	return errors.New("woxui: linux WebView developer tools are not implemented")
}

func (*linuxWebViewDriver) OpenInBrowser() error {
	return errors.New("woxui: linux WebView navigation is not implemented")
}

func (*linuxWebViewDriver) NavigationState() (webviewruntime.NavigationState, error) {
	return webviewruntime.NavigationState{}, errors.New("woxui: linux WebView navigation is not implemented")
}

func (d *linuxWebViewDriver) Pointer(event webviewruntime.PointerEvent) bool {
	native, err := d.window.openNative()
	return err == nil && C.wox_linux_window_forward_embedded_surface_pointer(native, C.float(event.Position.X), C.float(event.Position.Y)) == 0
}

func (*linuxWebViewDriver) Close() {}

// woxGoLinuxWebViewEscapeDiagnostic records the page decision and native focus handoff.
//
//export woxGoLinuxWebViewEscapeDiagnostic
func woxGoLinuxWebViewEscapeDiagnostic(context C.uintptr_t, detail *C.char) {
	webviewruntime.LogEscapeDiagnostic("linux", uintptr(context), C.GoString(detail))
}
