//go:build linux

package woxui

import "errors"

/*
#include "native_linux.h"
#include "native_linux_background_effect.h"
*/
import "C"

// setWindowChrome keeps compositor blur within the authored rounded outline.
// radius is the present clip. Linux used to ignore it, so full-width children could
// paint into the transparent corner cutouts that Windows and macOS clip natively.
// Utility windows request per-pixel alpha at create so this clip can punch the
// framebuffer after realize; GtkGLArea has_alpha cannot be enabled later.
func (w *platformWindow) setWindowChrome(custom bool, radius float32, backgroundBlur ...bool) error {
	native, err := w.openNative()
	if err != nil {
		return err
	}
	enabled := C.int32_t(0)
	if custom {
		enabled = 1
	}
	blur := C.int32_t(1)
	if len(backgroundBlur) > 0 && !backgroundBlur[0] {
		blur = 0
	}
	if C.wox_linux_window_set_window_chrome(native, enabled, C.float(radius), blur) != 0 {
		return errors.New("woxui: failed to update Linux window chrome")
	}
	return nil
}

// testLinuxBlurRegion returns the protocol rectangles without opening a native window.
func testLinuxBlurRegion(width, height int, radius float32) [][4]int {
	region := C.wox_linux_background_blur_region(C.int(width), C.int(height), C.float(radius))
	defer C.cairo_region_destroy(region)
	var rects [][4]int
	for i := C.int(0); i < C.cairo_region_num_rectangles(region); i++ {
		var rect C.cairo_rectangle_int_t
		C.cairo_region_get_rectangle(region, i, &rect)
		rects = append(rects, [4]int{int(rect.x), int(rect.y), int(rect.width), int(rect.height)})
	}
	return rects
}
