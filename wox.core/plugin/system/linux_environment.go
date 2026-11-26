package system

import (
	"os"
	"strings"

	"wox/util"
)

type linuxDesktop string

const (
	linuxDesktopOther    linuxDesktop = "other"
	linuxDesktopGNOME    linuxDesktop = "GNOME"
	linuxDesktopKDE      linuxDesktop = "KDE"
	linuxDesktopHyprland linuxDesktop = "Hyprland"
	linuxDesktopCOSMIC   linuxDesktop = "COSMIC"
)

type linuxDisplayProtocol string

const (
	linuxDisplayOther   linuxDisplayProtocol = "other"
	linuxDisplayWayland linuxDisplayProtocol = "wayland"
	linuxDisplayX11     linuxDisplayProtocol = "x11"
)

type linuxEnvironment struct {
	Desktop         linuxDesktop         `json:"desktop"`
	DisplayProtocol linuxDisplayProtocol `json:"displayProtocol"`
}

// currentLinuxEnvironment projects session hints onto public labels for shareable diagnostics.
func currentLinuxEnvironment() linuxEnvironment {
	environment := linuxEnvironment{Desktop: linuxDesktopOther, DisplayProtocol: linuxDisplayOther}
	switch {
	case util.IsLinuxWaylandSession():
		environment.DisplayProtocol = linuxDisplayWayland
	case strings.EqualFold(os.Getenv("XDG_SESSION_TYPE"), "x11") || os.Getenv("DISPLAY") != "":
		environment.DisplayProtocol = linuxDisplayX11
	}
	switch {
	case util.IsHyprlandSession():
		environment.Desktop = linuxDesktopHyprland
	case util.IsKDEDesktopSession():
		environment.Desktop = linuxDesktopKDE
	case util.IsCosmicDesktopSession():
		environment.Desktop = linuxDesktopCOSMIC
	case util.IsGnomeDesktopSession():
		environment.Desktop = linuxDesktopGNOME
	}
	return environment
}

func (protocol linuxDisplayProtocol) label() string {
	switch protocol {
	case linuxDisplayWayland:
		return "Wayland"
	case linuxDisplayX11:
		return "X11 (Xorg)"
	default:
		return string(linuxDisplayOther)
	}
}
