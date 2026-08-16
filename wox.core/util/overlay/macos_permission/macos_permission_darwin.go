//go:build darwin

package macospermission

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework CoreGraphics
#include "macos_permission_darwin.h"
#include <stdlib.h>
*/
import "C"

import (
	"unsafe"

	"wox/util/mainthread"
)

func openPermissionSettings(anchor string) {
	value := C.CString(anchor)
	defer C.free(unsafe.Pointer(value))
	mainthread.Call(func() { C.woxMacOSPermissionOpenSettings(value) })
}

func permissionSettingsWindow() (Rect, Rect, bool) {
	var frame C.WoxMacOSPermissionRect
	var workArea C.WoxMacOSPermissionRect
	status := C.woxMacOSPermissionSettingsWindow(&frame, &workArea)
	return rectFromNative(frame), rectFromNative(workArea), status != 0
}

func rectFromNative(rect C.WoxMacOSPermissionRect) Rect {
	return Rect{X: float64(rect.x), Y: float64(rect.y), Width: float64(rect.width), Height: float64(rect.height)}
}

func permissionApplicationPath() string {
	value := C.woxMacOSPermissionApplicationPath()
	if value == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(value))
	return C.GoString(value)
}
