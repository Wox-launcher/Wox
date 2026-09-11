//go:build darwin

package woxui

/*
#include "native_darwin.h"
*/
import "C"

import (
	"slices"
	"unsafe"
)

func nativeFloatingMaterialMode() floatingMaterialMode {
	return floatingMaterialOverlay
}

// applyFloatingMaterials hands the materials declared by the frame being encoded
// to the native window, which reuses its material views by index and hides the rest.
// It runs before the frame is presented so the material and the surface pixels it
// backs land in the same AppKit transaction. Unchanged frames skip the main-thread
// round trip entirely.
func (w *platformWindow) applyFloatingMaterials(native *C.WoxDarwinWindow, materials []floatingMaterial) C.int32_t {
	w.mu.Lock()
	unchanged := slices.Equal(materials, w.floatingMaterials)
	w.mu.Unlock()
	if unchanged {
		return 0
	}
	var result C.int32_t
	if len(materials) == 0 {
		result = C.wox_darwin_window_set_floating_materials(native, nil, 0)
	} else {
		items := make([]C.WoxDarwinFloatingMaterial, len(materials))
		for index, material := range materials {
			items[index] = C.WoxDarwinFloatingMaterial{
				x: C.float(material.bounds.X), y: C.float(material.bounds.Y), width: C.float(material.bounds.Width), height: C.float(material.bounds.Height),
				corner_radius: C.float(material.radius),
				tint_red:      C.uint8_t(material.tint.R), tint_green: C.uint8_t(material.tint.G), tint_blue: C.uint8_t(material.tint.B), tint_alpha: C.uint8_t(material.tint.A),
				edge_red: C.uint8_t(material.edge.R), edge_green: C.uint8_t(material.edge.G), edge_blue: C.uint8_t(material.edge.B), edge_alpha: C.uint8_t(material.edge.A),
			}
		}
		result = C.wox_darwin_window_set_floating_materials(native, (*C.WoxDarwinFloatingMaterial)(unsafe.Pointer(&items[0])), C.int32_t(len(items)))
	}
	if result == 0 {
		w.mu.Lock()
		w.floatingMaterials = append(w.floatingMaterials[:0], materials...)
		w.mu.Unlock()
	}
	return result
}
