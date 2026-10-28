//go:build linux

package screenshot

import "testing"

// TestLinuxRecordingToolbarPreparation keeps the focus change on Hyprland Wayland.
func TestLinuxRecordingToolbarPreparation(t *testing.T) {
	for _, tc := range []struct {
		desktop, session string
		prepared         bool
	}{
		{"Hyprland", "wayland", true},
		{"Hyprland", "x11", false},
		{"KDE", "wayland", false},
		{"GNOME", "wayland", false},
		{"sway", "wayland", false},
	} {
		t.Run(tc.desktop+"/"+tc.session, func(t *testing.T) {
			t.Setenv("XDG_CURRENT_DESKTOP", tc.desktop)
			t.Setenv("XDG_SESSION_DESKTOP", "")
			t.Setenv("DESKTOP_SESSION", "")
			t.Setenv("XDG_SESSION_TYPE", tc.session)
			t.Setenv("WAYLAND_DISPLAY", "")
			if got := linuxRecordingToolbarPreparation() != nil; got != tc.prepared {
				t.Fatalf("prepare toolbar = %v, want %v", got, tc.prepared)
			}
		})
	}
}
