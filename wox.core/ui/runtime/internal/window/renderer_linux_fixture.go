//go:build linux

package window

/*
#include <stdlib.h>
#include "native_linux.h"
*/
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"
)

// linuxOffscreenTarget retains a framebuffer and releases the current GL context between calls.
type linuxOffscreenTarget struct {
	native        *C.WoxLinuxWindow
	width, height int
}

// newLinuxOffscreenTarget reports a missing display separately from rendering failures.
func newLinuxOffscreenTarget(width, height int) (*linuxOffscreenTarget, bool, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var native *C.WoxLinuxWindow
	result := C.wox_linux_test_create_target(C.int32_t(width), C.int32_t(height), &native)
	if result == 1 {
		return nil, true, nil
	}
	if result != 0 {
		return nil, false, fmt.Errorf("create Linux offscreen target: %d", result)
	}
	return &linuxOffscreenTarget{native: native, width: width, height: height}, false, nil
}

func (r *linuxOffscreenTarget) destroy() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	C.wox_linux_test_destroy_target(r.native)
	r.native = nil
}

// render reuses the production frame setup and encoder, including failure cleanup.
func (r *linuxOffscreenTarget) render(d *DisplayList, scale float32) ([]byte, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	damage, clear := d.NativeDamage(), d.clearColor
	if result := C.wox_linux_test_begin_frame(r.native, C.float(scale), C.float(damage.X), C.float(damage.Y), C.float(damage.Width), C.float(damage.Height), C.uint8_t(clear.R), C.uint8_t(clear.G), C.uint8_t(clear.B), C.uint8_t(clear.A)); result != 0 {
		return nil, fmt.Errorf("begin Linux offscreen frame: %d", result)
	}
	family, monospace := C.CString("sans"), C.CString("monospace")
	defer C.free(unsafe.Pointer(family))
	defer C.free(unsafe.Pointer(monospace))
	encoded := encodeLinuxCommands(r.native, d, family, monospace)
	pixels := make([]byte, r.width*r.height*4)
	result := C.wox_linux_test_end_frame(r.native, (*C.uint8_t)(unsafe.Pointer(&pixels[0])), C.size_t(len(pixels)))
	if encoded != 0 {
		return nil, fmt.Errorf("encode Linux offscreen frame: %d", encoded)
	}
	if result != 0 {
		return nil, fmt.Errorf("finish Linux offscreen frame: %d", result)
	}
	return pixels, nil
}
