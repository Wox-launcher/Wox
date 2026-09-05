//go:build windows

package quickjump

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	woxui "wox/ui/runtime"
	"wox/util"
	"wox/util/windowhook"
)

var explorerDialogHookEnabled atomic.Bool

func setExplorerDialogHookEnabled(enabled bool) {
	explorerDialogHookEnabled.Store(enabled)
}

func navigateFileDialogWithHook(ctx context.Context, windowID string, pid int, targetPath string) bool {
	if pid <= 0 || strings.TrimSpace(targetPath) == "" {
		return false
	}
	if pid == os.Getpid() {
		hwnd, err := strconv.ParseUint(windowID, 10, 64)
		if err == nil {
			err = woxui.NavigateNativeFileDialog(uintptr(hwnd), targetPath)
		}
		if err != nil {
			util.GetLogger().Error(ctx, "navigate own file dialog: "+err.Error())
		}
		return err == nil
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
