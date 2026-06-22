package updater

import (
	"context"
	"errors"
	"testing"
	"wox/setting"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
)

func TestCheckUpdate(t *testing.T) {
	version1, v1Err := semver.NewVersion("2.0.0-beta.2")
	version2, v2Err := semver.NewVersion("2.0.0")

	assert.Nil(t, v1Err)
	assert.Nil(t, v2Err)
	assert.True(t, version1.LessThan(version2), true)
}

func TestManifestURLForReleaseChannel(t *testing.T) {
	assert.Equal(t, stableVersionManifestUrl, manifestURLForReleaseChannel(setting.ReleaseChannelStable))
	assert.Equal(t, betaVersionManifestUrl, manifestURLForReleaseChannel(setting.ReleaseChannelBeta))
	assert.Equal(t, stableVersionManifestUrl, manifestURLForReleaseChannel(setting.ReleaseChannel("nightly")))
}

func TestBuildUpdateInfoStableChannelIgnoresPrereleaseManifest(t *testing.T) {
	info := buildUpdateInfoFromManifest(context.Background(), "2.1.1", setting.ReleaseChannelStable, testVersionManifest("2.2.0-beta.1"))

	assert.False(t, info.HasUpdate)
	assert.Equal(t, UpdateStatusNone, info.Status)
	assert.Equal(t, "2.1.1", info.LatestVersion)
	assert.Equal(t, string(setting.ReleaseChannelStable), info.ReleaseChannel)
	assert.Empty(t, info.DownloadUrl)
}

func TestBuildUpdateInfoBetaChannelAcceptsPrereleaseManifest(t *testing.T) {
	info := buildUpdateInfoFromManifest(context.Background(), "2.1.1", setting.ReleaseChannelBeta, testVersionManifest("2.2.0-beta.1"))

	assert.True(t, info.HasUpdate)
	assert.Equal(t, UpdateStatusAvailable, info.Status)
	assert.Equal(t, "2.2.0-beta.1", info.LatestVersion)
	assert.Equal(t, string(setting.ReleaseChannelBeta), info.ReleaseChannel)
	assert.NotEmpty(t, info.DownloadUrl)
}

func TestBuildUpdateInfoBetaChannelAcceptsStableManifest(t *testing.T) {
	info := buildUpdateInfoFromManifest(context.Background(), "2.2.0-beta.1", setting.ReleaseChannelBeta, testVersionManifest("2.2.0"))

	assert.True(t, info.HasUpdate)
	assert.Equal(t, UpdateStatusAvailable, info.Status)
	assert.Equal(t, "2.2.0", info.LatestVersion)
	assert.Equal(t, string(setting.ReleaseChannelBeta), info.ReleaseChannel)
}

func TestResetCurrentUpdateInfoWhenReleaseChannelChanges(t *testing.T) {
	original := currentUpdateInfo
	defer func() {
		currentUpdateInfo = original
	}()

	currentUpdateInfo = UpdateInfo{
		ReleaseChannel: string(setting.ReleaseChannelStable),
		Status:         UpdateStatusReady,
		HasUpdate:      true,
		DownloadedPath: "/tmp/wox-2.2.0",
	}

	resetCurrentUpdateInfoForReleaseChannel(setting.ReleaseChannelStable)
	assert.Equal(t, UpdateStatusReady, currentUpdateInfo.Status)
	assert.Equal(t, "/tmp/wox-2.2.0", currentUpdateInfo.DownloadedPath)

	resetCurrentUpdateInfoForReleaseChannel(setting.ReleaseChannelBeta)
	assert.Equal(t, string(setting.ReleaseChannelBeta), currentUpdateInfo.ReleaseChannel)
	assert.Equal(t, UpdateStatusNone, currentUpdateInfo.Status)
	assert.False(t, currentUpdateInfo.HasUpdate)
	assert.Empty(t, currentUpdateInfo.DownloadedPath)
}

func TestGetUpdateChannelVersionsReturnsStableAndBetaVersions(t *testing.T) {
	versions := getUpdateChannelVersions(context.Background(), func(ctx context.Context, releaseChannel setting.ReleaseChannel) (VersionManifest, error) {
		switch releaseChannel {
		case setting.ReleaseChannelStable:
			return testVersionManifest("2.1.2"), nil
		case setting.ReleaseChannelBeta:
			return testVersionManifest("2.2.0-beta.1"), nil
		default:
			return VersionManifest{}, errors.New("unexpected release channel")
		}
	})

	assert.Len(t, versions, 2)
	assert.Equal(t, UpdateChannelVersion{Channel: "stable", LatestVersion: "2.1.2"}, versions[0])
	assert.Equal(t, UpdateChannelVersion{Channel: "beta", LatestVersion: "2.2.0-beta.1"}, versions[1])
}

func TestGetUpdateChannelVersionsKeepsOtherChannelWhenOneManifestFails(t *testing.T) {
	versions := getUpdateChannelVersions(context.Background(), func(ctx context.Context, releaseChannel setting.ReleaseChannel) (VersionManifest, error) {
		if releaseChannel == setting.ReleaseChannelStable {
			return VersionManifest{}, errors.New("network unavailable")
		}
		return testVersionManifest("2.2.0-beta.1"), nil
	})

	assert.Len(t, versions, 2)
	assert.Equal(t, "stable", versions[0].Channel)
	assert.Empty(t, versions[0].LatestVersion)
	assert.Contains(t, versions[0].Error, "network unavailable")
	assert.Equal(t, UpdateChannelVersion{Channel: "beta", LatestVersion: "2.2.0-beta.1"}, versions[1])
}

func TestGetUpdateChannelVersionsDoesNotExposePrereleaseAsStableLatest(t *testing.T) {
	versions := getUpdateChannelVersions(context.Background(), func(ctx context.Context, releaseChannel setting.ReleaseChannel) (VersionManifest, error) {
		if releaseChannel == setting.ReleaseChannelStable {
			return testVersionManifest("2.2.0-beta.1"), nil
		}
		return testVersionManifest("2.2.0-beta.1"), nil
	})

	assert.Len(t, versions, 2)
	assert.Equal(t, "stable", versions[0].Channel)
	assert.Empty(t, versions[0].LatestVersion)
	assert.Contains(t, versions[0].Error, "stable update channel ignored prerelease manifest version")
	assert.Equal(t, UpdateChannelVersion{Channel: "beta", LatestVersion: "2.2.0-beta.1"}, versions[1])
}

func testVersionManifest(version string) VersionManifest {
	return VersionManifest{
		Version:             version,
		MacArm64DownloadUrl: "https://example.com/wox-mac-arm64.dmg",
		MacArm64Checksum:    "mac-arm64-md5",
		MacAmd64DownloadUrl: "https://example.com/wox-mac-amd64.dmg",
		MacAmd64Checksum:    "mac-amd64-md5",
		WindowsDownloadUrl:  "https://example.com/wox-windows-amd64.exe",
		WindowsChecksum:     "windows-md5",
		LinuxDownloadUrl:    "https://example.com/wox-linux-amd64",
		LinuxChecksum:       "linux-md5",
		ReleaseNotes:        "release notes",
	}
}
