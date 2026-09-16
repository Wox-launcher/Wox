//go:build linux

package window

import (
	"context"
	"encoding/json"
	"os/exec"
)

// isHyprlandActiveWindowFullscreen uses compositor state, not the client's request:
// bit 1 is fullscreen; bit 0 alone is only maximized.
func isHyprlandActiveWindowFullscreen(ctx context.Context) bool {
	output, err := exec.CommandContext(ctx, "hyprctl", "-j", "activewindow").Output()
	if err != nil {
		return false
	}
	var active struct {
		Fullscreen int `json:"fullscreen"`
	}
	return json.Unmarshal(output, &active) == nil && active.Fullscreen&2 != 0
}
