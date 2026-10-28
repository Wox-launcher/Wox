//go:build linux

package window

/*
#include "native_linux.h"
*/
import "C"

import "errors"

// SetExclusiveKeyboard switches a Linux layer-shell window between exclusive and
// on-demand keyboard focus without hiding it. Other Linux windows and
// nonactivating surfaces retain their existing focus behavior.
func (w *Window) SetExclusiveKeyboard(enabled bool) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	native, err := w.native.openNative()
	if err != nil {
		return err
	}
	nativeEnabled := C.int32_t(0)
	if enabled {
		nativeEnabled = 1
	}
	if C.wox_linux_window_set_keyboard_exclusive(native, nativeEnabled) != 0 {
		return errors.New("woxui: failed to update Linux keyboard exclusivity")
	}
	return nil
}
