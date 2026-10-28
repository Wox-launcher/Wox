package ui

import (
	"encoding/json"
	"testing"

	"wox/common"
	"wox/setting"
	"wox/util/screen"
	"wox/util/tray"
)

// TestResolveLastLocationOnDisplays covers legacy data and monitor-relative restoration.
func TestResolveLastLocationOnDisplays(t *testing.T) {
	displays := []screen.Display{
		{ID: "right", Bounds: screen.Rect{X: 960, Width: 1280, Height: 720}, WorkArea: screen.Rect{X: 960, Width: 1280, Height: 672}, Scale: 2},
		{ID: "primary", Bounds: screen.Rect{Width: 1920, Height: 1080}, WorkArea: screen.Rect{Width: 1920, Height: 1032}, Scale: 1},
		{ID: "left", Bounds: screen.Rect{X: -1600, Y: -200, Width: 1600, Height: 1000}, WorkArea: screen.Rect{X: -1600, Y: -200, Width: 1600, Height: 960}, Scale: 1.5},
	}
	for _, tc := range []struct {
		name  string
		saved setting.SavedWindowPosition
		want  Position
		ok    bool
	}{
		{"overlap keeps primary", setting.SavedWindowPosition{X: 610, Y: 234, Valid: true, DisplayID: "primary"}, Position{Type: setting.PositionTypeLastLocation, X: 610, Y: 234, DisplayID: "primary"}, true},
		{"moved and rescaled display", setting.SavedWindowPosition{X: 3000, Y: 320, Valid: true, DisplayID: "right", WorkX: 2700, WorkY: 120}, Position{Type: setting.PositionTypeLastLocation, X: 1260, Y: 200, DisplayID: "right"}, true},
		{"negative origin", setting.SavedWindowPosition{X: -1300, Y: 10, Valid: true, DisplayID: "left", WorkX: -1600, WorkY: -200}, Position{Type: setting.PositionTypeLastLocation, X: -1300, Y: 10, DisplayID: "left"}, true},
		{"disconnected display", setting.SavedWindowPosition{X: 610, Y: 234, Valid: true, DisplayID: "disconnected"}, Position{}, false},
		{"outside selected display", setting.SavedWindowPosition{X: 1800, Y: 800, Valid: true, DisplayID: "right", WorkX: 960}, Position{}, false},
		{"partly left of primary", setting.SavedWindowPosition{X: -128, Y: 772, Width: 800, Height: 569, Valid: true, DisplayID: "primary"}, Position{Type: setting.PositionTypeLastLocation, X: -128, Y: 772, DisplayID: "primary"}, true},
		{"partly left of secondary", setting.SavedWindowPosition{X: 889, Y: 300, Width: 800, Height: 300, Valid: true, DisplayID: "right", WorkX: 960}, Position{Type: setting.PositionTypeLastLocation, X: 889, Y: 300, DisplayID: "right"}, true},
		{"partly above primary", setting.SavedWindowPosition{X: 200, Y: -80, Width: 800, Height: 100, Valid: true, DisplayID: "primary"}, Position{Type: setting.PositionTypeLastLocation, X: 200, Y: -80, DisplayID: "primary"}, true},
		{"partly below primary", setting.SavedWindowPosition{X: 200, Y: 1070, Width: 800, Height: 100, Valid: true, DisplayID: "primary"}, Position{Type: setting.PositionTypeLastLocation, X: 200, Y: 1070, DisplayID: "primary"}, true},
		{"touching left edge is offscreen", setting.SavedWindowPosition{X: -800, Y: 200, Width: 800, Height: 100, Valid: true, DisplayID: "primary"}, Position{}, false},
		{"touching top edge is offscreen", setting.SavedWindowPosition{X: 200, Y: -100, Width: 800, Height: 100, Valid: true, DisplayID: "primary"}, Position{}, false},
		{"legacy partially visible", setting.SavedWindowPosition{X: -100, Y: 900, Valid: true}, Position{Type: setting.PositionTypeLastLocation, X: -100, Y: 900}, true},
		{"legacy coordinates", setting.SavedWindowPosition{X: 610, Y: 234, Valid: true}, Position{Type: setting.PositionTypeLastLocation, X: 610, Y: 234}, true},
		{"invalid", setting.SavedWindowPosition{}, Position{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := resolveLastLocationOnDisplays(tc.saved, displays, 800, 300)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("got %+v, %v; want %+v, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
	var legacy setting.SavedWindowPosition
	if err := json.Unmarshal([]byte(`{"x":610,"y":234,"valid":true}`), &legacy); err != nil || legacy.DisplayID != "" || !legacy.Valid {
		t.Fatalf("legacy data = %+v, %v", legacy, err)
	}
}

// TestExplicitPositionAndTrayMonitor retains the source identity across overlapping displays.
func TestExplicitPositionAndTrayMonitor(t *testing.T) {
	displays := []screen.Display{
		{ID: "right", WorkArea: screen.Rect{X: 960, Width: 1280, Height: 672}},
		{ID: "primary", WorkArea: screen.Rect{Width: 1920, Height: 1032}},
	}
	got, ok := trayQueryDisplay(tray.ClickRect{X: 1000, Y: 300, DisplayID: "primary"}, displays)
	if !ok || got.ID != "primary" {
		t.Fatalf("tray display = %+v, %v", got, ok)
	}
	if _, ok := trayQueryDisplay(tray.ClickRect{X: 1000, Y: 300, DisplayID: "disconnected"}, displays); ok {
		t.Fatal("disconnected tray display should fall back to the mouse screen")
	}
	position := explicitWindowPosition(common.WindowPosition{X: 610, Y: 234, DisplayID: got.ID})
	if position.X != 610 || position.Y != 234 || position.DisplayID != "primary" {
		t.Fatalf("explicit position = %+v", position)
	}
}
