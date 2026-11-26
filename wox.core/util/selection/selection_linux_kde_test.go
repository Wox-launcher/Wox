//go:build linux && cgo

package selection

import "testing"

func TestKDEWaylandSupportsSelectionQuery(t *testing.T) {
	t.Setenv("XDG_SESSION_TYPE", "wayland")
	t.Setenv("XDG_CURRENT_DESKTOP", "KDE")
	if !SupportsSelectionQuery() {
		t.Fatal("KDE Wayland should expose the selection hotkey")
	}

	t.Setenv("XDG_CURRENT_DESKTOP", "GNOME")
	t.Setenv("XDG_SESSION_DESKTOP", "gnome")
	t.Setenv("DESKTOP_SESSION", "gnome")
	t.Setenv("GDMSESSION", "gnome")
	if SupportsSelectionQuery() {
		t.Fatal("GNOME Wayland should keep the selection hotkey disabled")
	}
}

func TestKDESelectionCopyUsesTerminalShortcut(t *testing.T) {
	for _, identity := range []string{"org.kde.konsole.desktop", "Konsole", "org.kde.yakuake", "foot"} {
		if !kdeSelectionCopyUsesShift(identity) {
			t.Errorf("%s should copy with Ctrl+Shift+C", identity)
		}
	}
	for _, identity := range []string{"com.google.Chrome", "org.kde.dolphin.desktop"} {
		if kdeSelectionCopyUsesShift(identity) {
			t.Errorf("%s should copy with Ctrl+C", identity)
		}
	}
}
