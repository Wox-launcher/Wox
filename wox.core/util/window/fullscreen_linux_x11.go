//go:build linux

package window

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
)

// isX11ActiveWindowFullscreen reads EWMH state; it never queries XWayland for a
// native Wayland window. xprop isolates X11 BadWindow races from the Wox process.
func isX11ActiveWindowFullscreen(ctx context.Context) bool {
	output, err := exec.CommandContext(ctx, "xprop", "-root", "_NET_ACTIVE_WINDOW").Output()
	if err != nil {
		return false
	}
	_, id, ok := strings.Cut(string(output), "#")
	id = strings.TrimSpace(id)
	windowID, err := strconv.ParseUint(strings.TrimPrefix(id, "0x"), 16, 32)
	if !ok || err != nil || windowID == 0 {
		return false
	}
	output, err = exec.CommandContext(ctx, "xprop", "-id", id, "_NET_WM_STATE").Output()
	if err != nil {
		return false
	}
	_, states, ok := strings.Cut(string(output), "=")
	if !ok {
		return false
	}
	for _, state := range strings.Split(states, ",") {
		if strings.TrimSpace(state) == "_NET_WM_STATE_FULLSCREEN" {
			return true
		}
	}
	return false
}
