//go:build windows

package macos

// #cgo CFLAGS: -DUNICODE
// #cgo LDFLAGS: -lWebView2Loader
// #include "webview.h"
import "C"
import (
	"unsafe"
)

type WebViewWindows struct {
}

func (w *WebViewWindows) CreateWebview(url string) {
	cURL := C.CString(url)
	defer C.free(unsafe.Pointer(cURL))
	C.createAndShowWindow(cURL)
}
