package window

import (
	"context"
	"os/exec"
	"time"
	"wox/util"
)

// fullscreenDetector selects only the backend for the current desktop session.
// Other Wayland compositors do not expose a portable foreground-window query.
func fullscreenDetector() (string, func(context.Context) bool) {
	if !util.IsLinuxWaylandSession() {
		return "xprop", isX11ActiveWindowFullscreen
	}
	if util.IsHyprlandSession() {
		return "hyprctl", isHyprlandActiveWindowFullscreen
	}
	return "", nil
}

func SupportsActiveWindowFullscreen() bool {
	command, detector := fullscreenDetector()
	_, err := exec.LookPath(command)
	return detector != nil && err == nil
}

// IsActiveWindowFullscreen fails open and bounds the cost of querying the desktop.
func IsActiveWindowFullscreen() bool {
	_, detector := fullscreenDetector()
	if detector == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	return detector(ctx)
}
