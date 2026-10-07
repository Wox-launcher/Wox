//go:build linux

package woxui

/*
#include "native_linux.h"
*/
import "C"

import "errors"

const clipboardUsesEncodedPNG = true

// flushClipboard lets the clipboard manager preserve compatibility formats before normal exit.
func flushClipboard() error {
	if C.wox_linux_flush_clipboard() != 0 {
		return errors.New("woxui: failed to store Linux clipboard image")
	}
	return nil
}
