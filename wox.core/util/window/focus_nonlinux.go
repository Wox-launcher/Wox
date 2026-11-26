//go:build windows || darwin

package window

import "context"

func focusCapturedWindow(ctx context.Context, id string, pid int) error {
	matches := func() bool {
		foregroundPID, foregroundID, _ := GetActiveWindowIdentity(ctx)
		return (pid <= 0 || foregroundPID == pid) && (id == "" || foregroundID == id)
	}
	if matches() {
		return ctx.Err()
	}
	if err := requestCapturedWindowActivation(id, pid); err != nil {
		return err
	}
	return waitForForeground(ctx, matches)
}
