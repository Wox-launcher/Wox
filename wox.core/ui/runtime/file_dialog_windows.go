//go:build windows

package woxui

/*
#cgo CXXFLAGS: -std=c++17 -DUNICODE -D_UNICODE
#cgo LDFLAGS: -lole32 -luuid -lstdc++
#include <stdlib.h>
#include "native_windows.h"
int32_t wox_windows_navigate_file_dialog(uintptr_t window, const wchar_t *path);
*/
import "C"

import (
	"fmt"
	"sync/atomic"
	"syscall"
	"unsafe"
)

var nativeFileDialogListener atomic.Value // func(uintptr, bool)

// SetNativeFileDialogListener observes Windows picker lifetime. The listener must
// return promptly: notifications run inside the native modal UI callback.
func SetNativeFileDialogListener(listener func(windowID uintptr, opened bool)) {
	nativeFileDialogListener.Store(listener)
}

//export woxGoNativeFileDialogChanged
func woxGoNativeFileDialogChanged(windowID C.uintptr_t, opened C.int32_t) {
	if listener, ok := nativeFileDialogListener.Load().(func(uintptr, bool)); ok && listener != nil {
		listener(uintptr(windowID), opened != 0)
	}
}

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

// NavigateNativeFileDialog changes the folder on the owning COM/UI thread.
// A stale HWND fails without falling back to a confirmation keystroke.
func NavigateNativeFileDialog(windowID uintptr, path string) error {
	nativePath, err := syscall.UTF16FromString(path)
	if err != nil {
		return err
	}
	var result C.int32_t
	if err := Call(func() {
		result = C.wox_windows_navigate_file_dialog(C.uintptr_t(windowID), (*C.wchar_t)(unsafe.Pointer(&nativePath[0])))
	}); err != nil {
		return err
	}
	if result < 0 {
		return hresultError("navigate file dialog", result)
	}
	return nil
}
