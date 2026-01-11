package updater

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"wox/util"
)

func init() {
	applyUpdaterInstance = &MacOSUpdater{}
}

type MacOSUpdater struct{}

func getExecutablePath() (string, error) {
	return os.Executable()
}

// extractAppFromDMG extracts the app from a DMG file to a temporary directory
// Returns the path to the extracted app
func extractAppFromDMG(ctx context.Context, dmgPath string) (string, error) {
	util.GetLogger().Info(ctx, fmt.Sprintf("Extracting app from DMG file: %s", dmgPath))

	// Check if DMG file exists
	if _, err := os.Stat(dmgPath); err != nil {
		return "", fmt.Errorf("DMG file does not exist: %w", err)
	}

	// Create a temporary directory to store the extracted app
	tempDir, err := os.MkdirTemp("", "wox_update_*")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}

	// Mount the DMG file
	cmd := exec.Command("hdiutil", "attach", "-nobrowse", "-mountpoint", tempDir, dmgPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("failed to mount DMG: %s, stderr: %s", err, stderr.String())
	}

	// Find the app in the mounted DMG
	var appPath string
	err = filepath.WalkDir(tempDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() && strings.HasSuffix(path, ".app") {
			appPath = path
			return filepath.SkipAll
		}

		return nil
	})

	if err != nil {
		exec.Command("hdiutil", "detach", tempDir, "-force").Run()
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("error searching for app: %w", err)
	}

	if appPath == "" {
		exec.Command("hdiutil", "detach", tempDir, "-force").Run()
		os.RemoveAll(tempDir)
		return "", errors.New("no .app found in DMG")
	}

	util.GetLogger().Info(ctx, fmt.Sprintf("Found app at: %s", appPath))

	// Create a new temporary directory for the extracted app
	extractDir, err := os.MkdirTemp("", "wox_app_*")
	if err != nil {
		exec.Command("hdiutil", "detach", tempDir, "-force").Run()
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("failed to create extraction directory: %w", err)
	}

	// Copy the app to the extraction directory
	appName := filepath.Base(appPath)
	extractedAppPath := filepath.Join(extractDir, appName)

	util.GetLogger().Info(ctx, fmt.Sprintf("Copying app to temporary directory: %s", extractedAppPath))
	cmd = exec.Command("cp", "-R", appPath, extractDir)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		exec.Command("hdiutil", "detach", tempDir, "-force").Run()
		os.RemoveAll(tempDir)
		os.RemoveAll(extractDir)
		return "", fmt.Errorf("failed to copy app: %s, stderr: %s", err, stderr.String())
	}

	// Unmount the DMG
	if err := exec.Command("hdiutil", "detach", tempDir, "-force").Run(); err != nil {
		util.GetLogger().Info(ctx, fmt.Sprintf("Warning: failed to unmount DMG: %s", err))
	}

	// Clean up the mount point
	os.RemoveAll(tempDir)

	util.GetLogger().Info(ctx, fmt.Sprintf("App extracted successfully to: %s", extractedAppPath))
	return extractedAppPath, nil
}

func (u *MacOSUpdater) ApplyUpdate(ctx context.Context, pid int, oldPath, newPath string, progress ApplyUpdateProgressCallback) error {
	updateLogFile := filepath.Join(util.GetLocation().GetLogDirectory(), "update.log")

	reportApplyProgress(progress, ApplyUpdateStageExtracting)
	util.GetLogger().Info(ctx, fmt.Sprintf("Processing DMG file: %s", newPath))
	extractedAppPath, err := extractAppFromDMG(ctx, newPath)
	if err != nil {
		return fmt.Errorf("failed to extract app from DMG: %w", err)
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("App extracted to: %s", extractedAppPath))

	shellPath := filepath.Join(util.GetLocation().GetOthersDirectory(), "macos_update.sh")
	if _, statErr := os.Stat(shellPath); statErr != nil {
		return fmt.Errorf("failed to find macos update script: %w", statErr)
	}

	// Execute the shell script
	reportApplyProgress(progress, ApplyUpdateStageReplacing)
	util.GetLogger().Info(ctx, "starting update process")
	cmd := exec.Command(
		"bash",
		shellPath,
		updateLogFile,
		currentUpdateInfo.LatestVersion,
		extractedAppPath,
		fmt.Sprintf("%d", pid),
		oldPath,
	)
	reportApplyProgress(progress, ApplyUpdateStageRestarting)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start update process: %w", err)
	}

	// Exit the application
	os.Exit(0)
	return nil // This line will never be reached due to os.Exit(0)
}

func cleanupBackupExecutable(_ context.Context) {
}
