package launcher

import (
	"errors"
	"testing"

	"wox/ui/contract"
	"wox/util/screen"
)

// TestLauncherResolveDisplay keeps placement on the monitor that produced the origin.
func TestLauncherResolveDisplay(t *testing.T) {
	primary := screen.Display{ID: "primary", Bounds: screen.Rect{Width: 1920, Height: 1080}, WorkArea: screen.Rect{Width: 1920, Height: 1032}, PixelBounds: screen.Rect{Width: 1920, Height: 1080}, Scale: 1}
	right := screen.Display{ID: "right", Bounds: screen.Rect{X: 960, Width: 1280, Height: 720}, WorkArea: screen.Rect{X: 960, Width: 1280, Height: 672}, PixelBounds: screen.Rect{X: 1920, Width: 2560, Height: 1440}, Scale: 2}
	left := screen.Display{ID: "left", PixelBounds: screen.Rect{X: -2400, Y: -300, Width: 2400, Height: 1500}, Scale: 1.5}
	for _, tc := range []struct {
		name    string
		display screen.Display
		x, y    float64
		wantX   int
		wantY   int
	}{
		{"issue primary", primary, 610, 234, 610, 234},
		{"high DPI right", right, 1250, 100, 2500, 200},
		{"negative origin", left, -1366, -100, -2049, -150},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, displays := range [][]screen.Display{{right, primary, left}, {left, primary, right}} {
				got, err := launcherResolveDisplay(displays, tc.display.ID, func() (screen.Display, error) {
					return screen.Display{}, errors.New("unexpected coordinate fallback")
				})
				if err != nil || got.ID != tc.display.ID {
					t.Fatalf("display = %q, error = %v; want %q", got.ID, err, tc.display.ID)
				}
				if x, y := int(tc.x*got.Scale), int(tc.y*got.Scale); x != tc.wantX || y != tc.wantY {
					t.Fatalf("physical origin = (%d, %d), want (%d, %d)", x, y, tc.wantX, tc.wantY)
				}
			}
		})
	}
	selected, err := launcherResolveDisplay([]screen.Display{right, primary}, primary.ID, func() (screen.Display, error) { return right, nil })
	if err != nil {
		t.Fatal(err)
	}
	// The issue's overlapping logical rectangles previously clipped 447 to 438
	// using the secondary work area and then placed the window at twice its X.
	y, height, _ := constrainLauncherHeightToWorkArea(false, false, 610, 234, 700, 447, 105, 0, []screen.Display{selected})
	if y != 234 || height != 447 {
		t.Fatalf("primary work area produced y=%v height=%v, want 234 and 447", y, height)
	}
	for _, id := range []string{"", "disconnected"} {
		got, err := launcherResolveDisplay([]screen.Display{primary}, id, func() (screen.Display, error) { return primary, nil })
		if err != nil || got.ID != primary.ID {
			t.Fatalf("fallback for %q = %q, %v", id, got.ID, err)
		}
	}
}

// TestShowOptionsPreserveDisplayID covers the core-to-launcher boundary.
func TestShowOptionsPreserveDisplayID(t *testing.T) {
	options := contract.ShowOptions{Position: contract.Position{Type: "mouse_screen", X: 610, Y: 234, DisplayID: "primary"}}
	got := fromCoreShowOptions(options).Position
	if got.DisplayID != "primary" || got.X != 610 || got.Y != 234 {
		t.Fatalf("position = %+v", got)
	}
}
