//go:build darwin

package clipboard

/*
#cgo CFLAGS: -fblocks -Wno-deprecated-declarations
#cgo LDFLAGS: -framework ImageIO
#include <stdlib.h>
#include "clipboard_image_darwin.h"
*/
import "C"

import (
	"errors"
	"unsafe"
)

// publishNativeImage publishes PNG directly, or normalizes native pixels for compatibility consumers.
func publishNativeImage(_ uintptr, payload *clipboardImage, _ *preparedImageNative) error {
	var result C.int32_t
	if len(payload.png) > 0 {
		result = C.wox_clipboard_darwin_write_png((*C.uint8_t)(unsafe.Pointer(&payload.png[0])), C.size_t(len(payload.png)))
	} else {
		result = C.wox_clipboard_darwin_write_pixels((*C.uint8_t)(unsafe.Pointer(&payload.pixels[0])),
			C.int32_t(payload.width), C.int32_t(payload.height), C.int32_t(payload.stride))
	}
	if result != 0 {
		return errors.New("clipboard: failed to publish macOS image")
	}
	return nil
}

// publishNativeText uses AppKit's owning thread for both UI and high-level clipboard calls.
func publishNativeText(_ uintptr, text string) error {
	nativeText := C.CString(text)
	defer C.free(unsafe.Pointer(nativeText))
	if C.wox_clipboard_darwin_write_text(nativeText) != 0 {
		return errors.New("clipboard: failed to publish macOS text")
	}
	return nil
}

func flushNativeImage() error {
	if C.wox_clipboard_darwin_flush() != 0 {
		return errors.New("clipboard: failed to materialize macOS image")
	}
	return nil
}
