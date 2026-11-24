//go:build !linux

package window

import "context"

// GetActiveWindowIdentity captures the window target before Wox takes focus.
func GetActiveWindowIdentity(_ context.Context) (pid int, id string, name string) {
	return GetActiveWindowPid(), GetActiveWindowId(), GetActiveWindowName()
}
