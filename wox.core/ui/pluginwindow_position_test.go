package ui

import (
	"testing"
	"wox/setting"
	"wox/util/screen"
)

// TestPluginWindowWorkArea follows the launcher's display setting in logical coordinates.
func TestPluginWindowWorkArea(t *testing.T) {
	displays := []screen.Display{
		{ID: "left", Bounds: screen.Rect{X: -1600, Y: -100, Width: 1600, Height: 900}, WorkArea: screen.Rect{X: -1600, Y: -100, Width: 1600, Height: 860}, Scale: 1.5},
		{ID: "primary", Bounds: screen.Rect{X: 0, Y: 0, Width: 1920, Height: 1080}, WorkArea: screen.Rect{X: 0, Y: 0, Width: 1920, Height: 1040}, Primary: true, Scale: 1},
	}
	mouse := screen.Size{DisplayID: "left", X: -1600, Y: -100, Width: 1600, Height: 900}
	active := screen.Size{DisplayID: "primary", Width: 1920, Height: 1080}
	for _, tc := range []struct {
		mode   setting.PositionType
		target setting.ShowDisplayTarget
		last   setting.SavedWindowPosition
		wantX  float32
	}{
		{mode: setting.PositionTypeMouseScreen, wantX: -1600},
		{mode: setting.PositionTypeActiveScreen, wantX: 0},
		{mode: setting.PositionTypeSpecificScreen, target: setting.ShowDisplayTarget{ID: "left"}, wantX: -1600},
		{mode: setting.PositionTypeLastLocation, last: setting.SavedWindowPosition{X: -1200, Y: 100, Width: 800, Height: 400, Valid: true, DisplayID: "left", WorkX: -1600, WorkY: -100}, wantX: -1600},
		{mode: setting.PositionTypeLastLocation, last: setting.SavedWindowPosition{Valid: true, DisplayID: "gone"}, wantX: -1600},
	} {
		got, ok := pluginWindowWorkAreaOnDisplays(tc.mode, tc.target, tc.last, displays, mouse, active, 800, 500, true)
		if !ok || got.X != tc.wantX {
			t.Fatalf("%s: area=%+v ok=%t", tc.mode, got, ok)
		}
	}
}
