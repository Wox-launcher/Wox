//go:build linux

package woxui

import "errors"

/*
#include "native_linux.h"
*/
import "C"

// setWindowChrome turns compositor blur off so a self-drawn outline is not covered.
// radius is the present clip. Linux used to ignore it, so full-width children could
// paint into the transparent corner cutouts that Windows and macOS clip natively.
func (w *platformWindow) setWindowChrome(custom bool, radius float32) error {
	native, err := w.openNative()
	if err != nil {
		return err
	}
	enabled := C.int32_t(0)
	if custom {
		enabled = 1
	}
	if C.wox_linux_window_set_window_chrome(native, enabled, C.float(radius)) != 0 {
		return errors.New("woxui: failed to update Linux window chrome")
	}
	return nil
}
