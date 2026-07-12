//go:build windows

package explorer

import (
	"context"
	"strings"
	"sync/atomic"
	"time"
	"wox/util/windowhook"
)

var explorerDialogHookEnabled atomic.Bool

func setExplorerDialogHookEnabled(enabled bool) {
	explorerDialogHookEnabled.Store(enabled)
}

func navigateFileDialogWithHook(ctx context.Context, windowID string, pid int, targetPath string) bool {
	if !explorerDialogHookEnabled.Load() || pid <= 0 || strings.TrimSpace(targetPath) == "" {
		return false
	}
	return windowhook.NavigateDialog(ctx, windowID, pid, targetPath)
}

// selectFileDialogItemWithHook retries briefly while a cross-folder navigation publishes its new Shell view.
func selectFileDialogItemWithHook(ctx context.Context, windowID string, pid int, targetPath string, waitForView bool) bool {
	if !explorerDialogHookEnabled.Load() || pid <= 0 || strings.TrimSpace(targetPath) == "" {
		return false
	}

	deadline := time.Now()
	if waitForView {
		deadline = deadline.Add(250 * time.Millisecond)
	}
	for {
		if windowhook.SelectDialogItem(ctx, windowID, pid, targetPath) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(15 * time.Millisecond)
	}
}
