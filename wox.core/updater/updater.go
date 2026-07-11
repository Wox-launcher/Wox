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

const stableVersionManifestUrl = "https://raw.githubusercontent.com/Wox-launcher/Wox/master/updater.json"
const betaVersionManifestUrl = "https://raw.githubusercontent.com/Wox-launcher/Wox/master/updater.beta.json"

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
	ReleaseChannel string
	ReleaseNotes   string
	DownloadUrl    string
	Checksum       string // Checksum for verification
	Status         UpdateStatus
	UpdateError    error
	DownloadedPath string
	HasUpdate      bool // Whether there is an update available
}

type UpdateChannelVersion struct {
	Channel       string
	LatestVersion string
	Error         string
}

type UpdateInfoCallback func(info UpdateInfo)
type versionManifestFetcher func(ctx context.Context, releaseChannel setting.ReleaseChannel) (VersionManifest, error)

type ApplyUpdateStage string

const (
	ApplyUpdateStagePreparing  ApplyUpdateStage = "preparing"
	ApplyUpdateStageExtracting ApplyUpdateStage = "extracting"
	ApplyUpdateStageReplacing  ApplyUpdateStage = "replacing"
	ApplyUpdateStageRestarting ApplyUpdateStage = "restarting"
)

type ApplyUpdateProgressCallback func(stage ApplyUpdateStage)

type applyUpdater interface {
	ApplyUpdate(ctx context.Context, pid int, oldPath, newPath string, progress ApplyUpdateProgressCallback) error
}

var applyUpdaterInstance applyUpdater

// StartAutoUpdateChecker starts a background task that periodically checks for updates
func StartAutoUpdateChecker(ctx context.Context) {
	util.Go(ctx, "auto-update-checker", func() {
		newCtx := util.NewTraceContext()
		CheckForUpdatesWithCallback(newCtx, nil)
		for range time.NewTicker(time.Hour * 6).C {
			CheckForUpdatesWithCallback(newCtx, nil)
		}
	})
}

func CheckForUpdates(ctx context.Context) {
	CheckForUpdatesWithCallback(ctx, nil)
}

func CheckForUpdatesWithCallback(ctx context.Context, callback UpdateInfoCallback) {
	util.GetLogger().Info(ctx, "start checking for updates")

	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	releaseChannel := setting.ReleaseChannelStable
	if woxSetting != nil {
		releaseChannel = woxSetting.ReleaseChannel.Get()
	}
	resetCurrentUpdateInfoForReleaseChannel(releaseChannel)

	if woxSetting != nil && !woxSetting.EnableAutoUpdate.Get() {
		util.GetLogger().Info(ctx, "auto update is disabled, skipping")
		currentUpdateInfo = UpdateInfo{
			CurrentVersion: CURRENT_VERSION,
			ReleaseChannel: string(setting.NormalizeReleaseChannel(string(releaseChannel))),
			Status:         UpdateStatusNone,
			HasUpdate:      false,
		}
		if callback != nil {
			callback(currentUpdateInfo)
		}
		return
	}

	if currentUpdateInfo.Status == UpdateStatusDownloading {
		util.GetLogger().Info(ctx, "update is downloading, skipping")
		if callback != nil {
			callback(currentUpdateInfo)
		}
		return
	}

	if currentUpdateInfo.Status == UpdateStatusReady && currentUpdateInfo.DownloadedPath != "" {
		util.GetLogger().Info(ctx, "update is ready to install, skipping")
		if callback != nil {
			callback(currentUpdateInfo)
		}
		return
	}

	currentUpdateInfo = parseLatestVersion(ctx, releaseChannel)
	if callback != nil {
		callback(currentUpdateInfo)
	}
	if !currentUpdateInfo.HasUpdate {
		util.GetLogger().Info(ctx, "no update available, skipping")
		return
	}

	downloadUpdate(ctx, callback)
}

func resetCurrentUpdateInfoForReleaseChannel(releaseChannel setting.ReleaseChannel) {
	normalizedChannel := setting.NormalizeReleaseChannel(string(releaseChannel))
	if currentUpdateInfo.ReleaseChannel == "" {
		currentUpdateInfo.ReleaseChannel = string(normalizedChannel)
		return
	}
	if currentUpdateInfo.ReleaseChannel == string(normalizedChannel) {
		return
	}

	currentUpdateInfo = UpdateInfo{
		ReleaseChannel: string(normalizedChannel),
		Status:         UpdateStatusNone,
	}
}

