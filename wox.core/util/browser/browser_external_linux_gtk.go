//go:build linux && cgo

package browser

/*
#cgo pkg-config: gtk+-3.0
#include <stdlib.h>
#include "browser_external_linux_gtk.h"
*/
import "C"

import (
	"errors"
	"unsafe"
)

// openExternalURL keeps GTK launch context while accepting only a generic native owner capability.
func openExternalURL(rawURL string, owner uintptr) error {
	nativeURL := C.CString(rawURL)
	defer C.free(unsafe.Pointer(nativeURL))
	if C.wox_browser_open_external_url(nativeURL, C.uintptr_t(owner)) != 0 {
		return errors.New("browser: failed to open external URL on Linux")
	}
	return nil
}
