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

const macosUpdateScript = `
LOG_FILE="$1"
VERSION="$2"
APP_PATH="$3"
PID="$4"
OLD_PATH="$5"

if [ -z "$LOG_FILE" ]; then
  LOG_FILE="/tmp/wox_update.log"
fi

if [ -z "$APP_PATH" ] || [ -z "$PID" ]; then
  echo "$(date "+%Y-%m-%d %H:%M:%S") missing required args" >> "$LOG_FILE"
  exit 1
fi

log() {
  local now
  now=$(date "+%Y-%m-%d %H:%M:%S")
  echo "$now $1" >> "$LOG_FILE"
  echo "$1"
}

log "Update process started for version $VERSION"
log "Args: log=$LOG_FILE app=$APP_PATH pid=$PID old=$OLD_PATH"
log "Extracted app path: $APP_PATH"

log "Waiting for application with PID $PID to exit..."
WAIT_COUNT=0

is_process_running() {
  ps -p "$PID" > /dev/null 2>&1
  return $?
}

while is_process_running; do
  if [ $((WAIT_COUNT % 5)) -eq 0 ]; then
    log "Still waiting for application to exit after ${WAIT_COUNT}s"
  fi

  if [ $WAIT_COUNT -eq 30 ]; then
    log "WARNING: Waited for 30 seconds. Forcing continue."
    break
  fi

  sleep 1
  WAIT_COUNT=$((WAIT_COUNT + 1))
done

log "Application has exited or timeout reached after ${WAIT_COUNT}s"

APP_NAME=$(basename "$APP_PATH")
TARGET_DIR=""
TARGET_APP_NAME="$APP_NAME"
TARGET_APP_PATH=""

if [ -n "$OLD_PATH" ]; then
  if [[ "$OLD_PATH" == *".app/"* ]]; then
    OLD_APP_PATH="${OLD_PATH%%.app/*}.app"
    TARGET_APP_NAME=$(basename "$OLD_APP_PATH")
    TARGET_DIR=$(dirname "$OLD_APP_PATH")
  elif [[ "$OLD_PATH" == *.app ]]; then
    TARGET_APP_NAME=$(basename "$OLD_PATH")
    TARGET_DIR=$(dirname "$OLD_PATH")
  else
    TARGET_DIR=$(dirname "$OLD_PATH")
  fi
fi

if [ -z "$TARGET_DIR" ]; then
  TARGET_DIR="/Applications"
fi

TARGET_APP_PATH="$TARGET_DIR/$TARGET_APP_NAME"
log "Target app path: $TARGET_APP_PATH"

log "Copying $APP_NAME to $TARGET_DIR/"

if [ -d "$TARGET_APP_PATH" ]; then
  log "Removing existing app: $TARGET_APP_PATH"
  rm -rf "$TARGET_APP_PATH"
  if [ $? -ne 0 ]; then
    log "Failed to remove existing app, trying with sudo"
    sudo rm -rf "$TARGET_APP_PATH"
    if [ $? -ne 0 ]; then
      log "ERROR: Failed to remove existing app even with sudo"
      exit 1
    fi
  fi
fi

log "Copying app to target folder"
cp -R "$APP_PATH" "$TARGET_DIR/"
if [ $? -ne 0 ]; then
  log "Failed to copy app, trying with sudo"
  sudo cp -R "$APP_PATH" "$TARGET_DIR/"
  if [ $? -ne 0 ]; then
    log "ERROR: Failed to copy app to target folder"
    exit 1
  fi
fi

if [ "$APP_NAME" != "$TARGET_APP_NAME" ] && [ -d "$TARGET_DIR/$APP_NAME" ]; then
  rm -rf "$TARGET_APP_PATH"
  mv "$TARGET_DIR/$APP_NAME" "$TARGET_APP_PATH"
fi

if [ ! -d "$TARGET_APP_PATH" ]; then
  log "ERROR: App was not copied to target folder"
  exit 1
fi

log "App copied successfully to target folder"

log "Cleaning up temporary directory"
rm -rf "$(dirname "$APP_PATH")"

log "Opening new application: $TARGET_APP_PATH"
open "$TARGET_APP_PATH" --args --updated

log "Update process completed successfully"
`

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

	// Execute the shell script
	reportApplyProgress(progress, ApplyUpdateStageReplacing)
	util.GetLogger().Info(ctx, "starting update process")
	cmd := exec.Command(
		"bash", "-s", "--",
		updateLogFile,
		currentUpdateInfo.LatestVersion,
		extractedAppPath,
		fmt.Sprintf("%d", pid),
		oldPath,
	)
	cmd.Stdin = strings.NewReader(macosUpdateScript)
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
