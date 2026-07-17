//go:build darwin

package wallpaper

/*
#cgo CFLAGS: -fblocks -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>

char *woxGetSystemWallpaperPath(void);
*/
import "C"

import (
	"errors"
	"unsafe"
)

func getSystemWallpaperPath() (string, error) {
	value := C.woxGetSystemWallpaperPath()
	if value == nil {
		return "", errors.New("desktop wallpaper is unavailable")
	}
	defer C.free(unsafe.Pointer(value))
	path := C.GoString(value)
	if path == "" {
		return "", errors.New("desktop wallpaper is unavailable")
	}
	return path, nil
}
