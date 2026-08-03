package lottie

/*
#cgo CXXFLAGS: -std=c++14 -DTVG_STATIC -I${SRCDIR}/thorvg/src/bindings/capi
#cgo darwin,arm64 LDFLAGS: ${SRCDIR}/thorvg/build/darwin-arm64/src/libthorvg-1.a -lc++
#cgo darwin,amd64 LDFLAGS: ${SRCDIR}/thorvg/build/darwin-amd64/src/libthorvg-1.a -lc++
#cgo linux,amd64 LDFLAGS: ${SRCDIR}/thorvg/build/linux-amd64/src/libthorvg-1.a -lstdc++ -lm -lpthread
#cgo windows,amd64 LDFLAGS: ${SRCDIR}/thorvg/build/windows-amd64/src/libthorvg-1.a -lstdc++
#include <stdlib.h>
#include "wox_lottie.h"
*/
import "C"

import (
	"fmt"
	"image"
	"unsafe"
)

const (
	maxJSONSize  = 4 << 20
	maxDimension = 2048
)

// Animation owns one ThorVG Lottie document and its reusable software canvas.
type Animation struct {
	handle *C.WoxLottie
	width  int
	height int
}

// New parses a Lottie JSON document into a fixed-size software canvas.
func New(data string, width, height int) (*Animation, error) {
	if data == "" || len(data) > maxJSONSize {
		return nil, fmt.Errorf("lottie JSON size is invalid: %d", len(data))
	}
	if width <= 0 || height <= 0 || width > maxDimension || height > maxDimension {
		return nil, fmt.Errorf("lottie dimensions are invalid: %dx%d", width, height)
	}
	json := C.CBytes([]byte(data))
	defer C.free(json)
	var errorCode C.int
	handle := C.wox_lottie_create((*C.char)(json), C.size_t(len(data)), C.uint32_t(width), C.uint32_t(height), &errorCode)
	if handle == nil {
		return nil, fmt.Errorf("create ThorVG animation: stage %d", int(errorCode))
	}
	return &Animation{handle: handle, width: width, height: height}, nil
}

// Duration returns the source composition duration in seconds.
func (a *Animation) Duration() float64 {
	if a == nil || a.handle == nil {
		return 0
	}
	return float64(C.wox_lottie_duration(a.handle))
}

// Render rasterizes one normalized animation position into premultiplied RGBA pixels.
func (a *Animation) Render(progress float64) (*image.RGBA, error) {
	if a == nil || a.handle == nil {
		return nil, fmt.Errorf("lottie animation is closed")
	}
	rgba := image.NewRGBA(image.Rect(0, 0, a.width, a.height))
	result := C.wox_lottie_render(a.handle, C.float(progress), (*C.uint8_t)(unsafe.Pointer(&rgba.Pix[0])), C.size_t(len(rgba.Pix)))
	if result != 0 {
		return nil, fmt.Errorf("render ThorVG animation: stage %d", int(result))
	}
	return rgba, nil
}

// Close releases the native animation and canvas.
func (a *Animation) Close() {
	if a == nil || a.handle == nil {
		return
	}
	C.wox_lottie_destroy(a.handle)
	a.handle = nil
}
