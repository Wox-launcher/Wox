//go:build linux

package screen

import (
	"encoding/json"
	"testing"
)

func TestHyprlandMonitorFractionalScaleGeometry(t *testing.T) {
	monitor := HyprlandMonitor{Name: "HDMI-A-1", Width: 1920, Height: 1200, X: 1920, Y: 0, Scale: 1.5}
	if got := monitor.LogicalSize(); got != (Size{X: 1920, Y: 0, Width: 1280, Height: 800}) {
		t.Fatalf("logical size = %+v", got)
	}
	if got := monitor.PixelSize(); got != (Size{Width: 1920, Height: 1200}) {
		t.Fatalf("pixel size = %+v", got)
	}
}

func TestHyprlandMonitorRotatedGeometry(t *testing.T) {
	monitor := HyprlandMonitor{Width: 1920, Height: 1080, Scale: 1.5, Transform: 1}
	if got := monitor.LogicalSize(); got.Width != 720 || got.Height != 1280 {
		t.Fatalf("rotated logical size = %+v", got)
	}
	if got := monitor.PixelSize(); got.Width != 1080 || got.Height != 1920 {
		t.Fatalf("rotated pixel size = %+v", got)
	}
}

func TestHyprlandMonitorWorkspaceJSON(t *testing.T) {
	var monitors []HyprlandMonitor
	payload := []byte(`[{"name":"HDMI-A-1","width":1920,"height":1080,"x":0,"y":0,"scale":1,"activeWorkspace":{"id":1,"name":"1"},"specialWorkspace":{"id":-99,"name":"special:scratch"}}]`)
	if err := json.Unmarshal(payload, &monitors); err != nil {
		t.Fatal(err)
	}
	if len(monitors) != 1 || monitors[0].ActiveWorkspace.ID != 1 || monitors[0].SpecialWorkspace.ID != -99 {
		t.Fatalf("workspaces = %+v %+v", monitors[0].ActiveWorkspace, monitors[0].SpecialWorkspace)
	}
}
