//go:build windows

package woxui

/*
#cgo CXXFLAGS: -std=c++17 -DUNICODE -D_UNICODE
#cgo LDFLAGS: -ld3d11 -ldxgi
#include "desktop_duplication_windows.h"
*/
import "C"

import (
	"errors"
	"image"
	"unsafe"
)

type windowsDXGIRectCapturer struct {
	handle *C.WoxDXGIRectCapturer
	width  int
	height int
}

// newWindowsDXGIRectCapturer captures one desktop rectangle without hiding the hardware cursor.
func newWindowsDXGIRectCapturer(bounds image.Rectangle) (*windowsDXGIRectCapturer, error) {
	var handle *C.WoxDXGIRectCapturer
	result := C.wox_dxgi_rect_capturer_create(C.int32_t(bounds.Min.X), C.int32_t(bounds.Min.Y), C.int32_t(bounds.Dx()), C.int32_t(bounds.Dy()), &handle)
	if result < 0 || handle == nil {
		return nil, errors.New("DXGI desktop duplication is unavailable for this rectangle")
	}
	return &windowsDXGIRectCapturer{handle: handle, width: bounds.Dx(), height: bounds.Dy()}, nil
}

func (capturer *windowsDXGIRectCapturer) Capture() (*image.RGBA, error) {
	if capturer == nil || capturer.handle == nil {
		return nil, errors.New("recording capture surface is closed")
	}
	output := image.NewRGBA(image.Rect(0, 0, capturer.width, capturer.height))
	result := C.wox_dxgi_rect_capturer_capture(capturer.handle, (*C.uint8_t)(unsafe.Pointer(&output.Pix[0])), C.int32_t(output.Stride))
	if result < 0 {
		return nil, errors.New("failed to duplicate Windows desktop pixels")
	}
	return output, nil
}

func (capturer *windowsDXGIRectCapturer) Close() {
	if capturer == nil || capturer.handle == nil {
		return
	}
	C.wox_dxgi_rect_capturer_destroy(capturer.handle)
	capturer.handle = nil
}
