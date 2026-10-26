//go:build linux

package window

import "strings"

// isLinuxWlrootsWindowSession retains the allowlist of compositors sharing this policy.
func isLinuxWlrootsWindowSession(desktop string) bool {
	for _, compositor := range []string{"sway", "wayfire", "river", "wlroots"} {
		if strings.Contains(desktop, compositor) {
			return true
		}
	}
	return false
}

func linuxWlrootsWindowPolicy() linuxWindowPolicy {
	return linuxWindowPolicy{name: "wlroots", layerShell: true}
}
