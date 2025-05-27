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

func (u *LinuxUpdater) ApplyUpdate(ctx context.Context, pid int, oldPath, newPath string) error {
	// Create a shell script to replace the executable after the app exits
	shellContent := fmt.Sprintf(
		"#!/bin/bash\n"+
			"# Wait for the application to exit\n"+
			"while ps -p %d > /dev/null 2>&1 || pgrep -f $(basename %s) > /dev/null 2>&1; do\n"+
			"  sleep 1\n"+
			"done\n"+
			"# Replace the executable\n"+
			"cp %s %s\n"+
			"chmod +x %s\n"+
			"# Start the new version\n"+
			"%s &\n"+
			"# Clean up\n"+
			"rm $0\n",
		pid, oldPath, newPath, oldPath, oldPath, oldPath,
	)

	// Write the shell script
	shellPath := filepath.Join(filepath.Dir(newPath), "update.sh")
	if err := os.WriteFile(shellPath, []byte(shellContent), 0755); err != nil {
		return fmt.Errorf("failed to create update shell script: %w", err)
	}

	// Execute the shell script
	util.GetLogger().Info(ctx, "starting Linux update process")
	cmd := exec.Command("bash", shellPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start update process: %w", err)
	}

	// Exit the application
	util.GetLogger().Info(ctx, "exiting application for update to proceed")
	os.Exit(0)

	return nil // This line will never be reached due to os.Exit(0)
}
