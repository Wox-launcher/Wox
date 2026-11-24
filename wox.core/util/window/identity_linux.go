package window

import (
	"context"
	"wox/util"
)

// GetActiveWindowIdentity captures the window target before Wox takes focus.
func GetActiveWindowIdentity(ctx context.Context) (pid int, id string, name string) {
	if util.IsKDEWayland() {
		return getKDEActiveWindowIdentity(ctx)
	}
	return GetActiveWindowPid(), GetActiveWindowId(), GetActiveWindowName()
}
