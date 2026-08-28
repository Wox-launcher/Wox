//go:build linux

package woxui

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
