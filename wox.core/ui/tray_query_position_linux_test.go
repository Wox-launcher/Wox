//go:build linux

package ui

import (
	"context"
	"testing"

	"wox/common"
	"wox/util/tray"
)

func TestTrayQueryPositionAtPanelEdges(t *testing.T) {
	t.Setenv("OMARCHY_PATH", t.TempDir())
	t.Setenv("XDG_SESSION_TYPE", "wayland")
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	t.Setenv("XDG_CURRENT_DESKTOP", "Hyprland")
	t.Setenv("DESKTOP_SESSION", "omarchy")
	manager := &Manager{}
	screen := common.WindowRect{X: -1600, Y: -200, Width: 1600, Height: 960}
	for _, tc := range []struct {
		name    string
		y, want int
	}{
		{"top", -180, -200},
		{"bottom", 750, 442},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pos := manager.getTrayQueryWindowPosition(context.Background(), tray.ClickRect{X: -800, Y: tc.y, Width: 1, Height: 1}, screen, 400, 300, 0)
			if pos.X != -999 || pos.Y != tc.want {
				t.Fatalf("position = %+v, want (-999, %d)", pos, tc.want)
			}
		})
	}
}

// TestTrayQueryTopEdgeIgnoresPointerY keeps the popup against the panel at any click height.
func TestTrayQueryTopEdgeIgnoresPointerY(t *testing.T) {
	t.Setenv("OMARCHY_PATH", t.TempDir())
	t.Setenv("XDG_SESSION_TYPE", "wayland")
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	t.Setenv("XDG_CURRENT_DESKTOP", "Hyprland")
	t.Setenv("DESKTOP_SESSION", "omarchy")
	manager := &Manager{}
	for _, y := range []int{-200, -190, -175} {
		area := common.WindowRect{X: -1600, Y: -174, Width: 1600, Height: 974}
		pos := manager.getTrayQueryWindowPosition(context.Background(), tray.ClickRect{X: -800, Y: y, Width: 1, Height: 1}, area, 400, 300, 0)
		if pos.Y != area.Y {
			t.Fatalf("pointer y=%d: popup y=%d, want %d", y, pos.Y, area.Y)
		}
	}
}
