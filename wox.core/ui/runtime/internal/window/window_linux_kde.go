//go:build linux

package window

import "strings"

// linuxKDEWindowPolicy keeps utility windows off Plasma's taskbar. GTK's skip
// hints only affect X11; KWin Wayland needs a layer surface instead of xdg_toplevel.
func linuxKDEWindowPolicy() linuxWindowPolicy {
	// KWin interprets the layer namespace as a window type. Its default normal
	// type adds open/close animations and retains hidden launcher pixels in captures.
	return linuxWindowPolicy{name: "kde", layerShell: true, layerNamespace: "utility"}
}

func isLinuxKDEWindowSession(desktop string) bool {
	return strings.Contains(desktop, "kde") || strings.Contains(desktop, "plasma")
}
