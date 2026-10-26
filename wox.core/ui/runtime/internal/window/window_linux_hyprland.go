//go:build linux

package window

import "strings"

// linuxHyprlandWindowPolicy uses layer-shell for utility placement and focus.
func linuxHyprlandWindowPolicy() linuxWindowPolicy {
	return linuxWindowPolicy{name: "hyprland", layerShell: true}
}

func isLinuxHyprlandWindowSession(desktop string) bool {
	return strings.Contains(desktop, "hyprland")
}
