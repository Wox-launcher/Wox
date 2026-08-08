//go:build windows

package woxui

/*
#cgo CXXFLAGS: -std=c++17 -DUNICODE -D_UNICODE
#cgo LDFLAGS: -lshell32 -lole32 -luuid -lstdc++
#include <stdint.h>
#include <stdlib.h>
#include "native_windows.h"
*/
import "C"

import (
	"errors"
	"unsafe"
)

// startFileDrag runs the native Windows OLE drag loop on the window thread.
func (w *platformWindow) startFileDrag(paths []string) (FileDragStatus, error) {
	if len(paths) == 0 {
		return FileDragStatusCancel, errors.New("file drag has no paths")
	}
	values := make([]*C.char, 0, len(paths))
	for _, path := range paths {
		values = append(values, C.CString(path))
	}
	defer func() {
		for _, value := range values {
			C.free(unsafe.Pointer(value))
		}
	}()
	result := C.wox_windows_start_file_drag(
		C.uintptr_t(w.hwnd),
		(**C.char)(unsafe.Pointer(&values[0])),
		C.int32_t(len(values)),
	)
	switch result {
	case 0:
		return FileDragStatusSuccess, nil
	case 1:
		return FileDragStatusCancel, nil
	case 2:
		return FileDragStatusCancelInSource, nil
	default:
		return FileDragStatusCancel, errors.New("native Windows file drag failed")
	}
}
