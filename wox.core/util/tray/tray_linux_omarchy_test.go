//go:build linux

package tray

import (
	"strings"
	"testing"
)

func TestOmarchyTraySession(t *testing.T) {
	t.Setenv("OMARCHY_PATH", t.TempDir())
	t.Setenv("XDG_SESSION_TYPE", "wayland")
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	t.Setenv("XDG_CURRENT_DESKTOP", "Hyprland")
	t.Setenv("XDG_SESSION_DESKTOP", "Hyprland")
	t.Setenv("DESKTOP_SESSION", "omarchy")
	if !isOmarchyTraySession() {
		t.Fatal("expected Omarchy session to use its tray icon")
	}
	t.Setenv("DESKTOP_SESSION", "hyprland")
	if isOmarchyTraySession() {
		t.Fatal("another Hyprland session must keep the regular tray icon")
	}
	t.Setenv("DESKTOP_SESSION", "omarchy")
	t.Setenv("XDG_CURRENT_DESKTOP", "KDE")
	t.Setenv("XDG_SESSION_DESKTOP", "KDE")
	if isOmarchyTraySession() {
		t.Fatal("another desktop must keep the regular tray icon")
	}
}

func TestOmarchyTrayIconIsSymbolic(t *testing.T) {
	icon, err := omarchyTrayIcon.ReadFile("wox-tray-symbolic.svg")
	if err != nil || !strings.Contains(string(icon), "<path") {
		t.Fatalf("symbolic icon unavailable: %v", err)
	}
}
