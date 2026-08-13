//go:build windows

package woxui

/*
#cgo CXXFLAGS: -std=c++17 -DUNICODE -D_UNICODE
#cgo LDFLAGS: -lole32 -luuid -lstdc++
#include <stdlib.h>
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

func saveFileNative(owner uintptr, options SaveFileOptions) (string, error) {
	title := C.CString(options.Title)
	defaultName := C.CString(options.DefaultFileName)
	extension := C.CString(options.Extension)
	defer C.free(unsafe.Pointer(title))
	defer C.free(unsafe.Pointer(defaultName))
	defer C.free(unsafe.Pointer(extension))
	var path *C.char
	result := C.wox_windows_save_file(C.uintptr_t(owner), title, defaultName, extension, &path)
	if result == 1 {
		return "", nil
	}
	if result < 0 {
		return "", hresultError("save file dialog", result)
	}
	if path == nil {
		return "", fmt.Errorf("save file dialog returned no path")
	}
	defer C.wox_windows_free_string(path)
	return C.GoString((*C.char)(unsafe.Pointer(path))), nil
}
