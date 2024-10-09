package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"wox/util"

	"github.com/Masterminds/semver/v3"
)

const versionManifestUrl = "https://raw.githubusercontent.com/Wox-launcher/Wox/v2/updater.json"

var logger = util.GetLogger()

type VersionManifest struct {
	Version            string
	MacDownloadUrl     string
	WindowsDownloadUrl string
	LinuxDownloadUrl   string
	ReleaseNotes       string
}

type UpdateInfo struct {
	CurrentVersion string
	LatestVersion  string
	ReleaseNotes   string
	DownloadUrl    string
}

func CheckUpdate(ctx context.Context) (info UpdateInfo, err error) {
	logger.Info(ctx, "start checking for updates")
	latestVersion, err := getLatestVersion(ctx)
	if err != nil {
		logger.Error(ctx, err.Error())
		return UpdateInfo{}, err
	}

	// compare with current version
	existingVersion, existingErr := semver.NewVersion(CURRENT_VERSION)
	if existingErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to parse current version: %s", existingErr.Error()))
		return UpdateInfo{}, fmt.Errorf("failed to parse current version: %w", existingErr)
	}
	newVersion, newErr := semver.NewVersion(latestVersion.Version)
	if newErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to parse latest version: %s", newErr.Error()))
		return UpdateInfo{}, fmt.Errorf("failed to parse latest version: %w", newErr)
	}
	if existingVersion.LessThan(newVersion) || existingVersion.Equal(newVersion) {
		logger.Info(ctx, fmt.Sprintf("no new version available, current: %s, latest: %s", existingVersion.String(), newVersion.String()))
		return UpdateInfo{
			CurrentVersion: existingVersion.String(),
			LatestVersion:  newVersion.String(),
			ReleaseNotes:   latestVersion.ReleaseNotes,
		}, errors.New("no new version available")
	}

	logger.Info(ctx, fmt.Sprintf("new version available, current: %s, latest: %s", existingVersion.String(), newVersion.String()))

	var downloadUrl string
	if util.IsMacOS() {
		downloadUrl = latestVersion.MacDownloadUrl
	}
	if util.IsWindows() {
		downloadUrl = latestVersion.WindowsDownloadUrl
	}
	if util.IsLinux() {
		downloadUrl = latestVersion.LinuxDownloadUrl
	}
	if downloadUrl == "" {
		logger.Error(ctx, "no download url found")
		return UpdateInfo{}, errors.New("no download url found")
	}

	return UpdateInfo{
		CurrentVersion: existingVersion.String(),
		LatestVersion:  newVersion.String(),
		ReleaseNotes:   latestVersion.ReleaseNotes,
		DownloadUrl:    downloadUrl,
	}, nil
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
