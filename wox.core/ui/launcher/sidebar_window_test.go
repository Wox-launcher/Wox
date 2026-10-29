package launcher

import (
	"testing"

	woxui "wox/ui/runtime"
	"wox/util/screen"
)

func TestSidebarExpansionStaysOnLogicalDisplay(t *testing.T) {
	displays := []screen.Display{
		{ID: "left", WorkArea: screen.Rect{X: -1280, Y: 0, Width: 1280, Height: 720}, Scale: 1.25},
		{ID: "main", WorkArea: screen.Rect{X: 0, Y: 0, Width: 1920, Height: 1040}, Scale: 2, Primary: true},
	}
	got := fitSidebarWindowToDisplays(woxui.Rect{X: -600, Y: 40, Width: 880, Height: 600}, woxui.Rect{X: -600, Y: 40, Width: 460, Height: 600}, displays)
	if got.X != -880 || got.Y != 40 || got.Width != 880 {
		t.Fatalf("expanded logical bounds = %+v", got)
	}
	got = fitSidebarWindowToDisplays(woxui.Rect{X: -400, Y: 40, Width: 880, Height: 600}, woxui.Rect{X: -400, Y: 40, Width: 460, Height: 600}, displays)
	if got.X != -880 {
		t.Fatalf("expansion switched the original display: %+v", got)
	}
	got = fitSidebarWindowToDisplays(woxui.Rect{X: 1700, Y: 600, Width: 880, Height: 600}, woxui.Rect{X: 1700, Y: 600, Width: 640, Height: 600}, displays)
	if got.X != 1040 || got.Y != 440 {
		t.Fatalf("main display bounds = %+v", got)
	}
}
