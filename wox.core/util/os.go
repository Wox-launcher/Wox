package util

import (
	"os"
	"runtime"
	"strings"
)

type Platform = string

const (
	PlatformWindows Platform = "windows"
	PlatformMacOS   Platform = "darwin"
	PlatformLinux   Platform = "linux"
)

func IsWindows() bool {
	return strings.ToLower(runtime.GOOS) == PlatformWindows
}

func IsMacOS() bool {
	return strings.ToLower(runtime.GOOS) == PlatformMacOS
}

func IsArm64() bool {
	return strings.ToLower(runtime.GOARCH) == "arm64"
}

func IsAmd64() bool {
	return strings.ToLower(runtime.GOARCH) == "amd64"
}

func IsLinux() bool {
	return strings.ToLower(runtime.GOOS) == PlatformLinux
}

// IsKDEWayland reports whether Wox is running in a KDE/Plasma Wayland session.
func IsKDEWayland() bool {
	return IsLinuxWaylandSession() && IsKDEDesktopSession()
}

// IsGnomeWayland reports whether Wox is running in a GNOME Wayland session.
func IsGnomeWayland() bool {
	return IsLinuxWaylandSession() && IsGnomeDesktopSession()
}

// IsKDEDesktopSession reports whether the current desktop session identifies as KDE/Plasma.
func IsKDEDesktopSession() bool {
	return currentDesktopSessionContains("kde", "plasma")
}

// IsGnomeDesktopSession reports whether the current desktop session identifies as GNOME.
func IsGnomeDesktopSession() bool {
	return currentDesktopSessionContains("gnome") || os.Getenv("GNOME_DESKTOP_SESSION_ID") != ""
}

// IsHyprlandSession reports whether the current desktop session is Hyprland.
// Used to select the native Hyprland hotkey backend (hl.bind + wox:// deeplink)
// instead of the portal backend, and for diagnostics.
func IsHyprlandSession() bool {
	if !IsLinuxWaylandSession() {
		return false
	}
	return currentDesktopSessionContains("hyprland")
}

func GetCurrentPlatform() string {
	return strings.ToLower(runtime.GOOS)
}

func IsSupportedPlatform(platform string) bool {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case PlatformWindows, PlatformMacOS, PlatformLinux:
		return true
	default:
		return false
	}
}

func currentDesktopSessionContains(values ...string) bool {
	for _, envName := range []string{"XDG_CURRENT_DESKTOP", "XDG_SESSION_DESKTOP", "DESKTOP_SESSION", "GDMSESSION"} {
		if desktopSessionContains(os.Getenv(envName), values...) {
			return true
		}
	}
	return false
}

func desktopSessionContains(session string, values ...string) bool {
	parts := strings.FieldsFunc(strings.ToLower(session), func(r rune) bool {
		return r == ':' || r == ';' || r == ',' || r == ' '
	})
	for _, part := range parts {
		for _, value := range values {
			if part == value {
				return true
			}
		}
	}

	return false
}
