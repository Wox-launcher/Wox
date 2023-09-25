//go:build darwin

package macos

// #cgo CFLAGS: -x objective-c
// #cgo LDFLAGS: -framework Cocoa -framework WebKit
// #include "webview.h"
import "C"
import (
	"unsafe"
)

type WebViewMacOs struct {
}

func (w *WebViewMacOs) CreateWebview(url string) {
	cURL := C.CString(url)
	defer C.free(unsafe.Pointer(cURL))
	C.createAndShowWindow(cURL)
}
