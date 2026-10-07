//go:build windows

package clipboard

/*
#cgo CXXFLAGS: -std=c++17 -DUNICODE -D_UNICODE
#cgo LDFLAGS: -luser32 -lole32 -luuid -lstdc++
#include <stdlib.h>

#include "clipboard_image_windows.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

// publishNativeText keeps the supplied HWND and clipboard transaction on its owning window thread.
func publishNativeText(owner uintptr, text string) error {
	nativeText := C.CString(text)
	defer C.free(unsafe.Pointer(nativeText))
	result := C.wox_windows_write_clipboard_text(C.uintptr_t(owner), nativeText)
	if result < 0 {
		return fmt.Errorf("clipboard: publish text failed (HRESULT 0x%08x)", uint32(result))
	}
	return nil
}

const clipboardUsesEncodedPNG = false
const clipboardAcceptsPackedRGBA = true

type preparedImageNative struct{ handle *C.WoxWindowsClipboardImage }

// prepareNativeImage converts packed pixels off the UI thread and before opening the clipboard.
func prepareNativeImage(image *clipboardImage) (*preparedImageNative, error) {
	if image == nil || len(image.pixels) == 0 {
		return nil, errors.New("clipboard image is empty")
	}
	if image.width <= 0 || image.height <= 0 || image.width > 16384 || image.height > 16384 ||
		image.stride < image.width*4 || len(image.pixels) < image.width*4 ||
		image.height-1 > (len(image.pixels)-image.width*4)/image.stride {
		return nil, errors.New("clipboard pixel buffer is invalid")
	}
	var png *C.uint8_t
	if len(image.png) > 0 {
		png = (*C.uint8_t)(unsafe.Pointer(&image.png[0]))
	}
	var native *C.WoxWindowsClipboardImage
	var premultiplied C.int32_t
	if image.premultiplied {
		premultiplied = 1
	}
	result := C.wox_windows_prepare_clipboard_image(
		(*C.uint8_t)(unsafe.Pointer(&image.pixels[0])),
		C.uint32_t(image.width),
		C.uint32_t(image.height),
		C.uint32_t(image.stride),
		premultiplied,
		png,
		C.uint32_t(len(image.png)),
		&native,
	)
	if result < 0 {
		return nil, fmt.Errorf("clipboard: prepare image failed (HRESULT 0x%08x)", uint32(result))
	}
	return &preparedImageNative{handle: native}, nil
}

func releaseNativeImage(native *preparedImageNative) {
	C.wox_windows_destroy_clipboard_image(native.handle)
}

// publishNativeImage commits prepared handles while PreparedImage holds the lifetime lock.
func publishNativeImage(owner uintptr, _ *clipboardImage, native *preparedImageNative) error {
	result := C.wox_windows_publish_clipboard_image(C.uintptr_t(owner), native.handle)
	if result < 0 {
		return fmt.Errorf("clipboard: publish image failed (HRESULT 0x%08x)", uint32(result))
	}
	return nil
}

func flushNativeImage() error { return nil }
