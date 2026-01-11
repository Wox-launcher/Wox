package updater

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"wox/util"
)

func init() {
	applyUpdaterInstance = &LinuxUpdater{}
}

type LinuxUpdater struct{}

func getExecutablePath() (string, error) {
	if appImagePath := os.Getenv("APPIMAGE"); appImagePath != "" {
		return appImagePath, nil
	}
	return os.Executable()
}

func (u *LinuxUpdater) ApplyUpdate(ctx context.Context, pid int, oldPath, newPath string, progress ApplyUpdateProgressCallback) error {
	shellPath := filepath.Join(util.GetLocation().GetOthersDirectory(), "linux_update.sh")
	if _, statErr := os.Stat(shellPath); statErr != nil {
		return fmt.Errorf("failed to find linux update script: %w", statErr)
	}

	// Execute the shell script
	reportApplyProgress(progress, ApplyUpdateStageRestarting)
	util.GetLogger().Info(ctx, "starting Linux update process")
	logPath := filepath.Join(util.GetLocation().GetLogDirectory(), "update.log")
	cmd := exec.Command("bash", shellPath, fmt.Sprintf("%d", pid), oldPath, newPath, logPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start update process: %w", err)
	}

	// Exit the application
	util.GetLogger().Info(ctx, "exiting application for update to proceed")
	os.Exit(0)

	return nil // This line will never be reached due to os.Exit(0)
}

func cleanupBackupExecutable(_ context.Context) {
}
