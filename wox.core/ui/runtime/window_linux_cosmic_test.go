//go:build linux

package woxui

import "testing"

func TestCosmicLayerShellSelection(t *testing.T) {
	for _, name := range []string{"XDG_CURRENT_DESKTOP", "XDG_SESSION_DESKTOP", "DESKTOP_SESSION", "GDMSESSION"} {
		t.Setenv(name, "")
	}
	for _, desktop := range []string{"COSMIC", "pop:COSMIC", "GNOME", "pop:GNOME", "KDE", "Hyprland", ""} {
		t.Setenv("XDG_CURRENT_DESKTOP", desktop)
		want := desktop == "COSMIC" || desktop == "pop:COSMIC"
		if got := woxGoLinuxCosmicUsesLayerShell() != 0; got != want {
			t.Fatalf("desktop %q: COSMIC layer-shell selection = %v, want %v", desktop, got, want)
		}
	}
}
