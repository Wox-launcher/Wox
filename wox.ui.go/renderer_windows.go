//go:build windows

package woxui

/*
#cgo CXXFLAGS: -std=c++17 -DUNICODE -D_UNICODE
#cgo LDFLAGS: -ld3d11 -ldxgi -ldcomp -ld2d1 -ldwrite -lole32 -luuid -lstdc++
#include <stdlib.h>
#include "renderer_windows.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

type nativeRenderer struct {
	handle *C.WoxRenderer
}

// newNativeRenderer attaches a DirectComposition swap chain to windowHandle.
func newNativeRenderer(windowHandle uintptr, width, height int) (*nativeRenderer, error) {
	var handle *C.WoxRenderer
	result := C.wox_renderer_create(C.uintptr_t(windowHandle), C.uint32_t(width), C.uint32_t(height), &handle)
	if result < 0 {
		return nil, hresultError("create renderer", result)
	}
	return &nativeRenderer{handle: handle}, nil
}

func (r *nativeRenderer) resize(width, height int) error {
	result := C.wox_renderer_resize(r.handle, C.uint32_t(width), C.uint32_t(height))
	if result < 0 {
		return hresultError("resize renderer", result)
	}
	return nil
}

// render replays one logical display list into the physical DirectComposition surface.
func (r *nativeRenderer) render(displayList *DisplayList, scale float32) error {
	result := C.wox_renderer_begin_frame(r.handle, C.float(scale), C.uint8_t(displayList.clearColor.R), C.uint8_t(displayList.clearColor.G), C.uint8_t(displayList.clearColor.B), C.uint8_t(displayList.clearColor.A))
	if result < 0 {
		return hresultError("begin frame", result)
	}

	for _, command := range displayList.commands {
		var commandResult C.int32_t
		switch command.kind {
		case displayCommandFillRoundedRect:
			commandResult = C.wox_renderer_fill_rounded_rect(
				r.handle,
				C.float(command.rect.X),
				C.float(command.rect.Y),
				C.float(command.rect.Width),
				C.float(command.rect.Height),
				C.float(command.radius),
				C.uint8_t(command.color.R),
				C.uint8_t(command.color.G),
				C.uint8_t(command.color.B),
				C.uint8_t(command.color.A),
			)
		case displayCommandDrawText:
			text := C.CString(command.text)
			commandResult = C.wox_renderer_draw_text(
				r.handle,
				text,
				C.float(command.rect.X),
				C.float(command.rect.Y),
				C.float(command.rect.Width),
				C.float(command.rect.Height),
				C.float(command.style.Size),
				C.uint8_t(command.style.Weight),
				C.uint8_t(command.color.R),
				C.uint8_t(command.color.G),
				C.uint8_t(command.color.B),
				C.uint8_t(command.color.A),
			)
			C.free(unsafe.Pointer(text))
		}
		if commandResult < 0 {
			_ = r.endFrame()
			return hresultError("draw frame command", commandResult)
		}
	}

	return r.endFrame()
}

func (r *nativeRenderer) endFrame() error {
	result := C.wox_renderer_end_frame(r.handle)
	if result < 0 {
		return hresultError("present frame", result)
	}
	return nil
}

func (r *nativeRenderer) destroy() {
	if r.handle != nil {
		C.wox_renderer_destroy(r.handle)
		r.handle = nil
	}
}

func hresultError(operation string, result C.int32_t) error {
	return fmt.Errorf("%s failed with HRESULT 0x%08X", operation, uint32(result))
}
