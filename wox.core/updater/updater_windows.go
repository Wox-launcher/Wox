package updater

import (
	"context"
	"fmt"
	"os"
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

	// if backup file exists, remove it
	if _, err := os.Stat(backupPath); err == nil {
		util.GetLogger().Info(ctx, "removing existing backup executable")
		if err := os.Remove(backupPath); err != nil {
			return fmt.Errorf("failed to remove existing backup executable: %w", err)
		}
	}

	util.GetLogger().Info(ctx, "replacing Windows executable in-place")
	if err := os.Rename(oldPath, backupPath); err != nil {
		return fmt.Errorf("failed to rename current executable: %w", err)
	} else {
		util.GetLogger().Info(ctx, "current executable renamed to backup successfully")
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

	util.GetLogger().Info(ctx, "removing backup executable")
	removeBackupErr := os.Remove(backupPath)
	if removeBackupErr != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to remove backup executable: %v", removeBackupErr))
	} else {
		util.GetLogger().Info(ctx, "backup executable removed successfully")
	}

	util.GetLogger().Info(ctx, "starting updated application")
	startCmd := fmt.Sprintf("timeout /t 1 /nobreak >nul & start \"\" \"%s\"", oldPath)
	if _, err := shell.Run("cmd", "/c", startCmd); err != nil {
		return fmt.Errorf("failed to start updated application: %w", err)
	}

	util.GetLogger().Info(ctx, "exiting application for update to proceed")
	os.Exit(0)

	return nil // This line will never be reached due to os.Exit(0)
}
