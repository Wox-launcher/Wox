//go:build linux

package window

import "strings"

// linuxKDEWindowPolicy keeps utility windows off Plasma's taskbar. GTK's skip
// hints only affect X11; KWin Wayland needs a layer surface instead of xdg_toplevel.
func linuxKDEWindowPolicy() linuxWindowPolicy {
	return linuxWindowPolicy{name: "kde", layerShell: true}
}

func isLinuxKDEWindowSession(desktop string) bool {
	return strings.Contains(desktop, "kde") || strings.Contains(desktop, "plasma")
}
