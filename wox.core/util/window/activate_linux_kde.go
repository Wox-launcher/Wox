//go:build linux

package window

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// FocusKDEWindow restores the captured KWin window and checks fresh compositor
// state before the caller sends input. Multiple windows may share one PID.
func FocusKDEWindow(ctx context.Context, windowID string, pid int) error {
	target := ""
	if windowID != "" {
		id, err := uuid.Parse(windowID)
		if err != nil || id == uuid.Nil {
			return ErrWindowManagementWindowNotFound
		}
		target = id.String()
	}
	if target == "" && pid <= 0 {
		return ErrWindowManagementWindowNotFound
	}

	ctx, cancel := context.WithTimeout(ctx, 350*time.Millisecond)
	defer cancel()
	app, err := queryKDEWindow(ctx, fmt.Sprintf(kwinFocusWindowScript, target, pid))
	if err != nil && !errors.Is(err, errLinuxForegroundMissing) {
		return err
	}

	// XWayland can report focus after the activation request returns to the
	// script. Read again without reactivating over a subsequent focus change.
	for (target != "" && app.WindowID != target) || (pid > 0 && app.Pid != pid) {
		select {
		case <-ctx.Done():
			return fmt.Errorf("KWin did not focus window %s: %w", target, ctx.Err())
		case <-time.After(25 * time.Millisecond):
		}
		app, err = queryKDEForeground(ctx)
		if err != nil && !errors.Is(err, errLinuxForegroundMissing) {
			return err
		}
	}
	return nil
}

const kwinFocusWindowScript = `
    var targetId = "%s";
    var targetPid = %d;
    var active = workspace.activeWindow;
    function matchesTarget(window) {
      return window && (!targetId || windowId(window) === targetId) &&
        (targetPid <= 0 || Number(window.pid) === targetPid);
    }
    if (!matchesTarget(active)) {
      var windows = workspace.stackingOrder;
      for (var i = windows.length - 1; i >= 0; i--) {
        var candidate = windows[i];
        if (matchesTarget(candidate)) {
          workspace.activeWindow = candidate;
          break;
        }
      }
    }
`
