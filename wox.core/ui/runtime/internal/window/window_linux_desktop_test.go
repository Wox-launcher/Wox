//go:build linux

package window

import "testing"

// TestLinuxWindowPolicy covers legacy aliases and the native-display override.
func TestLinuxWindowPolicy(t *testing.T) {
	for _, tc := range []struct {
		desktop, name string
		layerShell    bool
	}{
		{"COSMIC", "cosmic", true},
		{"pop:COSMIC", "cosmic", true},
		{"KDE", "kde", true},
		{"plasmawayland", "kde", true},
		{"Hyprland", "hyprland", true},
		{"hyprland-uwsm", "hyprland", true},
		{"sway:wlroots", "wlroots", true},
		{"Wayfire", "wlroots", true},
		{"river", "wlroots", true},
		{"wlroots", "wlroots", true},
		{"GNOME", "gnome", false},
		{"pop:GNOME", "gnome", false},
		{"unknown", "wayland", false},
		{"", "wayland", false},
	} {
		t.Run(tc.desktop, func(t *testing.T) {
			for _, name := range []string{"XDG_CURRENT_DESKTOP", "XDG_SESSION_DESKTOP", "DESKTOP_SESSION", "GDMSESSION", "GNOME_DESKTOP_SESSION_ID", "XDG_SESSION_TYPE", "WAYLAND_DISPLAY"} {
				t.Setenv(name, "")
			}
			t.Setenv("XDG_CURRENT_DESKTOP", tc.desktop)
			if got := selectLinuxWindowPolicy(true); got.name != tc.name || got.layerShell != tc.layerShell {
				t.Fatalf("Wayland policy = %+v, want %s layerShell=%v", got, tc.name, tc.layerShell)
			}
			t.Setenv("XDG_SESSION_TYPE", "wayland")
			t.Setenv("WAYLAND_DISPLAY", "wayland-0")
			if got := selectLinuxWindowPolicy(false); got.name != "x11" || got.layerShell {
				t.Fatalf("actual X11 display must override session variables: %+v", got)
			}
		})
	}
}

// TestLinuxWindowDesktopFallback preserves precedence for conflicting session metadata.
func TestLinuxWindowDesktopFallback(t *testing.T) {
	for _, tc := range []struct {
		current, session, desktop, gdm string
		layerShell                     bool
	}{
		{session: "KDE", layerShell: true},
		{desktop: "plasmawayland", layerShell: true},
		{current: "GNOME", session: "KDE", layerShell: false},
		{current: "unknown", desktop: "sway", layerShell: false},
		{gdm: "KDE", layerShell: false},
		// COSMIC already used the shared session detector, including GDMSESSION.
		{gdm: "COSMIC", layerShell: true},
		{current: "GNOME", session: "COSMIC", layerShell: true},
	} {
		t.Run(tc.current+"/"+tc.session+"/"+tc.desktop+"/"+tc.gdm, func(t *testing.T) {
			t.Setenv("XDG_CURRENT_DESKTOP", tc.current)
			t.Setenv("XDG_SESSION_DESKTOP", tc.session)
			t.Setenv("DESKTOP_SESSION", tc.desktop)
			t.Setenv("GDMSESSION", tc.gdm)
			if got := selectLinuxWindowPolicy(true).layerShell; got != tc.layerShell {
				t.Fatalf("layerShell = %v, want %v", got, tc.layerShell)
			}
		})
	}
}
