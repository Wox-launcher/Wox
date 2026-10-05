package window

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// queryHyprlandForeground reads the focused window from the compositor.
// class is the Wayland app id; initialClass keeps the id from before a rename.
func queryHyprlandForeground(ctx context.Context) (linuxForegroundApp, error) {
	output, err := exec.CommandContext(ctx, "hyprctl", "-j", "activewindow").Output()
	if err != nil {
		return linuxForegroundApp{}, fmt.Errorf("hyprctl activewindow: %w", err)
	}
	return parseHyprlandForeground(output)
}

func parseHyprlandForeground(output []byte) (linuxForegroundApp, error) {
	var active struct {
		Class        string `json:"class"`
		InitialClass string `json:"initialClass"`
		Title        string `json:"title"`
		Pid          int    `json:"pid"`
	}
	if err := json.Unmarshal(output, &active); err != nil {
		return linuxForegroundApp{}, fmt.Errorf("hyprctl activewindow: %w", err)
	}
	app := linuxForegroundApp{
		Pid:   active.Pid,
		Title: active.Title,
		AppID: active.Class,
		AltID: active.InitialClass,
	}
	if normalizeLinuxAppID(app.AppID) == "" && normalizeLinuxAppID(app.AltID) == "" && app.Pid <= 0 {
		return linuxForegroundApp{}, errLinuxForegroundMissing
	}
	return app, nil
}
