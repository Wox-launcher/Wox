//go:build windows

package woxui

/*
#cgo CXXFLAGS: -std=c++17 -DUNICODE -D_UNICODE
#cgo LDFLAGS: -static -static-libgcc -static-libstdc++ -ld3d11 -ldxgi -ldcomp -ld2d1 -ldwrite -lole32 -luuid -lstdc++
#include <stdlib.h>
#include "renderer_windows.h"
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"unsafe"

	"wox/util"
)

type nativeRenderer struct {
	handle                       *C.WoxRenderer
	windowHandle                 uintptr
	width                        int
	height                       int
	enableEmbeddedSurfaceOverlay bool
	fontFamily                   string
}

const (
	d2dErrRecreateTarget       = uint32(0x8899000C)
	dxgiErrDeviceRemoved       = uint32(0x887A0005)
	dxgiErrDeviceHung          = uint32(0x887A0006)
	dxgiErrDeviceReset         = uint32(0x887A0007)
	dxgiErrDriverInternalError = uint32(0x887A0020)
)

type windowsRendererError struct {
	operation string
	code      uint32
}

func (e *windowsRendererError) Error() string {
	return fmt.Sprintf("%s failed with HRESULT 0x%08X", e.operation, e.code)
}

func traceNativeCall(format string, args ...any) {
	if util.IsDev() {
		util.GetLogger().Debug(context.Background(), fmt.Sprintf("native crash trace: "+format, args...))
	}
}

// newNativeRenderer attaches a DirectComposition swap chain to windowHandle.
func newNativeRenderer(windowHandle uintptr, width, height int, enableEmbeddedSurfaceOverlay bool) (*nativeRenderer, error) {
	var handle *C.WoxRenderer
	traceNativeCall("renderer create enter hwnd=%#x size=%dx%d", windowHandle, width, height)
	enableOverlay := C.int32_t(0)
	if enableEmbeddedSurfaceOverlay {
		enableOverlay = 1
	}
	result := C.wox_renderer_create(C.uintptr_t(windowHandle), C.uint32_t(width), C.uint32_t(height), enableOverlay, &handle)
	traceNativeCall("renderer create exit hwnd=%#x handle=%p result=%d", windowHandle, handle, result)
	if result < 0 {
		return nil, hresultError("create renderer", result)
	}
	return &nativeRenderer{
		handle:                       handle,
		windowHandle:                 windowHandle,
		width:                        width,
		height:                       height,
		enableEmbeddedSurfaceOverlay: enableEmbeddedSurfaceOverlay,
	}, nil
}

func (r *nativeRenderer) resize(width, height int) error {
	if width > 0 && height > 0 {
		// Preserve the requested size when ResizeBuffers reports device loss so recreation uses the new bounds.
		r.width = width
		r.height = height
	}
	traceNativeCall("renderer resize enter handle=%p size=%dx%d", r.handle, width, height)
	result := C.wox_renderer_resize(r.handle, C.uint32_t(width), C.uint32_t(height))
	traceNativeCall("renderer resize exit handle=%p result=%d", r.handle, result)
	if result < 0 {
		return hresultError("resize renderer", result)
	}
	return nil
}

func (r *nativeRenderer) setFontFamily(family string) error {
	nativeFamily := C.CString(family)
	defer C.free(unsafe.Pointer(nativeFamily))
	traceNativeCall("renderer font enter handle=%p family=%q", r.handle, family)
	result := C.wox_renderer_set_font_family(r.handle, nativeFamily)
	traceNativeCall("renderer font exit handle=%p result=%d", r.handle, result)
	if result < 0 {
		return hresultError("set font family", result)
	}
	r.fontFamily = family
	return nil
}

// recreate replaces all resources tied to a lost Direct3D device while keeping the Go renderer identity stable.
func (r *nativeRenderer) recreate() error {
	fontFamily := r.fontFamily
	r.destroy()
	replacement, err := newNativeRenderer(r.windowHandle, r.width, r.height, r.enableEmbeddedSurfaceOverlay)
	if err != nil {
		return err
	}
	if fontFamily != "" {
		if err := replacement.setFontFamily(fontFamily); err != nil {
			replacement.destroy()
			return err
		}
	}
	*r = *replacement
	return nil
}

// trim asks DXGI to discard driver-managed allocations that are no longer needed while hidden.
func (r *nativeRenderer) trim() error {
	traceNativeCall("renderer trim enter handle=%p", r.handle)
	result := C.wox_renderer_trim(r.handle)
	traceNativeCall("renderer trim exit handle=%p result=%d", r.handle, result)
	if result < 0 {
		return hresultError("trim renderer", result)
	}
	return nil
}

// clearImageCache releases transient icon textures and Direct2D's idle resource cache while hidden.
func (r *nativeRenderer) clearImageCache() error {
	traceNativeCall("renderer clear image cache enter handle=%p", r.handle)
	result := C.wox_renderer_clear_image_cache(r.handle)
	traceNativeCall("renderer clear image cache exit handle=%p result=%d", r.handle, result)
	if result < 0 {
		return hresultError("clear renderer image cache", result)
	}
	return nil
}

// measureText uses DirectWrite without opening a draw transaction.
func (r *nativeRenderer) measureText(text string, style TextStyle) (TextMetrics, error) {
	nativeText := C.CString(text)
	defer C.free(unsafe.Pointer(nativeText))
	var width C.float
	var height C.float
	var baseline C.float
	traceNativeCall("renderer measure enter handle=%p textLen=%d size=%.2f weight=%d", r.handle, len(text), style.Size, style.Weight)
	result := C.wox_renderer_measure_text(r.handle, nativeText, C.float(style.Size), C.uint8_t(style.Weight), &width, &height, &baseline)
	traceNativeCall("renderer measure exit handle=%p result=%d", r.handle, result)
	if result < 0 {
		return TextMetrics{}, hresultError("measure text", result)
	}
	return TextMetrics{Size: Size{Width: float32(width), Height: float32(height)}, Baseline: float32(baseline)}, nil
}

// render replays one logical display list into the physical DirectComposition surface.
func (r *nativeRenderer) render(displayList *DisplayList, scale float32) error {
	damage := displayList.NativeDamage()
	traceNativeCall("renderer frame begin frameId=%d handle=%p commands=%d scale=%.2f damage=%+v", displayList.FrameMetricsID(), r.handle, len(displayList.commands), scale, damage)
	result := C.wox_renderer_begin_frame(r.handle, C.float(scale), C.float(damage.X), C.float(damage.Y), C.float(damage.Width), C.float(damage.Height), C.uint8_t(displayList.clearColor.R), C.uint8_t(displayList.clearColor.G), C.uint8_t(displayList.clearColor.B), C.uint8_t(displayList.clearColor.A))
	traceNativeCall("renderer begin_frame exit frameId=%d handle=%p result=%d", displayList.FrameMetricsID(), r.handle, result)
	if result < 0 {
		return hresultError("begin frame", result)
	}

	for index, command := range displayList.commands {
		traceNativeCall("renderer command enter frameId=%d handle=%p index=%d kind=%d", displayList.FrameMetricsID(), r.handle, index, command.kind)
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
		case displayCommandFillConvexPolygon:
			commandResult = C.wox_renderer_fill_convex_polygon(
				r.handle,
				(*C.float)(unsafe.Pointer(&command.points[0])),
				C.int32_t(len(command.points)),
				C.uint8_t(command.color.R),
				C.uint8_t(command.color.G),
				C.uint8_t(command.color.B),
				C.uint8_t(command.color.A),
			)
		case displayCommandStrokeRoundedRect:
			commandResult = C.wox_renderer_stroke_rounded_rect(
				r.handle,
				C.float(command.rect.X),
				C.float(command.rect.Y),
				C.float(command.rect.Width),
				C.float(command.rect.Height),
				C.float(command.radius),
				C.float(command.stroke),
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
		case displayCommandDrawImage:
			commandResult = C.wox_renderer_draw_image(
				r.handle,
				C.uint64_t(command.image.id),
				(*C.uint8_t)(unsafe.Pointer(&command.image.pixels[0])),
				C.uint32_t(command.image.Width),
				C.uint32_t(command.image.Height),
				C.uint32_t(command.image.Width*4),
				C.float(command.rect.X),
				C.float(command.rect.Y),
				C.float(command.rect.Width),
				C.float(command.rect.Height),
				C.float(command.rotation),
				C.float(command.radius),
			)
		case displayCommandBeginEmbeddedSurfaceOverlay:
			commandResult = C.wox_renderer_begin_embedded_surface_overlay(r.handle)
		case displayCommandSetClipRect:
			commandResult = C.wox_renderer_set_clip_rect(r.handle, C.float(command.rect.X), C.float(command.rect.Y), C.float(command.rect.Width), C.float(command.rect.Height))
		case displayCommandClearClip:
			commandResult = C.wox_renderer_clear_clip(r.handle)
		}
		if commandResult < 0 {
			_ = r.endFrame()
			return hresultError("draw frame command", commandResult)
		}
	}

	traceNativeCall("renderer end_frame enter frameId=%d handle=%p", displayList.FrameMetricsID(), r.handle)
	err := r.endFrame()
	traceNativeCall("renderer frame exit frameId=%d handle=%p err=%v", displayList.FrameMetricsID(), r.handle, err)
	return err
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
		traceNativeCall("renderer destroy enter handle=%p", r.handle)
		C.wox_renderer_destroy(r.handle)
		traceNativeCall("renderer destroy exit handle=%p", r.handle)
		r.handle = nil
	}
}

func hresultError(operation string, result C.int32_t) error {
	return &windowsRendererError{operation: operation, code: uint32(result)}
}

func isRecoverableRendererError(err error) bool {
	var rendererErr *windowsRendererError
	if !errors.As(err, &rendererErr) {
		return false
	}
	switch rendererErr.code {
	case d2dErrRecreateTarget, dxgiErrDeviceRemoved, dxgiErrDeviceHung, dxgiErrDeviceReset, dxgiErrDriverInternalError:
		return true
	default:
		return false
	}
}
