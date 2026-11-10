//go:build linux && cgo

package screen

import (
	"fmt"
	"wox/util"
)

// getWaylandMouseScreen selects compositor-owned geometry before the stable GTK fallback.
func getWaylandMouseScreen() (Size, string, error) {
	switch {
	case util.IsKDEDesktopSession():
		return getKDEMouseScreen()
	case isHyprlandSession():
		return getHyprlandMouseScreen()
	default:
		return Size{}, "", fmt.Errorf("wayland-no-global-pointer")
	}
}
