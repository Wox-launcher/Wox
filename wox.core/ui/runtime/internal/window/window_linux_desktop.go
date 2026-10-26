//go:build linux

package window

/*
#include <stdint.h>
*/
import "C"

import (
	"os"
	"strings"

	"wox/util"
)

// linuxWindowPolicy describes desktop policy, not protocol availability. GTK still
// verifies the actual display and layer-shell support before creating a surface.
type linuxWindowPolicy struct {
	name       string
	layerShell bool
}

// selectLinuxWindowPolicy keeps X11 independent of Wayland environment variables.
// The native display is authoritative, including GDK_BACKEND=x11 in a Wayland session.
func selectLinuxWindowPolicy(wayland bool) linuxWindowPolicy {
	if !wayland {
		return linuxX11WindowPolicy()
	}
	desktop := linuxWindowDesktop()
	switch {
	case util.IsCosmicDesktopSession():
		return linuxCosmicWindowPolicy()
	case isLinuxKDEWindowSession(desktop):
		return linuxKDEWindowPolicy()
	case isLinuxHyprlandWindowSession(desktop):
		return linuxHyprlandWindowPolicy()
	case isLinuxWlrootsWindowSession(desktop):
		return linuxWlrootsWindowPolicy()
	case util.IsGnomeDesktopSession():
		return linuxGnomeWindowPolicy()
	default:
		return linuxWindowPolicy{name: "wayland"}
	}
}

// woxGoLinuxWindowUsesLayerShell is the sole desktop-policy bridge used by GTK.
// Application windows are excluded by the native caller before this decision.
//
//export woxGoLinuxWindowUsesLayerShell
func woxGoLinuxWindowUsesLayerShell(wayland C.int32_t) C.int32_t {
	if selectLinuxWindowPolicy(wayland != 0).layerShell {
		return 1
	}
	return 0
}

// linuxWindowDesktop preserves the native backend's first-nonempty lookup order.
// Desktop adapters retain legacy aliases such as plasmawayland and sway:wlroots.
func linuxWindowDesktop() string {
	for _, name := range []string{"XDG_CURRENT_DESKTOP", "XDG_SESSION_DESKTOP", "DESKTOP_SESSION"} {
		if desktop := os.Getenv(name); desktop != "" {
			return strings.ToLower(desktop)
		}
	}
	return ""
}
