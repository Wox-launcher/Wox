//go:build linux

package window

import "testing"

func TestLinuxLayerShellStacksTopmostAboveLauncher(t *testing.T) {
	const (
		top     = int32(2)
		overlay = int32(3)
	)
	if got := testLinuxLayerShellStackLayer(false, false); got != top {
		t.Fatalf("launcher layer = %d, want TOP (%d) so query chrome stays below HUDs", got, top)
	}
	if got := testLinuxLayerShellStackLayer(true, false); got != overlay {
		t.Fatalf("topmost overlay layer = %d, want OVERLAY (%d)", got, overlay)
	}
	if got := testLinuxLayerShellStackLayer(false, true); got != overlay {
		t.Fatalf("screenshot layer = %d, want OVERLAY (%d) so capture chrome covers the launcher", got, overlay)
	}
	if testLinuxLayerShellStackLayer(true, false) <= testLinuxLayerShellStackLayer(false, false) {
		t.Fatal("topmost overlays must stack above the launcher query window")
	}
}

func TestLinuxLayerShellScreenshotCoversPanel(t *testing.T) {
	if got := testLinuxLayerShellExclusiveZone(false); got != 0 {
		t.Fatalf("launcher exclusive zone = %d, want 0 so a panel keeps its reserved strip", got)
	}
	if got := testLinuxLayerShellExclusiveZone(true); got != -1 {
		t.Fatalf("screenshot exclusive zone = %d, want -1 so the mask covers a panel", got)
	}
}
