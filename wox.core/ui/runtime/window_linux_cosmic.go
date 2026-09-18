//go:build linux

package woxui

/*
#include <stdint.h>
*/
import "C"

import "wox/util"

// woxGoLinuxCosmicUsesLayerShell keeps query-driven sizing under client control.
// COSMIC retains a mapped xdg_toplevel's size; launcher result changes need a layer surface.
// Settings remain ordinary application windows and never enter the layer-shell path.
//
//export woxGoLinuxCosmicUsesLayerShell
func woxGoLinuxCosmicUsesLayerShell() C.int32_t {
	if util.IsCosmicDesktopSession() {
		return 1
	}
	return 0
}
