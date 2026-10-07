//go:build darwin

package woxui

/*
#cgo LDFLAGS: -framework ImageIO
#include "native_darwin.h"
*/
import "C"

import "errors"

const clipboardUsesEncodedPNG = true

// flushClipboard preserves TIFF-only paste consumers after a normal application exit.
func flushClipboard() error {
	if C.wox_darwin_flush_clipboard() != 0 {
		return errors.New("woxui: failed to materialize macOS clipboard image")
	}
	return nil
}
