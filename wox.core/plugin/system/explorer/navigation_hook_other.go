//go:build !windows

package explorer

import "context"

func setExplorerDialogHookEnabled(enabled bool) {}

func navigateFileDialogWithHook(ctx context.Context, windowID string, pid int, targetPath string) bool {
	return false
}
