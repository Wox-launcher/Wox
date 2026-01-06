package updater

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	batchContent := fmt.Sprintf(
		"@echo off\n"+
			":loop\n"+
			"tasklist | find /i \"wox.exe\" >nul 2>&1\n"+
			"if errorlevel 1 (\n"+
			"  move /y \"%s\" \"%s\"\n"+
			"  start \"\" \"%s\"\n"+
			"  del %%0\n"+
			") else (\n"+
			"  timeout /t 1 /nobreak >nul\n"+
			"  goto loop\n"+
			")\n",
		newPath, oldPath, oldPath,
	)

	// Write the batch file
	batchPath := filepath.Join(filepath.Dir(newPath), "update.bat")
	if err := os.WriteFile(batchPath, []byte(batchContent), 0644); err != nil {
		return fmt.Errorf("failed to create update batch file: %w", err)
	}

	// Execute the batch file
	util.GetLogger().Info(ctx, "starting Windows update process")
	if _, err := shell.Run("cmd", "/c", "start", "", batchPath); err != nil {
		return fmt.Errorf("failed to start update process: %w", err)
	}

	// Exit the application
	util.GetLogger().Info(ctx, "exiting application for update to proceed")
	os.Exit(0)

	return nil // This line will never be reached due to os.Exit(0)
}
