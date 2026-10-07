//go:build linux && cgo

package clipboard

/*
#cgo pkg-config: gtk+-3.0
#include <stdlib.h>
#include "clipboard_image_linux_gtk.h"
*/
import "C"

import (
	"errors"
	"unsafe"
)

// publishNativeImage uses an explicitly supplied GTK display on its UI thread, independent of desktop backend selection.
func publishNativeImage(display uintptr, payload *clipboardImage, _ *preparedImageNative) error {
	var result C.int32_t
	if len(payload.png) > 0 {
		result = C.wox_clipboard_gtk_write_png(C.uintptr_t(display), (*C.uint8_t)(unsafe.Pointer(&payload.png[0])), C.size_t(len(payload.png)))
	} else {
		result = C.wox_clipboard_gtk_write_pixels(C.uintptr_t(display), (*C.uint8_t)(unsafe.Pointer(&payload.pixels[0])),
			C.int32_t(payload.width), C.int32_t(payload.height), C.int32_t(payload.stride))
	}
	if result != 0 {
		return errors.New("clipboard: failed to publish GTK image")
	}
	return nil
}

// publishNativeText keeps GTK selection ownership on the caller-provided display and thread.
func publishNativeText(display uintptr, text string) error {
	nativeText := C.CString(text)
	defer C.free(unsafe.Pointer(nativeText))
	if C.wox_clipboard_gtk_write_text(C.uintptr_t(display), nativeText) != 0 {
		return errors.New("clipboard: failed to publish GTK text")
	}
	return nil
}

func flushNativeImage() error {
	C.wox_clipboard_gtk_flush()
	return nil
}
