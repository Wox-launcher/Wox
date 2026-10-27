package window

import (
	"context"

	"wox/util"
)

// LinuxAppActivation is an existing top-level window for one desktop entry.
// Focus it only after the launcher has hidden.
type LinuxAppActivation struct {
	backend string
	address string
}

// PrepareLinuxAppActivation finds a window for the desktop file without
// changing focus. It reports false when this session cannot see a match, so
// the caller can launch the application.
func PrepareLinuxAppActivation(ctx context.Context, desktopPath string) (LinuxAppActivation, bool) {
	app, ok := parseLinuxDesktopApp(desktopPath)
	if !ok {
		return LinuxAppActivation{}, false
	}
	switch linuxActivationBackend(util.IsLinuxWaylandSession(), util.IsHyprlandSession()) {
	case "hyprland":
		address, found := findHyprlandActivationAddress(ctx, app)
		if !found {
			return LinuxAppActivation{}, false
		}
		return LinuxAppActivation{backend: "hyprland", address: address}, true
	default:
		return LinuxAppActivation{}, false
	}
}

// Focus raises the matched window and switches to its workspace.
func (a LinuxAppActivation) Focus(ctx context.Context) bool {
	switch a.backend {
	case "hyprland":
		return focusHyprlandActivation(ctx, a.address)
	default:
		return false
	}
}

// LinuxAppWindow is one top-level window a desktop entry can own.
type LinuxAppWindow struct {
	Pid   int
	Class string
	AltID string
}

// ListLinuxAppWindows returns the compositor's top-level windows.
// Sessions without a client list return nil, and process matching continues
// from /proc alone.
func ListLinuxAppWindows(ctx context.Context) []LinuxAppWindow {
	switch linuxActivationBackend(util.IsLinuxWaylandSession(), util.IsHyprlandSession()) {
	case "hyprland":
		return listHyprlandAppWindows(ctx)
	default:
		return nil
	}
}

// linuxActivationBackend chooses who can raise an existing window.
// Hyprland exposes clients and a focus dispatcher. Other sessions still launch.
func linuxActivationBackend(wayland bool, hyprland bool) string {
	if wayland && hyprland {
		return "hyprland"
	}
	return ""
}
