package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/tidwall/sjson"
)

type storeThemeManifest struct {
	Id          string
	Name        string
	Version     string
	Website     string
	DownloadUrl string
	DateUpdated string
}

func runTheme() {
	if err := checkThemeNewVersion(); err != nil {
		fmt.Println("Check theme new version err: " + err.Error())
		os.Exit(1)
	}
}

func checkThemeNewVersion() error {
	fileStr, err := os.ReadFile("../store-theme.json")
	if err != nil {
		return err
	}

	var themes []storeThemeManifest
	if err := json.Unmarshal(fileStr, &themes); err != nil {
		return fmt.Errorf("unmarshal theme store json err: %s", err.Error())
	}

	hasUpdate := false
	for index, theme := range themes {
		newVersion, versionErr := themeRemoteVersion(theme)
		if versionErr != nil {
			fmt.Printf("[%s] Get latest theme version err: %s\n", theme.Name, versionErr.Error())
			continue
		}

		existVersion, existVersionErr := semver.NewVersion(theme.Version)
		if existVersionErr != nil {
			fmt.Printf("[%s] Parse exist version err: %s\n", theme.Name, existVersionErr.Error())
			continue
		}
		currentVersion, currentVersionErr := semver.NewVersion(newVersion)
		if currentVersionErr != nil {
			fmt.Printf("[%s] Parse new version err: %s\n", theme.Name, currentVersionErr.Error())
			continue
		}
		if !currentVersion.GreaterThan(existVersion) {
			fmt.Printf("[%s] Exist version: %s, New version: %s\n", theme.Name, existVersion, currentVersion)
			continue
		}

		updatedAt := time.Now().Format("2006-01-02 15:04:05")
		updatedBytes, setErr := sjson.SetBytes(fileStr, fmt.Sprintf("%d.Version", index), currentVersion.String())
		if setErr != nil {
			return fmt.Errorf("set Version err: %w", setErr)
		}
		updatedBytes, setErr = sjson.SetBytes(updatedBytes, fmt.Sprintf("%d.DateUpdated", index), updatedAt)
		if setErr != nil {
			return fmt.Errorf("set DateUpdated err: %w", setErr)
		}
		fileStr = updatedBytes
		themes[index].Version = currentVersion.String()
		themes[index].DateUpdated = updatedAt
		hasUpdate = true
		fmt.Printf("[%s] Exist version: %s, New version: %s, update found\n", theme.Name, existVersion, currentVersion)
	}

	if hasUpdate {
		return os.WriteFile("../store-theme.json", fileStr, 0644)
	}
	return nil
}

// themeRemoteVersion reads Version from a single-file JSON theme, or the GitHub release for a package.
func themeRemoteVersion(theme storeThemeManifest) (string, error) {
	switch themeDownloadExtension(theme.DownloadUrl) {
	case ".json":
		return getLatestGistVersion(theme.DownloadUrl)
	case ".wox-theme":
		if theme.Website == "" {
			return "", fmt.Errorf("package theme website is empty")
		}
		return getLatestReleaseVersion(theme.Website)
	default:
		return "", fmt.Errorf("unsupported theme download URL")
	}
}

func themeDownloadExtension(downloadURL string) string {
	trimmed := strings.TrimSpace(downloadURL)
	if trimmed == "" {
		return ""
	}
	name := trimmed
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Path != "" {
		name = path.Base(parsed.Path)
	}
	ext := strings.ToLower(path.Ext(name))
	if ext == "." {
		return ""
	}
	return ext
}
