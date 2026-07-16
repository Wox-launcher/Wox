//go:build windows

package woxui

/*
#cgo CXXFLAGS: -std=c++17 -DUNICODE -D_UNICODE
#cgo LDFLAGS: -luser32 -lole32 -luuid -lstdc++
#include <stdlib.h>
#include "native_windows.h"
*/
import "C"

import (
	"errors"
	"unsafe"
)

func writeClipboardTextNative(owner uintptr, text string) error {
	nativeText := C.CString(text)
	defer C.free(unsafe.Pointer(nativeText))
	result := C.wox_windows_write_clipboard_text(C.uintptr_t(owner), nativeText)
	if result < 0 {
		return hresultError("write clipboard text", result)
	}
	return nil
}

func writeClipboardImageNative(owner uintptr, image *clipboardImage) error {
	if image == nil || len(image.pixels) == 0 || len(image.png) == 0 {
		return errors.New("clipboard image is empty")
	}
	result := C.wox_windows_write_clipboard_image(
		C.uintptr_t(owner),
		(*C.uint8_t)(unsafe.Pointer(&image.pixels[0])),
		C.uint32_t(image.width),
		C.uint32_t(image.height),
		C.uint32_t(image.stride),
		(*C.uint8_t)(unsafe.Pointer(&image.png[0])),
		C.uint32_t(len(image.png)),
	)
	if result < 0 {
		return hresultError("write clipboard image", result)
	}
	return nil
}
