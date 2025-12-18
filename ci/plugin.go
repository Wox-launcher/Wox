package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/imroc/req/v3"
	"github.com/tidwall/pretty"
)

// should be same as StorePluginManifest in store.go
type storePluginManifest struct {
	Id             string
	Name           string // supported i18n
	Author         string
	Version        string
	MinWoxVersion  string
	Runtime        string
	Description    string // supported i18n
	IconUrl        string
	IconEmoji      string
	Website        string
	DownloadUrl    string
	ScreenshotUrls []string
	SupportedOS    []string
	DateCreated    string
	DateUpdated    string

	// I18n holds inline translations for the store manifest.
	// Map structure: langCode -> key -> translatedValue
	// Example: {"en_US": {"plugin_name": "Hello"}, "zh_CN": {"plugin_name": "你好"}}
	I18n map[string]map[string]string
}

func runPlugin() {
	err := checkPluginNewVersion()
	if err != nil {
		fmt.Println("Check plugin new version err: " + err.Error())
		os.Exit(1)
	}
}

func checkPluginNewVersion() error {
	fileStr, err := os.ReadFile("../store-plugin.json")
	if err != nil {
		return err
	}

	var plugins []storePluginManifest
	unmarshalErr := json.Unmarshal(fileStr, &plugins)
	if unmarshalErr != nil {
		return fmt.Errorf("unmarshal plugin store json err: %s", unmarshalErr.Error())
	}

	hasUpdate := false
	for index, plugin := range plugins {
		var newVersion string
		var versionErr error

		// check if it's a gist-hosted plugin
		if strings.Contains(plugin.DownloadUrl, "gist.githubusercontent.com") || strings.Contains(plugin.Website, "gist.github.com") {
			newVersion, versionErr = getLatestGistVersion(plugin.DownloadUrl)
			if versionErr != nil {
				fmt.Printf("[%s] Get latest gist version err: %s\n", plugin.Name, versionErr.Error())
				continue
			}
		} else if strings.HasPrefix(plugin.DownloadUrl, "https://github.com") || strings.HasPrefix(plugin.DownloadUrl, "https://www.github.com") {
			// check plugins that hosted on github repo
			newVersion, versionErr = getLatestReleaseVersion(plugin.Website)
			if versionErr != nil {
				fmt.Printf("[%s] Get latest release version err: %s\n", plugin.Name, versionErr.Error())
				continue
			}
		} else {
			fmt.Printf("[%s] is not hosted on github or gist\n", plugin.Name)
			continue
		}

		existVersion, existVersionErr := semver.NewVersion(plugin.Version)
		if existVersionErr != nil {
			fmt.Printf("[%s] Parse exist version err: %s", plugin.Name, existVersionErr.Error())
			continue
		}

		currentVersion, currentVersionErr := semver.NewVersion(newVersion)
		if currentVersionErr != nil {
			fmt.Printf("[%s] Parse new version err: %s", plugin.Name, currentVersionErr.Error())
			continue
		}

		if currentVersion.GreaterThan(existVersion) {
			plugins[index].Version = currentVersion.String()
			plugins[index].DateUpdated = time.Now().Format("2006-01-02 15:04:05")
			hasUpdate = true
			fmt.Printf("[%s] Exist version: %s, New version: %s, update found\n", plugin.Name, existVersion, currentVersion)
		} else {
			fmt.Printf("[%s] Exist version: %s, New version: %s\n", plugin.Name, existVersion, currentVersion)
		}
	}

	if hasUpdate {
		marshal, marshalErr := json.Marshal(plugins)
		if marshalErr != nil {
			return fmt.Errorf("marshal plugin store json err: %s", marshalErr.Error())
		}
		return os.WriteFile("../store-plugin.json", pretty.Pretty(marshal), 0644)
	}

	return nil
}

func getLatestGistVersion(downloadUrl string) (string, error) {
	if downloadUrl == "" {
		return "", fmt.Errorf("downloadUrl is empty")
	}

	// Fetch the latest gist content
	result, err := req.Get(downloadUrl)
	if err != nil {
		return "", err
	}

	content := result.String()
	// Only read the first 20 lines and match "Version": "x.y.z"
	lines := strings.Split(content, "\n")
	if len(lines) > 20 {
		lines = lines[:20]
	}
	head := strings.Join(lines, "\n")

	groups := findRegexGroups(`(?m)"Version"\s*:\s*"(?P<version>\d+\.\d+\.\d+)"`, head)
	if len(groups) > 0 {
		return strings.TrimSpace(groups[0]["version"]), nil
	}

	return "", fmt.Errorf("can not find Version in gist content head")
}

func getLatestReleaseVersion(website string) (string, error) {
	latestReleaseUrl := fmt.Sprintf("%s/releases/latest", website)
	req.SetRedirectPolicy(req.MaxRedirectPolicy(3))
	result, err := req.Get(latestReleaseUrl)
	if err != nil {
		return "", err
	}

	groups := findRegexGroups(`(?ms)breadcrumb-item-selected\">(.*?)class="Link">(?P<version>.*?)</a>`, result.String())
	if len(groups) == 0 {
		return "", fmt.Errorf("can not find version from %s", latestReleaseUrl)
	}

	return strings.TrimSpace(groups[0]["version"]), nil
}

func findRegexGroups(regexExpression, raw string) (groups []map[string]string) {
	var compRegEx = regexp.MustCompile(regexExpression)
	matches := compRegEx.FindAllStringSubmatch(raw, -1)

	for _, match := range matches {
		subGroup := make(map[string]string)
		for i, name := range compRegEx.SubexpNames() {
			if i > 0 && i <= len(match) {
				subGroup[name] = match[i]
			}
		}
		groups = append(groups, subGroup)
	}

	return groups
}
