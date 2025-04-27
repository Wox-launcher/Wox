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
		checkForUpdates(newCtx)
		for range time.NewTicker(time.Hour * 6).C {
			checkForUpdates(newCtx)
		}
	})
}

func checkForUpdates(ctx context.Context) {
	util.GetLogger().Info(ctx, "start checking for updates")

	setting := setting.GetSettingManager().GetWoxSetting(ctx)
	if setting != nil && !setting.EnableAutoUpdate {
		util.GetLogger().Info(ctx, "auto update is disabled, skipping")
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
		// On macOS, we use a similar approach to Linux
		// Create a shell script to replace the executable after the app exits
		shellContent := fmt.Sprintf(
			"#!/bin/bash\n"+
				"while pgrep -f \"$(basename %s)\" > /dev/null; do\n"+
				"  sleep 1\n"+
				"done\n"+
				"cp %s %s\n"+
				"chmod +x %s\n"+
				"open %s\n"+
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
