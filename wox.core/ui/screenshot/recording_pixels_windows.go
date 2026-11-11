//go:build windows && cgo

package screenshot

/*
#cgo CFLAGS: -O3
#include "recording_pixels_windows.h"
*/
import "C"

import (
	"image"
	"unsafe"
)

func writeRecordingI420(frame *image.RGBA, output []byte) {
	C.wox_recording_bgra_to_i420((*C.uint8_t)(unsafe.Pointer(&frame.Pix[0])), C.int(frame.Stride), C.int(frame.Rect.Dx()), C.int(frame.Rect.Dy()), (*C.uint8_t)(unsafe.Pointer(&output[0])))
}
