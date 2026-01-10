package updater

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"wox/util"
	"wox/util/shell"
)

func init() {
	applyUpdaterInstance = &WindowsUpdater{}
}

type WindowsUpdater struct{}

func getExecutablePath() (string, error) {
	return os.Executable()
}

func (u *WindowsUpdater) ApplyUpdate(ctx context.Context, pid int, oldPath, newPath string) error {
	backupPath := fmt.Sprintf("%s.old", oldPath)

	util.GetLogger().Info(ctx, "replacing Windows executable in-place")
	if err := os.Rename(oldPath, backupPath); err != nil {
		return fmt.Errorf("failed to rename current executable: %w", err)
	} else {
		util.GetLogger().Info(ctx, "current executable renamed to backup successfully")
		hideBackupExecutable(ctx, backupPath)
	}

	util.GetLogger().Info(ctx, "moving new executable to current location")
	if err := os.Rename(newPath, oldPath); err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to move new executable to current location, attempting to restore backup: %v", err))

		restoreErr := os.Rename(backupPath, oldPath)
		if restoreErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to restore backup executable: %v", restoreErr))
		} else {
			util.GetLogger().Info(ctx, "backup executable restored successfully")
		}

		return fmt.Errorf("failed to move new executable to current location: %w", err)
	} else {
		util.GetLogger().Info(ctx, "new executable moved to current location successfully")
	}

	util.GetLogger().Info(ctx, "starting updated application")
	scriptPath := filepath.Join(util.GetLocation().GetOthersDirectory(), "windows_update_restart.cmd")
	if _, statErr := os.Stat(scriptPath); statErr != nil {
		return fmt.Errorf("failed to find windows update restart script: %w", statErr)
	}
	logPath := filepath.Join(util.GetLocation().GetLogDirectory(), "update_restart.log")
	if _, err := shell.Run("cmd.exe", "/c", "call", scriptPath, oldPath, logPath); err != nil {
		return fmt.Errorf("failed to start updated application: %w", err)
	}

	util.GetLogger().Info(ctx, "exiting application for update to proceed")
	os.Exit(0)

	return nil // This line will never be reached due to os.Exit(0)
}

func hideBackupExecutable(ctx context.Context, path string) {
	ptr, ptrErr := syscall.UTF16PtrFromString(path)
	if ptrErr != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to encode backup executable path: %v", ptrErr))
		return
	}

	attrs, attrErr := syscall.GetFileAttributes(ptr)
	if attrErr != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to get backup executable attributes: %v", attrErr))
		return
	}

	if attrs&syscall.FILE_ATTRIBUTE_HIDDEN != 0 {
		return
	}

	if err := syscall.SetFileAttributes(ptr, attrs|syscall.FILE_ATTRIBUTE_HIDDEN); err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to hide backup executable: %v", err))
	}
}
