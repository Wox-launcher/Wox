//go:build windows

package woxui

/*
#cgo CXXFLAGS: -std=c++17 -DUNICODE -D_UNICODE
#cgo LDFLAGS: -lole32 -luuid -lstdc++
#include "native_windows.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func pickFileNative(owner uintptr, options FileDialogOptions) (string, error) {
	directory := C.int32_t(0)
	if options.Directory {
		directory = 1
	}
	var path *C.char
	result := C.wox_windows_pick_file(C.uintptr_t(owner), directory, &path)
	if result == 1 {
		return "", nil
	}
	if result < 0 {
		return "", hresultError("open file dialog", result)
	}
	if path == nil {
		return "", fmt.Errorf("open file dialog returned no path")
	}
	defer C.wox_windows_free_string(path)
	return C.GoString((*C.char)(unsafe.Pointer(path))), nil
}
