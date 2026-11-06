//go:build darwin

package window

/*
#include "native_darwin.h"
*/
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"
)

type darwinOffscreenTarget struct {
	native        *C.WoxDarwinWindow
	width, height int
}

// newDarwinOffscreenTarget creates a retained physical bitmap without starting AppKit or showing a window.
func newDarwinOffscreenTarget(width, height int) (*darwinOffscreenTarget, error) {
	var native *C.WoxDarwinWindow
	if C.wox_darwin_test_create_target(C.int32_t(width), C.int32_t(height), &native) != 0 {
		return nil, fmt.Errorf("create macOS offscreen target")
	}
	return &darwinOffscreenTarget{native: native, width: width, height: height}, nil
}

func (r *darwinOffscreenTarget) destroy() {
	C.wox_darwin_test_destroy_target(r.native)
	r.native = nil
}

// render shares the production command encoder and restores the target even when command encoding fails.
func (r *darwinOffscreenTarget) render(d *DisplayList, scale float32) ([]byte, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := C.wox_darwin_autorelease_pool_push()
	defer C.wox_darwin_autorelease_pool_pop(pool)
	damage, clear := d.NativeDamage(), d.clearColor
	if result := C.wox_darwin_test_begin_frame(r.native, C.float(scale), C.float(damage.X), C.float(damage.Y), C.float(damage.Width), C.float(damage.Height), C.uint8_t(clear.R), C.uint8_t(clear.G), C.uint8_t(clear.B), C.uint8_t(clear.A)); result != 0 {
		return nil, fmt.Errorf("begin macOS offscreen frame: %d", result)
	}
	encoded := encodeDarwinCommands(r.native, d, nil, nil)
	pixels := make([]byte, r.width*r.height*4)
	result := C.wox_darwin_test_end_frame(r.native, (*C.uint8_t)(unsafe.Pointer(&pixels[0])), C.size_t(len(pixels)))
	if encoded.failed {
		return nil, fmt.Errorf("encode macOS command %d: %d", encoded.commandIndex, encoded.result)
	}
	if result != 0 {
		return nil, fmt.Errorf("finish macOS offscreen frame: %d", result)
	}
	return pixels, nil
}
