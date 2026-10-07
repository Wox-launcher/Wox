//go:build windows && cgo

package imageencode

/*
#include "png_rows_windows.h"
*/
import "C"

import "unsafe"

// filterPNGRow keeps large pixel loops in native code even when Go's debug build disables optimization.
func filterPNGRow(destination, source []byte, premultiplied bool, bytesPerPixel int) {
	var alpha C.int
	if premultiplied {
		alpha = 1
	}
	C.wox_filter_png_row((*C.uint8_t)(unsafe.Pointer(&destination[0])), (*C.uint8_t)(unsafe.Pointer(&source[0])), C.size_t(len(destination)), alpha, C.size_t(bytesPerPixel))
}
