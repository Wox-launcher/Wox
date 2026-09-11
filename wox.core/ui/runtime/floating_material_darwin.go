//go:build darwin

package woxui

/*
#include "native_darwin.h"
*/
import "C"

import "errors"

func nativeFloatingMaterialAvailable() bool {
	return true
}

func (w *platformWindow) showFloatingMaterial(bounds Rect, style FloatingMaterialStyle) error {
	native, err := w.openNative()
	if err != nil {
		return err
	}
	tint, edge := style.Tint, style.Edge
	if C.wox_darwin_window_show_floating_material(native, C.float(bounds.X), C.float(bounds.Y), C.float(bounds.Width), C.float(bounds.Height), C.float(style.CornerRadius),
		C.uint8_t(tint.R), C.uint8_t(tint.G), C.uint8_t(tint.B), C.uint8_t(tint.A), C.uint8_t(edge.R), C.uint8_t(edge.G), C.uint8_t(edge.B), C.uint8_t(edge.A)) != 0 {
		return errors.New("woxui: failed to show macOS floating material")
	}
	return nil
}

func (w *platformWindow) hideFloatingMaterial() error {
	native, err := w.openNative()
	if err != nil {
		return err
	}
	if C.wox_darwin_window_hide_floating_material(native) != 0 {
		return errors.New("woxui: failed to hide macOS floating material")
	}
	return nil
}
