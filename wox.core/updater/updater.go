package updater

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
	"wox/setting"
	"wox/util"

	"github.com/Masterminds/semver/v3"
)

type UpdateStatus string

const (
	UpdateStatusNone        UpdateStatus = "none"        // No update available or checked
	UpdateStatusAvailable   UpdateStatus = "available"   // Update is available but not downloaded
	UpdateStatusDownloading UpdateStatus = "downloading" // Update is being downloaded
	UpdateStatusReady       UpdateStatus = "ready"       // Update is downloaded and ready to install
	UpdateStatusError       UpdateStatus = "error"       // Error occurred during update
)

var currentUpdateInfo = UpdateInfo{Status: UpdateStatusNone} // global variable to store update info

const versionManifestUrl = "https://raw.githubusercontent.com/Wox-launcher/Wox/master/updater.json"

type VersionManifest struct {
	Version string

	MacArm64DownloadUrl string
	MacArm64Checksum    string

	MacAmd64DownloadUrl string
	MacAmd64Checksum    string

	WindowsDownloadUrl string
	WindowsChecksum    string

	LinuxDownloadUrl string
	LinuxChecksum    string

	ReleaseNotes string // newline separated with \n
}

type UpdateInfo struct {
	CurrentVersion string
	LatestVersion  string
	ReleaseNotes   string
	DownloadUrl    string
	Checksum       string // Checksum for verification
	Status         UpdateStatus
	UpdateError    error
	DownloadedPath string
	HasUpdate      bool // Whether there is an update available
}

// StartAutoUpdateChecker starts a background task that periodically checks for updates
func StartAutoUpdateChecker(ctx context.Context) {
	util.Go(ctx, "auto-update-checker", func() {
		newCtx := util.NewTraceContext()
		CheckForUpdates(newCtx)
		for range time.NewTicker(time.Hour * 6).C {
			CheckForUpdates(newCtx)
		}
	})
}

func CheckForUpdates(ctx context.Context) {
	util.GetLogger().Info(ctx, "start checking for updates")

	setting := setting.GetSettingManager().GetWoxSetting(ctx)
	if setting != nil && !setting.EnableAutoUpdate {
		util.GetLogger().Info(ctx, "auto update is disabled, skipping")
		currentUpdateInfo.Status = UpdateStatusNone
		currentUpdateInfo.HasUpdate = false
		currentUpdateInfo.DownloadedPath = ""
		currentUpdateInfo.UpdateError = nil
		return
	}

	if currentUpdateInfo.Status == UpdateStatusDownloading {
		util.GetLogger().Info(ctx, "update is downloading, skipping")
		return
	}

	if currentUpdateInfo.Status == UpdateStatusReady && currentUpdateInfo.DownloadedPath != "" {
		util.GetLogger().Info(ctx, "update is ready to install, skipping")
		return
	}

	currentUpdateInfo = parseLatestVersion(ctx)
	if !currentUpdateInfo.HasUpdate {
		util.GetLogger().Info(ctx, "no update available, skipping")
		return
	}

	downloadUpdate(ctx)
}

