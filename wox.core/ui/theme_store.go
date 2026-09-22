package ui

import (
	"context"
	"fmt"
	"os"
	"time"
	"wox/common"
	"wox/updater"
	"wox/util"

	"github.com/Masterminds/semver/v3"
)

const themeDownloadTimeout = 2 * time.Minute

// InstallManifest downloads one store theme, installs it, and selects it.
// A .json URL is a single-file theme. A .wox-theme URL is a package that may contain images.
func (s *Store) InstallManifest(ctx context.Context, manifest common.StoreThemeManifest) error {
	return s.installManifest(ctx, manifest, true, true)
}

// InstallManifestLocal downloads a store theme for cloud sync without selecting it or writing an oplog.
func (s *Store) InstallManifestLocal(ctx context.Context, manifest common.StoreThemeManifest) error {
	return s.installManifest(ctx, manifest, false, false)
}

func (s *Store) installManifest(ctx context.Context, manifest common.StoreThemeManifest, syncInstall bool, applyTheme bool) error {
	if err := manifest.Validate(); err != nil {
		return err
	}
	if err := (common.Theme{ThemeName: manifest.Name, MinWoxVersion: manifest.MinWoxVersion}).EnsureWoxVersionSupported(updater.CURRENT_VERSION); err != nil {
		return err
	}
	if installed := GetUIManager().GetThemeById(manifest.Id); installed.ThemeId != "" {
		installedVersion, installedErr := semver.NewVersion(installed.Version)
		availableVersion, availableErr := semver.NewVersion(manifest.Version)
		if installedErr == nil && availableErr == nil && installedVersion.GreaterThan(availableVersion) {
			return fmt.Errorf("skip %s(%s), already installed(%s)", manifest.Name, manifest.Version, installed.Version)
		}
	}

	kind, err := common.ClassifyThemeArtifact(manifest.DownloadUrl)
	if err != nil {
		return err
	}
	downloadCtx, cancel := context.WithTimeout(ctx, themeDownloadTimeout)
	defer cancel()
	switch kind {
	case common.ThemeArtifactSingleFile:
		data, err := util.HttpGet(downloadCtx, manifest.DownloadUrl)
		if err != nil {
			return err
		}
		theme, err := common.ThemeDocumentForStoreInstall(data, manifest, updater.CURRENT_VERSION)
		if err != nil {
			return err
		}
		return s.install(ctx, theme, syncInstall, applyTheme)
	case common.ThemeArtifactPackage:
		return s.installThemePackage(ctx, downloadCtx, manifest, syncInstall, applyTheme)
	default:
		return fmt.Errorf("unsupported theme download %s", manifest.DownloadUrl)
	}
}

// installThemePackage checks the archive identity before writing it into the theme directory.
func (s *Store) installThemePackage(ctx, downloadCtx context.Context, manifest common.StoreThemeManifest, syncInstall bool, applyTheme bool) error {
	file, err := os.CreateTemp("", "wox-theme-*.wox-theme")
	if err != nil {
		return err
	}
	packagePath := file.Name()
	if err := file.Close(); err != nil {
		os.Remove(packagePath)
		return err
	}
	defer os.Remove(packagePath)
	if err := util.HttpDownload(downloadCtx, manifest.DownloadUrl, packagePath); err != nil {
		return err
	}
	theme, err := common.ReadThemePackage(packagePath, updater.CURRENT_VERSION)
	if err != nil {
		return err
	}
	if theme.ThemeId != manifest.Id {
		return fmt.Errorf("theme id %s does not match store entry %s", theme.ThemeId, manifest.Id)
	}
	return s.install(ctx, theme, syncInstall, applyTheme)
}
