//go:build windows

package woxui

import (
	"testing"
	"wox/util/screen"
)

// TestWindowsDisplayForLogicalBounds covers overlapping logical desktops and display transitions.
func TestWindowsDisplayForLogicalBounds(t *testing.T) {
	primary := screen.Display{ID: "main", PixelBounds: screen.Rect{Width: 2560, Height: 1440}, Scale: 1, Primary: true}
	lower := screen.Display{ID: "lower", PixelBounds: screen.Rect{Y: 1440, Width: 2560, Height: 1200}, Scale: 1.25}
	left := screen.Display{ID: "left", PixelBounds: screen.Rect{X: -1920, Y: -600, Width: 1920, Height: 1200}, Scale: 1.5}
	for _, tc := range []struct {
		name   string
		bounds Rect
		want   string
	}{
		{"primary show", Rect{905, 410, 750, 75}, "main"},
		{"lower show overlaps primary logically", Rect{663, 1322, 750, 75}, "lower"},
		{"lower results expand", Rect{663, 1322, 750, 571}, "lower"},
		{"return to primary", Rect{905, 410, 750, 571}, "main"},
		{"negative desktop origin", Rect{-1100, -300, 750, 571}, "left"},
		{"outside desktop retains fallback", Rect{10000, 10000, 750, 75}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, displays := range [][]screen.Display{{primary, lower, left}, {left, lower, primary}} {
				if got := windowsDisplayForLogicalBounds(tc.bounds, displays); got.ID != tc.want {
					t.Fatalf("display = %q, want %q", got.ID, tc.want)
				}
			}
		})
	}
}