// ResetUpdateInfoForReleaseChannel clears cached update state after a user switches release channels.
func ResetUpdateInfoForReleaseChannel(releaseChannel setting.ReleaseChannel) {
	resetCurrentUpdateInfoForReleaseChannel(releaseChannel)
}

func parseLatestVersion(ctx context.Context, releaseChannel setting.ReleaseChannel) UpdateInfo {
	util.GetLogger().Info(ctx, "start parsing lastest version")
	latestVersion, err := getLatestVersion(ctx, releaseChannel)
	if err != nil {
		util.GetLogger().Error(ctx, err.Error())
		return UpdateInfo{
			ReleaseChannel: string(setting.NormalizeReleaseChannel(string(releaseChannel))),
			Status:         UpdateStatusError,
			UpdateError:    err,
		}
	}

	return buildUpdateInfoFromManifest(ctx, CURRENT_VERSION, releaseChannel, latestVersion)
}

func buildUpdateInfoFromManifest(ctx context.Context, currentVersion string, releaseChannel setting.ReleaseChannel, latestVersion VersionManifest) UpdateInfo {
	normalizedChannel := setting.NormalizeReleaseChannel(string(releaseChannel))

	// compare with current version
	existingVersion, existingErr := semver.NewVersion(currentVersion)
	if existingErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to parse current version: %s", existingErr.Error()))
		return UpdateInfo{
			ReleaseChannel: string(normalizedChannel),
			Status:         UpdateStatusError,
			UpdateError:    fmt.Errorf("failed to parse current version: %s", existingErr.Error()),
		}
	}
	newVersion, newErr := semver.NewVersion(latestVersion.Version)
	if newErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to parse latest version: %s", newErr.Error()))
		return UpdateInfo{
			ReleaseChannel: string(normalizedChannel),
			Status:         UpdateStatusError,
			UpdateError:    fmt.Errorf("failed to parse latest version: %s", newErr.Error()),
		}
	}

	info := UpdateInfo{
		CurrentVersion: existingVersion.String(),
		LatestVersion:  newVersion.String(),
		ReleaseChannel: string(normalizedChannel),
		ReleaseNotes:   latestVersion.ReleaseNotes,
	}

	if normalizedChannel == setting.ReleaseChannelStable && newVersion.Prerelease() != "" {
		util.GetLogger().Warn(ctx, fmt.Sprintf("stable update channel ignored prerelease manifest version: %s", newVersion.String()))
		info.LatestVersion = existingVersion.String()
		info.ReleaseNotes = ""
		info.Status = UpdateStatusNone
		info.HasUpdate = false
		return info
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

func manifestURLForReleaseChannel(releaseChannel setting.ReleaseChannel) string {
	switch setting.NormalizeReleaseChannel(string(releaseChannel)) {
	case setting.ReleaseChannelBeta:
		return betaVersionManifestUrl
	default:
		return stableVersionManifestUrl
	}
}

// getUpdateChannelVersions reads both channel manifests while keeping stable protected from prerelease manifests.
func getUpdateChannelVersions(ctx context.Context, fetchManifest versionManifestFetcher) []UpdateChannelVersion {
	channels := []setting.ReleaseChannel{setting.ReleaseChannelStable, setting.ReleaseChannelBeta}
	versions := make([]UpdateChannelVersion, 0, len(channels))
	for _, releaseChannel := range channels {
		normalizedChannel := setting.NormalizeReleaseChannel(string(releaseChannel))
		channelVersion := UpdateChannelVersion{Channel: string(normalizedChannel)}
		manifest, err := fetchManifest(ctx, normalizedChannel)
		if err != nil {
			channelVersion.Error = err.Error()
			versions = append(versions, channelVersion)
			continue
		}

		latestVersion, parseErr := semver.NewVersion(manifest.Version)
		if parseErr != nil {
			channelVersion.Error = fmt.Sprintf("failed to parse latest version: %s", parseErr.Error())
			versions = append(versions, channelVersion)
			continue
		}

		if normalizedChannel == setting.ReleaseChannelStable && latestVersion.Prerelease() != "" {
			channelVersion.Error = fmt.Sprintf("stable update channel ignored prerelease manifest version: %s", latestVersion.String())
			versions = append(versions, channelVersion)
			continue
		}

		channelVersion.LatestVersion = latestVersion.String()
		versions = append(versions, channelVersion)
	}
	return versions
}

// GetUpdateChannelVersions returns the latest manifest version for each update channel.
func GetUpdateChannelVersions(ctx context.Context) []UpdateChannelVersion {
	return getUpdateChannelVersions(ctx, getLatestVersion)
}

func getLatestVersion(ctx context.Context, releaseChannel setting.ReleaseChannel) (VersionManifest, error) {
	body, err := util.HttpGet(ctx, manifestURLForReleaseChannel(releaseChannel))
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

func downloadUpdate(ctx context.Context, callback UpdateInfoCallback) {
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
			if callback != nil {
				callback(currentUpdateInfo)
			}
			return
		} else {
			// Checksum doesn't match or verification failed, delete file and download again
			util.GetLogger().Info(ctx, "existing update invalid or corrupted, will download again")
			os.Remove(downloadPath)
		}
	}

	currentUpdateInfo.Status = UpdateStatusDownloading
	if callback != nil {
		callback(currentUpdateInfo)
	}

	util.Go(ctx, "download-update", func() {
		util.GetLogger().Info(ctx, fmt.Sprintf("downloading update from %s to %s", currentUpdateInfo.DownloadUrl, downloadPath))
		err := util.HttpDownload(ctx, currentUpdateInfo.DownloadUrl, downloadPath)
		if err != nil {
			currentUpdateInfo.Status = UpdateStatusError
			currentUpdateInfo.UpdateError = fmt.Errorf("failed to download update: %w", err)
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to download update: %s", err.Error()))
			if callback != nil {
				callback(currentUpdateInfo)
			}
			return
		}

		util.GetLogger().Info(ctx, "verifying checksum")
		fileChecksum, checksumErr := calculateFileChecksum(downloadPath)
		if checksumErr != nil {
			currentUpdateInfo.Status = UpdateStatusError
			currentUpdateInfo.UpdateError = fmt.Errorf("failed to calculate checksum: %w", checksumErr)
			if callback != nil {
				callback(currentUpdateInfo)
			}
			return
		}
		if fileChecksum != currentUpdateInfo.Checksum {
			currentUpdateInfo.Status = UpdateStatusError
			currentUpdateInfo.UpdateError = fmt.Errorf("checksum verification failed: expected %s, got %s", currentUpdateInfo.Checksum, fileChecksum)
			// Remove the invalid file
			os.Remove(downloadPath)
			if callback != nil {
				callback(currentUpdateInfo)
			}
			return
		}
		util.GetLogger().Info(ctx, "checksum verification passed")

		currentUpdateInfo.Status = UpdateStatusReady
		currentUpdateInfo.DownloadedPath = downloadPath
		if callback != nil {
			callback(currentUpdateInfo)
		}

		util.GetLogger().Info(ctx, "update downloaded and ready to install")
	})
}

// ApplyUpdate applies the downloaded update
// This should be called when the user confirms they want to update
func ApplyUpdate(ctx context.Context, progress ApplyUpdateProgressCallback) error {
	util.GetLogger().Info(ctx, "start applying update")

	if currentUpdateInfo.Status != UpdateStatusReady || currentUpdateInfo.DownloadedPath == "" {
		return errors.New("no update ready to apply")
	}
	newPath := currentUpdateInfo.DownloadedPath

	reportApplyProgress(progress, ApplyUpdateStagePreparing)

	// Get the current executable path (AppImage-aware on Linux)
	oldPath, err := getExecutablePath()
	if err != nil {
		return fmt.Errorf("failed to get current executable path: %w", err)
	}

	util.GetLogger().Info(ctx, fmt.Sprintf("applying update from %s to %s", oldPath, newPath))
	apllyErr := applyUpdaterInstance.ApplyUpdate(ctx, os.Getpid(), oldPath, newPath, progress)
	if apllyErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to apply update: %s", apllyErr.Error()))
		return apllyErr
	}

	return nil
}

func reportApplyProgress(callback ApplyUpdateProgressCallback, stage ApplyUpdateStage) {
	if callback == nil {
		return
	}
	callback(stage)
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