func parseLatestVersion(ctx context.Context) UpdateInfo {
	util.GetLogger().Info(ctx, "start parsing lastest version")
	latestVersion, err := getLatestVersion(ctx)
	if err != nil {
		util.GetLogger().Error(ctx, err.Error())
		return UpdateInfo{
			Status:      UpdateStatusError,
			UpdateError: err,
		}
	}

	// compare with current version
	existingVersion, existingErr := semver.NewVersion(CURRENT_VERSION)
	if existingErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to parse current version: %s", existingErr.Error()))
		return UpdateInfo{
			Status:      UpdateStatusError,
			UpdateError: fmt.Errorf("failed to parse current version: %s", existingErr.Error()),
		}
	}
	newVersion, newErr := semver.NewVersion(latestVersion.Version)
	if newErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to parse latest version: %s", newErr.Error()))
		return UpdateInfo{
			Status:      UpdateStatusError,
			UpdateError: fmt.Errorf("failed to parse latest version: %s", newErr.Error()),
		}
	}

	info := UpdateInfo{
		CurrentVersion: existingVersion.String(),
		LatestVersion:  newVersion.String(),
		ReleaseNotes:   latestVersion.ReleaseNotes,
	}

	if newVersion.LessThan(existingVersion) || newVersion.Equal(existingVersion) {
		util.GetLogger().Info(ctx, fmt.Sprintf("no new version available, current: %s, latest: %s", existingVersion.String(), newVersion.String()))
		info.Status = UpdateStatusNone
		info.HasUpdate = false
		return info
	}

	util.GetLogger().Info(ctx, fmt.Sprintf("new version available, current: %s, latest: %s", existingVersion.String(), newVersion.String()))

	var downloadUrl string
	var checksum string
	if util.IsMacOS() {
		if util.IsArm64() {
			downloadUrl = latestVersion.MacArm64DownloadUrl
			checksum = latestVersion.MacArm64Checksum
		} else {
			downloadUrl = latestVersion.MacAmd64DownloadUrl
			checksum = latestVersion.MacAmd64Checksum
		}
	}
	if util.IsWindows() {
		downloadUrl = latestVersion.WindowsDownloadUrl
		checksum = latestVersion.WindowsChecksum
	}
	if util.IsLinux() {
		downloadUrl = latestVersion.LinuxDownloadUrl
		checksum = latestVersion.LinuxChecksum
	}
	if downloadUrl == "" {
		util.GetLogger().Error(ctx, "no download url found")
		return UpdateInfo{
			Status:      UpdateStatusError,
			UpdateError: errors.New("no download url found"),
		}
	}

	info.DownloadUrl = downloadUrl
	info.Checksum = checksum
	info.Status = UpdateStatusAvailable
	info.HasUpdate = true
	return info
}

func getLatestVersion(ctx context.Context) (VersionManifest, error) {
	body, err := util.HttpGet(ctx, versionManifestUrl)
	if err != nil {
		return VersionManifest{}, fmt.Errorf("failed to download version manifest file: %w", err)
	}

	var manifest VersionManifest
	if unmarshalErr := json.Unmarshal(body, &manifest); unmarshalErr != nil {
		return VersionManifest{}, fmt.Errorf("failed to unmarshal version manifest: %w", unmarshalErr)
	}

	return manifest, nil
}

func GetUpdateInfo() UpdateInfo {
	return currentUpdateInfo
}

func downloadUpdate(ctx context.Context) {
	if currentUpdateInfo.DownloadUrl == "" {
		util.GetLogger().Error(ctx, "no download URL provided")
		return
	}

	if currentUpdateInfo.Checksum == "" {
		util.GetLogger().Error(ctx, "no checksum provided")
		return
	}

	// Check if the same version has already been downloaded
	fileName := fmt.Sprintf("wox-%s", currentUpdateInfo.LatestVersion)
	if util.IsWindows() {
		fileName += ".exe"
	} else if util.IsMacOS() {
		fileName += ".dmg"
	}
	downloadPath := filepath.Join(util.GetLocation().GetUpdatesDirectory(), fileName)

	// If file already exists, verify checksum
	if _, err := os.Stat(downloadPath); err == nil {
		util.GetLogger().Info(ctx, "found existing downloaded update, verifying checksum")
		fileChecksum, checksumErr := calculateFileChecksum(downloadPath)
		if checksumErr == nil && fileChecksum == currentUpdateInfo.Checksum {
			// Checksum matches, mark as ready to install
			currentUpdateInfo.Status = UpdateStatusReady
			currentUpdateInfo.DownloadedPath = downloadPath
			util.GetLogger().Info(ctx, "existing update verified and ready to install")
			return
		} else {
			// Checksum doesn't match or verification failed, delete file and download again
			util.GetLogger().Info(ctx, "existing update invalid or corrupted, will download again")
			os.Remove(downloadPath)
		}
	}

	currentUpdateInfo.Status = UpdateStatusDownloading

	util.Go(ctx, "download-update", func() {
		util.GetLogger().Info(ctx, fmt.Sprintf("downloading update from %s to %s", currentUpdateInfo.DownloadUrl, downloadPath))
		err := util.HttpDownload(ctx, currentUpdateInfo.DownloadUrl, downloadPath)
		if err != nil {
			currentUpdateInfo.Status = UpdateStatusError
			currentUpdateInfo.UpdateError = fmt.Errorf("failed to download update: %w", err)
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to download update: %s", err.Error()))
			return
		}

		util.GetLogger().Info(ctx, "verifying checksum")
		fileChecksum, checksumErr := calculateFileChecksum(downloadPath)
		if checksumErr != nil {
			currentUpdateInfo.Status = UpdateStatusError
			currentUpdateInfo.UpdateError = fmt.Errorf("failed to calculate checksum: %w", checksumErr)
			return
		}
		if fileChecksum != currentUpdateInfo.Checksum {
			currentUpdateInfo.Status = UpdateStatusError
			currentUpdateInfo.UpdateError = fmt.Errorf("checksum verification failed: expected %s, got %s", currentUpdateInfo.Checksum, fileChecksum)
			// Remove the invalid file
			os.Remove(downloadPath)
			return
		}
		util.GetLogger().Info(ctx, "checksum verification passed")

		currentUpdateInfo.Status = UpdateStatusReady
		currentUpdateInfo.DownloadedPath = downloadPath

		util.GetLogger().Info(ctx, "update downloaded and ready to install")
	})
}

