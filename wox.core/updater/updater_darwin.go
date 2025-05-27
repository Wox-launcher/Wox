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

func (u *MacOSUpdater) ApplyUpdate(ctx context.Context, pid int, oldPath, newPath string) error {
	updateLogFile := filepath.Join(util.GetLocation().GetLogDirectory(), "update.log")

	util.GetLogger().Info(ctx, fmt.Sprintf("Processing DMG file: %s", newPath))
	extractedAppPath, err := extractAppFromDMG(ctx, newPath)
	if err != nil {
		return fmt.Errorf("failed to extract app from DMG: %w", err)
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("App extracted to: %s", extractedAppPath))

	// Create a shell script that will wait for the app to exit and then copy the extracted app
	shellContent := fmt.Sprintf(
		`#!/bin/bash

# Log file setup
LOG_FILE="%s"
LOG_TIMESTAMP=$(date "+%%Y-%%m-%%d %%H:%%M:%%S")

# Log function
log() {
  echo "$LOG_TIMESTAMP $1" >> "$LOG_FILE"
  echo "$1"
}

log "Update process started for version %s"
log "Extracted app path: %s"

# Wait for the current app to exit
log "Waiting for application with PID %d to exit..."
WAIT_COUNT=0

# Simple function to check if the PID is still running
is_process_running() {
  ps -p %d > /dev/null 2>&1
  return $?
}

# Wait for the process to exit
while is_process_running; do
  # Log every 5 seconds to show progress
  if [ $((WAIT_COUNT %% 5)) -eq 0 ]; then
    log "Still waiting for application to exit after ${WAIT_COUNT}s"
  fi

  # After 30 seconds, force continue
  if [ $WAIT_COUNT -eq 30 ]; then
    log "WARNING: Waited for 30 seconds. Forcing continue."
    break
  fi

  sleep 1
  WAIT_COUNT=$((WAIT_COUNT + 1))
done

log "Application has exited or timeout reached after ${WAIT_COUNT}s"

# Now that the app has exited, copy the extracted app to Applications
APP_PATH="%s"
APP_NAME=$(basename "$APP_PATH")
log "Copying $APP_NAME to /Applications/"

# Remove existing app if it exists
if [ -d "/Applications/$APP_NAME" ]; then
  log "Removing existing app: /Applications/$APP_NAME"
  rm -rf "/Applications/$APP_NAME"
  if [ $? -ne 0 ]; then
    log "Failed to remove existing app, trying with sudo"
    sudo rm -rf "/Applications/$APP_NAME"
    if [ $? -ne 0 ]; then
      log "ERROR: Failed to remove existing app even with sudo"
      exit 1
    fi
  fi
fi

# Copy the app
log "Copying app to Applications folder"
cp -R "$APP_PATH" "/Applications/"
if [ $? -ne 0 ]; then
  log "Failed to copy app, trying with sudo"
  sudo cp -R "$APP_PATH" "/Applications/"
  if [ $? -ne 0 ]; then
    log "ERROR: Failed to copy app to Applications"
    exit 1
  fi
fi

# Verify the app was copied
if [ ! -d "/Applications/$APP_NAME" ]; then
  log "ERROR: App was not copied to Applications"
  exit 1
fi

log "App copied successfully to Applications folder"

# Clean up the temporary directory
log "Cleaning up temporary directory"
rm -rf "$(dirname "$APP_PATH")"

# Open the new app
log "Opening new application: /Applications/$APP_NAME"
open "/Applications/$APP_NAME" || open -a "/Applications/$APP_NAME"

# Clean up
log "Cleaning up update script"
log "Update process completed successfully"
rm -f "$0"
`,
		updateLogFile, currentUpdateInfo.LatestVersion, extractedAppPath, pid, pid, extractedAppPath,
	)

	// Write the shell script
	shellPath := filepath.Join(filepath.Dir(newPath), "update.sh")
	if err := os.WriteFile(shellPath, []byte(shellContent), 0755); err != nil {
		return fmt.Errorf("failed to create update shell script: %w", err)
	}

	// Execute the shell script
	util.GetLogger().Info(ctx, "starting update process")
	cmd := exec.Command("bash", shellPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start update process: %w", err)
	}

	// Exit the application
	os.Exit(0)
	return nil // This line will never be reached due to os.Exit(0)
}
