//go:build windows

package woxui

/*
#cgo CXXFLAGS: -std=c++17 -DUNICODE -D_UNICODE
#cgo LDFLAGS: -lshlwapi -lole32 -luuid -lstdc++
#include <stdint.h>
#include <stdlib.h>
#include "native_windows.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

type windowsFilePreview struct {
	handle *C.WoxWindowsFilePreview
	path   string
}

// newWindowsFilePreview creates the shell preview handler on the window UI thread.
func newWindowsFilePreview(owner uintptr, path string, bounds Rect, scale float32) (*windowsFilePreview, error) {
	if scale <= 0 {
		scale = 1
	}
	filePath := C.CString(path)
	defer C.free(unsafe.Pointer(filePath))
	var handle *C.WoxWindowsFilePreview
	result := C.wox_windows_file_preview_create(
		C.uintptr_t(owner), filePath,
		C.int32_t(bounds.X*scale+0.5), C.int32_t(bounds.Y*scale+0.5),
		C.int32_t(bounds.Width*scale+0.5), C.int32_t(bounds.Height*scale+0.5), &handle,
	)
	if result < 0 {
		return nil, filePreviewHRESULT("create native file preview", result)
	}
	return &windowsFilePreview{handle: handle, path: path}, nil
}

func (p *windowsFilePreview) show(bounds Rect, scale float32) error {
	if p == nil || p.handle == nil {
		return errors.New("native file preview is unavailable")
	}
	if scale <= 0 {
		scale = 1
	}
	result := C.wox_windows_file_preview_show(
		p.handle,
		C.int32_t(bounds.X*scale+0.5), C.int32_t(bounds.Y*scale+0.5),
		C.int32_t(bounds.Width*scale+0.5), C.int32_t(bounds.Height*scale+0.5),
	)
	if result < 0 {
		return filePreviewHRESULT("show native file preview", result)
	}
	return nil
}

func (p *windowsFilePreview) hide() error {
	if p == nil || p.handle == nil {
		return nil
	}
	result := C.wox_windows_file_preview_hide(p.handle)
	if result < 0 {
		return filePreviewHRESULT("hide native file preview", result)
	}
	return nil
}

func (p *windowsFilePreview) destroy() {
	if p != nil && p.handle != nil {
		C.wox_windows_file_preview_destroy(p.handle)
		p.handle = nil
	}
}

func filePreviewHRESULT(operation string, result C.int32_t) error {
	return fmt.Errorf("woxui: %s failed with HRESULT 0x%08X", operation, uint32(result))
}
