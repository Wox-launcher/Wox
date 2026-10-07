//go:build darwin

package browser

/*
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
#include "browser_external_darwin.h"
*/
import "C"

import (
	"errors"
	"unsafe"
)

// openExternalURL delegates default-application routing to Launch Services on the caller's AppKit thread.
func openExternalURL(rawURL string, _ uintptr) error {
	nativeURL := C.CString(rawURL)
	defer C.free(unsafe.Pointer(nativeURL))
	if C.wox_browser_open_external_url(nativeURL) != 0 {
		return errors.New("browser: failed to open external URL on macOS")
	}
	return nil
}
