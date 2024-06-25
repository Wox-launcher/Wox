//go:build darwin

package macos

import "C"

// #cgo CFLAGS: -x objective-c
// #cgo LDFLAGS: -framework Cocoa -framework WebKit
// #include <stdlib.h>
//
// void createAndShowWindow(const char *url);
import "C"
import (
	"golang.design/x/hotkey/mainthread"
	"unsafe"
)

type WebViewMacOs struct {
}

func (w *WebViewMacOs) CreateWebview(url string) {
	mainthread.Call(func() {
		cURL := C.CString(url)
		defer C.free(unsafe.Pointer(cURL))
		C.createAndShowWindow(cURL)
	})
}
