//go:build windows

package explorer

import (
	"context"
	"strings"
	"sync/atomic"
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