// ApplyUpdate applies the downloaded update
// This should be called when the user confirms they want to update
func ApplyUpdate(ctx context.Context) error {
	if currentUpdateInfo.Status != UpdateStatusReady || currentUpdateInfo.DownloadedPath == "" {
		return errors.New("no update ready to apply")
	}
	filePath := currentUpdateInfo.DownloadedPath

	// Make the file executable (for Unix systems)
	if !util.IsWindows() {
		if err := os.Chmod(filePath, 0755); err != nil {
			return fmt.Errorf("failed to make update executable: %w", err)
		}
	}

	// Get the current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable path: %w", err)
	}

	// On Windows, we can't replace the running executable directly
	// So we need to use a batch file or similar approach to replace it after the app exits
	if util.IsWindows() {
		// Create a batch file to replace the executable after the app exits
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
			filePath, execPath, execPath,
		)

		batchPath := filepath.Join(filepath.Dir(filePath), "update.bat")
		if err := os.WriteFile(batchPath, []byte(batchContent), 0644); err != nil {
			return fmt.Errorf("failed to create update batch file: %w", err)
		}

		// Execute the batch file
		util.GetLogger().Info(ctx, "starting update process")
		cmd := exec.Command("cmd", "/c", "start", "", batchPath)
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start update process: %w", err)
		}

		// Exit the application
		os.Exit(0)
	} else if util.IsMacOS() {
		// On macOS, we need to mount the DMG file, copy the app to Applications, and then restart
		// Create a shell script to handle the DMG installation after the app exits

		// Get log directory for update logs
		logDir := util.GetLocation().GetLogDirectory()
		updateLogFile := filepath.Join(logDir, "update.log")

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

# Wait for the current app to exit
log "Waiting for current application to exit..."
log "Looking for Wox application process"
WAIT_COUNT=0

# Get the actual executable name and path
EXEC_NAME=$(basename %s)
EXEC_PATH=%s
log "Executable name: $EXEC_NAME"
log "Executable path: $EXEC_PATH"

# Function to check if the real Wox application is running
is_wox_running() {
  # More specific search to avoid matching update scripts and tail commands
  # Look for the actual executable in Applications folder or the original path
  if pgrep -f "/Applications/Wox.app/Contents/MacOS/wox" > /dev/null 2>&1; then
    return 0 # Found
  fi

  if [ -f "$EXEC_PATH" ] && pgrep -f "$EXEC_PATH" > /dev/null 2>&1; then
    return 0 # Found
  fi

  # Check for processes with Wox in the name that aren't update scripts or tail
  POSSIBLE_WOX=$(ps aux | grep -i "wox" | grep -v "update.sh" | grep -v "tail" | grep -v "grep")
  if [ -n "$POSSIBLE_WOX" ]; then
    log "Found possible Wox processes:"
    echo "$POSSIBLE_WOX" >> "$LOG_FILE"
    return 0 # Found
  fi

  return 1 # Not found
}

while is_wox_running; do
  # Log every 5 seconds to show progress
  if [ $((WAIT_COUNT %% 5)) -eq 0 ]; then
    log "Still waiting for Wox application to exit after ${WAIT_COUNT}s"
    log "Current processes with 'wox' in name (excluding update scripts and grep):"
    ps aux | grep -i "wox" | grep -v "update.sh" | grep -v "tail" | grep -v "grep" >> "$LOG_FILE"
  fi

  # After 30 seconds, provide more detailed information
  if [ $WAIT_COUNT -eq 30 ]; then
    log "Waiting for a long time (30s). Detailed process information:"
    ps aux | grep -i "wox" | grep -v "grep" >> "$LOG_FILE"
    log "All processes for current user:"
    ps -u $(whoami) >> "$LOG_FILE"
  fi

  # After 60 seconds, force continue
  if [ $WAIT_COUNT -eq 60 ]; then
    log "WARNING: Waited for 60 seconds. Forcing continue."
    log "The update will proceed but may not complete properly if Wox is still running."
    break
  fi

  sleep 1
  WAIT_COUNT=$((WAIT_COUNT + 1))
done

log "Wox application has exited or timeout reached after ${WAIT_COUNT}s"

# Check if DMG file exists
log "Checking if DMG file exists: %s"
if [ ! -f "%s" ]; then
  log "ERROR: DMG file does not exist"
  log "Current directory: $(pwd)"
  log "Directory listing:"
  ls -la "$(dirname "%s")" >> "$LOG_FILE"
  exit 1
fi
log "DMG file exists and has size: $(du -h "%s" | cut -f1)"
log "DMG file details:"
file "%s" >> "$LOG_FILE"

# Mount the DMG file
log "Mounting DMG file: %s"
log "Using hdiutil attach command"
MOUNT_OUTPUT=$(hdiutil attach -nobrowse -verbose "%s" 2>&1)
log "Full mount output:"
echo "$MOUNT_OUTPUT" >> "$LOG_FILE"

# Try to parse the mount point
VOLUME=$(echo "$MOUNT_OUTPUT" | tail -n1 | awk '{print $NF}')
log "Parsed volume path: $VOLUME"

# Verify if volume path is valid
if [ -z "$VOLUME" ]; then
  log "ERROR: Failed to parse volume path from mount output"
  log "Trying alternative parsing method"
  VOLUME=$(echo "$MOUNT_OUTPUT" | grep "mounted at" | sed 's/.*mounted at //g' | tr -d '\n')
  log "Alternative parsed volume path: $VOLUME"
fi

if [ -z "$VOLUME" ]; then
  log "ERROR: All parsing methods failed to identify mount point"
  log "Listing all volumes to check if DMG was mounted:"
  ls -la /Volumes/ >> "$LOG_FILE"
  log "Checking for Wox-related volumes:"
  find /Volumes -name "*Wox*" -o -name "*wox*" >> "$LOG_FILE"
  exit 1
fi

# Verify the volume exists
if [ ! -d "$VOLUME" ]; then
  log "ERROR: Parsed volume path does not exist: $VOLUME"
  log "Listing all volumes:"
  ls -la /Volumes/ >> "$LOG_FILE"
  exit 1
fi

log "DMG successfully mounted at: $VOLUME"
log "Volume contents:"
ls -la "$VOLUME" >> "$LOG_FILE"

# Find the .app in the mounted volume
log "Searching for .app in mounted volume: $VOLUME"
log "Listing contents of mounted volume:"
ls -la "$VOLUME" >> "$LOG_FILE"

# Check if we're looking at the right volume
log "Checking all available volumes:"
ls -la /Volumes/ >> "$LOG_FILE"

# Look for Wox-related volumes specifically
log "Looking for Wox-related volumes:"
find /Volumes -name "*Wox*" -o -name "*wox*" -o -name "*Installer*" >> "$LOG_FILE"

# Try different search methods on the parsed volume
log "Trying find command with maxdepth 1 on $VOLUME"
APP_PATH=$(find "$VOLUME" -name "*.app" -maxdepth 1)
if [ -z "$APP_PATH" ]; then
  log "No app found with maxdepth 1, trying maxdepth 2"
  APP_PATH=$(find "$VOLUME" -name "*.app" -maxdepth 2)
fi

if [ -z "$APP_PATH" ]; then
  log "Still no app found, trying ls command"
  APP_PATH=$(ls -d "$VOLUME"/*.app 2>/dev/null)
fi

# If still not found, try searching in all Wox-related volumes
if [ -z "$APP_PATH" ]; then
  log "No app found in primary volume, checking all Wox-related volumes"
  for VOL in $(find /Volumes -name "*Wox*" -o -name "*wox*" -o -name "*Installer*" -type d); do
    log "Checking volume: $VOL"
    ls -la "$VOL" >> "$LOG_FILE"

    FOUND_APP=$(find "$VOL" -name "*.app" -maxdepth 2)
    if [ -n "$FOUND_APP" ]; then
      APP_PATH="$FOUND_APP"
      VOLUME="$VOL"
      log "Found app in alternative volume: $VOL"
      break
    fi
  done
fi

# If still not found, try a more aggressive search
if [ -z "$APP_PATH" ]; then
  log "Still no app found, trying more aggressive search in all volumes"
  for VOL in /Volumes/*; do
    if [ -d "$VOL" ]; then
      log "Checking volume: $VOL"
      FOUND_APP=$(find "$VOL" -name "*.app" -maxdepth 2 2>/dev/null)
      if [ -n "$FOUND_APP" ]; then
        APP_PATH="$FOUND_APP"
        VOLUME="$VOL"
        log "Found app in volume: $VOL"
        break
      fi
    fi
  done
fi

# Last resort: check if Wox.app is already in Applications
if [ -z "$APP_PATH" ] && [ -d "/Applications/Wox.app" ]; then
  log "No app found in DMG, but Wox.app exists in Applications"
  log "Using existing Wox.app as the target"
  APP_PATH="/Applications/Wox.app"
  # Skip the copy step later by setting a flag
  SKIP_COPY=1
elif [ -z "$APP_PATH" ]; then
  log "ERROR: No .app found in DMG or any volumes using multiple methods"
  log "Volume contents (recursive):"
  find "$VOLUME" -type d -maxdepth 3 >> "$LOG_FILE"
  log "All volumes contents:"
  ls -la /Volumes/ >> "$LOG_FILE"
  log "Detaching volume"
  hdiutil detach "$VOLUME" -force
  exit 1
fi

log "Found app at: $APP_PATH"

# Copy the app to Applications folder
APP_NAME=$(basename "$APP_PATH")
log "Application name: $APP_NAME"

# Check if we should skip the copy step (set earlier if app is already in Applications)
if [ "${SKIP_COPY:-0}" -eq 1 ]; then
  log "Skipping copy step as application is already in Applications folder"
else
  log "Checking if application already exists in Applications folder"
  if [ -d "/Applications/$APP_NAME" ]; then
    log "Existing application found, removing: /Applications/$APP_NAME"
    rm -rf "/Applications/$APP_NAME"
    if [ $? -ne 0 ]; then
      log "ERROR: Failed to remove existing application"
      log "Permissions for /Applications:"
      ls -la /Applications/ >> "$LOG_FILE"
      log "Trying with sudo (may prompt for password)"
      sudo rm -rf "/Applications/$APP_NAME"
      if [ $? -ne 0 ]; then
        log "ERROR: Failed to remove existing application even with sudo"
        if [ -n "$VOLUME" ] && [ "$VOLUME" != "/" ]; then
          log "Detaching volume"
          hdiutil detach "$VOLUME" -force
        fi
        exit 1
      fi
      log "Successfully removed existing application with sudo"
    else
      log "Successfully removed existing application"
    fi
  else
    log "No existing application found in Applications folder"
  fi

  log "Copying $APP_PATH to /Applications/"
  log "Using cp -R command"
  CP_OUTPUT=$(cp -Rv "$APP_PATH" /Applications/ 2>&1)
  CP_STATUS=$?
  log "Copy command output:"
  echo "$CP_OUTPUT" >> "$LOG_FILE"

  if [ $CP_STATUS -ne 0 ]; then
    log "ERROR: Failed to copy application to /Applications/ (status: $CP_STATUS)"
    log "Checking permissions:"
    ls -la "$APP_PATH" >> "$LOG_FILE"
    ls -la /Applications/ >> "$LOG_FILE"
    log "Trying with sudo (may prompt for password)"
    sudo cp -R "$APP_PATH" /Applications/
    if [ $? -ne 0 ]; then
      log "ERROR: Failed to copy application even with sudo"
      if [ -n "$VOLUME" ] && [ "$VOLUME" != "/" ]; then
        log "Detaching volume"
        hdiutil detach "$VOLUME" -force
      fi
      exit 1
    fi
    log "Successfully copied application with sudo"
  else
    log "Application copied successfully"
  fi
fi

log "Verifying application exists in Applications folder"
if [ ! -d "/Applications/$APP_NAME" ]; then
  log "ERROR: Application not found in /Applications"
  log "Applications directory contents:"
  ls -la /Applications/ >> "$LOG_FILE"
  if [ -n "$VOLUME" ] && [ "$VOLUME" != "/" ]; then
    log "Detaching volume"
    hdiutil detach "$VOLUME" -force
  fi
  exit 1
fi
log "Application verified in /Applications folder"

# Detach the DMG if it was mounted
if [ -n "$VOLUME" ] && [ "$VOLUME" != "/" ] && [ -d "$VOLUME" ]; then
  log "Detaching DMG volume: $VOLUME"
  DETACH_OUTPUT=$(hdiutil detach "$VOLUME" -force 2>&1)
  DETACH_STATUS=$?
  log "Detach output:"
  echo "$DETACH_OUTPUT" >> "$LOG_FILE"

  if [ $DETACH_STATUS -ne 0 ]; then
    log "WARNING: Failed to detach volume (status: $DETACH_STATUS)"
    log "This is not critical, continuing with update"
  else
    log "Successfully detached volume"
  fi
else
  log "No volume to detach or volume is not valid"
fi

# Open the new app
log "Opening new application: /Applications/$APP_NAME"
log "Using open command"
OPEN_OUTPUT=$(open "/Applications/$APP_NAME" 2>&1)
OPEN_STATUS=$?
log "Open command output:"
echo "$OPEN_OUTPUT" >> "$LOG_FILE"

if [ $OPEN_STATUS -ne 0 ]; then
  log "ERROR: Failed to open new application (status: $OPEN_STATUS)"
  log "Checking application bundle:"
  ls -la "/Applications/$APP_NAME" >> "$LOG_FILE"
  log "Checking application executable:"
  ls -la "/Applications/$APP_NAME/Contents/MacOS/" >> "$LOG_FILE"

  log "Trying alternative open method"
  OPEN_OUTPUT=$(open -a "/Applications/$APP_NAME" 2>&1)
  if [ $? -ne 0 ]; then
    log "ERROR: Alternative open method also failed"
    log "Application may need to be opened manually"
    # Don't exit with error as the update itself was successful
  else
    log "Alternative open method succeeded"
  fi
else
  log "New application opened successfully"
fi

# Check if application is running
sleep 2
log "Checking if application is running"
PS_OUTPUT=$(ps aux | grep -i "$APP_NAME" | grep -v grep)
log "Process check output:"
echo "$PS_OUTPUT" >> "$LOG_FILE"
if [ -z "$PS_OUTPUT" ]; then
  log "WARNING: Application does not appear to be running"
  log "User may need to open the application manually"
else
  log "Application appears to be running"
fi

# Clean up
log "Cleaning up update script"
log "Update process completed successfully"
rm -f "$0"
`,
			updateLogFile, currentUpdateInfo.LatestVersion, execPath, execPath, filePath, filePath, filePath, filePath, filePath, filePath, filePath,
		)

		shellPath := filepath.Join(filepath.Dir(filePath), "update.sh")
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
	} else {
		// On Linux, we can replace the executable and restart
		// Create a shell script to replace the executable after the app exits
		shellContent := fmt.Sprintf(
			"#!/bin/bash\n"+
				"while pgrep -f $(basename %s) > /dev/null; do\n"+
				"  sleep 1\n"+
				"done\n"+
				"cp %s %s\n"+
				"chmod +x %s\n"+
				"%s &\n"+
				"rm $0\n",
			execPath, filePath, execPath, execPath, execPath,
		)

		shellPath := filepath.Join(filepath.Dir(filePath), "update.sh")
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
	}

	return nil
}

// calculateFileChecksum calculates the MD5 checksum of a file
func calculateFileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file for checksum calculation: %w", err)
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to calculate checksum: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
