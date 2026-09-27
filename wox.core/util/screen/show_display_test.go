package screen

import "testing"

func TestResolveShowDisplayFollowsTrustedIDAfterMove(t *testing.T) {
	saved := SavedDisplay{ID: `\\.\DISPLAY2`, WorkX: 0, WorkY: 0, WorkWidth: 1920, WorkHeight: 1080}
	displays := []Display{
		{ID: `\\.\DISPLAY1`, Primary: true, Bounds: Rect{X: 0, Y: 0, Width: 1920, Height: 1080}, WorkArea: Rect{X: 0, Y: 0, Width: 1920, Height: 1080}},
		{ID: `\\.\DISPLAY2`, Bounds: Rect{X: 1920, Y: 0, Width: 1920, Height: 1080}, WorkArea: Rect{X: 1920, Y: 0, Width: 1920, Height: 1080}},
	}

	display, match := ResolveShowDisplay(displays, saved, true)
	if match != ShowDisplayMatchID || display.ID != `\\.\DISPLAY2` {
		t.Fatalf("match = %v display %q, want the moved saved output", match, display.ID)
	}
}

func TestResolveShowDisplayUsesGeometryWhenOneSimilarMonitorStays(t *testing.T) {
	saved := SavedDisplay{ID: "0", WorkX: -1920, WorkY: 0, WorkWidth: 1920, WorkHeight: 1080}
	displays := []Display{
		{ID: "1", Primary: true, Bounds: Rect{X: 0, Y: 0, Width: 1600, Height: 900}, WorkArea: Rect{X: 0, Y: 0, Width: 1600, Height: 900}},
		{ID: "2", Bounds: Rect{X: -1900, Y: 20, Width: 1920, Height: 1080}, WorkArea: Rect{X: -1900, Y: 20, Width: 1920, Height: 1080}},
	}

	display, match := ResolveShowDisplay(displays, saved, false)
	if match != ShowDisplayMatchGeometry || display.ID != "2" {
		t.Fatalf("match = %v display %q, want the nearby monitor", match, display.ID)
	}
}

func TestResolveShowDisplayUsesLocationBetweenIdenticalMonitors(t *testing.T) {
	saved := SavedDisplay{ID: "0", WorkX: 0, WorkY: 0, WorkWidth: 1920, WorkHeight: 1080}
	displays := []Display{
		{ID: "1", Primary: true, Bounds: Rect{X: 1920, Y: 0, Width: 1920, Height: 1080}, WorkArea: Rect{X: 1920, Y: 0, Width: 1920, Height: 1080}},
		{ID: "0", Bounds: Rect{X: 0, Y: 0, Width: 1920, Height: 1080}, WorkArea: Rect{X: 0, Y: 0, Width: 1920, Height: 1080}},
	}

	display, match := ResolveShowDisplay(displays, saved, false)
	if match != ShowDisplayMatchGeometry || display.ID != "0" {
		t.Fatalf("match = %v display %q, want the monitor at the saved location", match, display.ID)
	}
}

func TestResolveShowDisplayDoesNotGuessWhenIdenticalMonitorsOverlap(t *testing.T) {
	saved := SavedDisplay{WorkX: 0, WorkY: 0, WorkWidth: 1920, WorkHeight: 1080}
	displays := []Display{
		{ID: "a", Primary: true, WorkArea: Rect{Width: 1920, Height: 1080}},
		{ID: "b", WorkArea: Rect{X: 100, Width: 1920, Height: 1080}},
	}
	display, match := ResolveShowDisplay(displays, saved, false)
	if match != ShowDisplayMatchFallback || !display.Primary {
		t.Fatalf("match = %v display %q, want primary fallback", match, display.ID)
	}
}

func TestResolveShowDisplayFallsBackWhenSavedMonitorIsGone(t *testing.T) {
	saved := SavedDisplay{ID: `\\.\DISPLAY2`, WorkX: 1920, WorkY: 0, WorkWidth: 2560, WorkHeight: 1440}
	displays := []Display{
		{ID: `\\.\DISPLAY1`, Primary: true, Bounds: Rect{X: 0, Y: 0, Width: 1920, Height: 1080}, WorkArea: Rect{X: 0, Y: 40, Width: 1920, Height: 1040}},
	}

	display, match := ResolveShowDisplay(displays, saved, true)
	if match != ShowDisplayMatchFallback || display.ID != `\\.\DISPLAY1` {
		t.Fatalf("match = %v display %q, want the primary", match, display.ID)
	}
}

func TestResolveShowDisplayFollowsVerticalArrangementByID(t *testing.T) {
	saved := SavedDisplay{ID: "upper", WorkX: 0, WorkY: -1200, WorkWidth: 1920, WorkHeight: 1080}
	displays := []Display{
		{ID: "lower", Primary: true, Bounds: Rect{X: 0, Y: 0, Width: 1920, Height: 1080}, WorkArea: Rect{X: 0, Y: 0, Width: 1920, Height: 1080}},
		{ID: "upper", Bounds: Rect{X: 0, Y: -1080, Width: 1920, Height: 1080}, WorkArea: Rect{X: 0, Y: -1080, Width: 1920, Height: 1080}},
	}

	display, match := ResolveShowDisplay(displays, saved, true)
	if match != ShowDisplayMatchID || display.WorkArea.Y != -1080 {
		t.Fatalf("match = %v y %d, want the upper display's current work area", match, display.WorkArea.Y)
	}
}

func TestResolveShowDisplayEmptyList(t *testing.T) {
	_, match := ResolveShowDisplay(nil, SavedDisplay{ID: "1", WorkWidth: 100, WorkHeight: 100}, true)
	if match != ShowDisplayMatchNone {
		t.Fatalf("match = %v, want none", match)
	}
}

func TestPointOnAnyDisplay(t *testing.T) {
	displays := []Display{
		{Bounds: Rect{X: -1920, Y: 0, Width: 1920, Height: 1080}},
		{Bounds: Rect{X: 0, Y: 0, Width: 1920, Height: 1080}, WorkArea: Rect{X: 0, Y: 40, Width: 1920, Height: 1040}},
	}
	if !PointOnAnyDisplay(-100, 10, displays) {
		t.Fatal("origin on the left monitor should count")
	}
	if !PointOnAnyDisplay(10, 10, displays) {
		t.Fatal("origin in the system chrome should still count as on that monitor")
	}
	if PointOnAnyDisplay(4000, 10, displays) {
		t.Fatal("origin past every monitor should be off-screen")
	}
	if !PointOnAnyDisplay(1, 1, nil) {
		t.Fatal("an empty list cannot prove the origin is gone")
	}
}

func TestSortDisplaysUsesWorkArea(t *testing.T) {
	displays := []Display{
		{ID: "right", WorkArea: Rect{X: 1000, Y: 0, Width: 800, Height: 600}},
		{ID: "upper", WorkArea: Rect{X: 0, Y: -500, Width: 800, Height: 600}},
		{ID: "lower", WorkArea: Rect{X: 0, Y: 100, Width: 800, Height: 600}},
	}
	SortDisplays(displays)
	if displays[0].ID != "upper" || displays[1].ID != "lower" || displays[2].ID != "right" {
		t.Fatalf("order = %s %s %s", displays[0].ID, displays[1].ID, displays[2].ID)
	}
}
