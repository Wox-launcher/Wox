//go:build linux

package screenshot

import (
	"context"
	"image"
	"slices"
	"testing"

	utilscreen "wox/util/screen"
)

func TestParseLinuxHyprlandScreenshotWindows(t *testing.T) {
	payload := []byte(`[
		{"at":[0,0],"size":[1920,1080],"workspace":{"id":1},"mapped":true,"hidden":false,"floating":false,"fullscreen":0,"focusHistoryID":1,"pid":10},
		{"at":[400,300],"size":[600,400],"workspace":{"id":1},"mapped":true,"hidden":false,"floating":true,"fullscreen":0,"focusHistoryID":5,"pid":11},
		{"at":[0,0],"size":[800,600],"workspace":{"id":7},"mapped":true,"hidden":false,"floating":false,"fullscreen":0,"focusHistoryID":0,"pid":12},
		{"at":[10,10],"size":[300,200],"workspace":{"id":1},"mapped":false,"hidden":false,"floating":false,"fullscreen":0,"focusHistoryID":0,"pid":13},
		{"at":[20,20],"size":[100,80],"workspace":{"id":1},"mapped":true,"hidden":true,"floating":false,"fullscreen":0,"focusHistoryID":0,"pid":14},
		{"at":[12,38],"size":[400,300],"workspace":{"id":2},"mapped":true,"hidden":false,"pinned":true,"floating":false,"fullscreen":0,"focusHistoryID":4,"pid":15}
	]`)
	monitors := []utilscreen.HyprlandMonitor{{ActiveWorkspace: utilscreen.HyprlandWorkspaceRef{ID: 1}}}
	windows := parseLinuxHyprlandScreenshotWindows(payload, monitors)
	got := make([]Rect, len(windows))
	for i, window := range windows {
		got[i] = window.frame
	}
	want := []Rect{
		{X: 12, Y: 38, Width: 400, Height: 300},
		{X: 400, Y: 300, Width: 600, Height: 400},
		{X: 0, Y: 0, Width: 1920, Height: 1080},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("windows = %+v", got)
	}
	if windows[0].pid != 15 || windows[1].pid != 11 || windows[2].pid != 10 {
		t.Fatalf("pids = %d %d %d", windows[0].pid, windows[1].pid, windows[2].pid)
	}
}

func TestParseLinuxHyprlandScreenshotWindowsStacksFullscreenAndSpecialWorkspace(t *testing.T) {
	payload := []byte(`[
		{"at":[0,38],"size":[200,100],"workspace":{"id":1},"mapped":true,"floating":true,"focusHistoryID":0,"pid":1},
		{"at":[0,0],"size":[1920,1080],"workspace":{"id":1},"mapped":true,"fullscreen":2,"focusHistoryID":3,"pid":2},
		{"at":[40,40],"size":[300,200],"workspace":{"id":-99},"mapped":true,"floating":true,"focusHistoryID":8,"pid":3}
	]`)
	monitors := []utilscreen.HyprlandMonitor{{
		ActiveWorkspace:  utilscreen.HyprlandWorkspaceRef{ID: 1},
		SpecialWorkspace: utilscreen.HyprlandWorkspaceRef{ID: -99},
	}}
	windows := parseLinuxHyprlandScreenshotWindows(payload, monitors)
	if len(windows) != 3 || windows[0].pid != 3 || windows[1].pid != 2 || windows[2].pid != 1 {
		t.Fatalf("stack = %+v", windows)
	}
}

func TestParseLinuxHyprlandScreenshotWindowsRejectsUnexpectedPayload(t *testing.T) {
	monitors := []utilscreen.HyprlandMonitor{{ActiveWorkspace: utilscreen.HyprlandWorkspaceRef{ID: 1}}}
	if parseLinuxHyprlandScreenshotWindows([]byte(`not json`), monitors) != nil {
		t.Fatal("accepted invalid JSON")
	}
	if parseLinuxHyprlandScreenshotWindows([]byte(`{}`), monitors) != nil {
		t.Fatal("accepted a non-array payload")
	}
}

func TestLinuxHyprlandScreenshotObjectSelectionUsesCaptureCoordinates(t *testing.T) {
	windows := []linuxScreenshotWindow{{
		pid:    7,
		frame:  Rect{X: 1920, Y: 40, Width: 800, Height: 600},
		client: Rect{X: 1920, Y: 40, Width: 800, Height: 600},
	}}
	bounds := Rect{X: 1920, Y: 0, Width: 1280, Height: 800}
	candidates, query := linuxHyprlandScreenshotObjectSelection(windows, bounds, image.NewRGBA(image.Rect(0, 0, 1280, 800)))
	want := []Rect{{X: 0, Y: 40, Width: 800, Height: 600}}
	if !slices.Equal(candidates, want) {
		t.Fatalf("candidates = %+v", candidates)
	}
	if query == nil {
		t.Fatal("missing element query")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if path := query(ctx, Point{X: 10, Y: 50}); path != nil {
		t.Fatal("cancelled element query returned objects")
	}
	offscreen := []linuxScreenshotWindow{{frame: Rect{X: -400, Y: 0, Width: 100, Height: 80}}}
	if candidates, query := linuxHyprlandScreenshotObjectSelection(offscreen, bounds, image.NewRGBA(image.Rect(0, 0, 100, 80))); candidates != nil || query != nil {
		t.Fatal("kept a window outside the capture")
	}
}
