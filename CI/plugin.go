package main

import (
	"encoding/json"
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/imroc/req/v3"
	"github.com/tidwall/pretty"
	"os"
	"regexp"
	"strings"
	"time"
)

type storePluginManifest struct {
	Id             string
	Name           string
	Author         string
	Version        string
	MinWoxVersion  string
	Runtime        string
	Description    string
	IconUrl        string
	Website        string
	DownloadUrl    string
	ScreenshotUrls []string
	DateCreated    string
	DateUpdated    string
}

func main() {
	err := checkPluginNewVersion()
	if err != nil {
		fmt.Println("Check plugin new version err: " + err.Error())
		os.Exit(1)
	}
}

func checkPluginNewVersion() error {
	fileStr, err := os.ReadFile("../plugin-store.json")
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
		// only check plugins that hosted on github
		if !strings.HasPrefix(plugin.Website, "https://github.com") && !strings.HasPrefix(plugin.Website, "https://www.github.com") {
			fmt.Println(fmt.Sprintf("[%s] is not hosted on github", plugin.Name))
			continue
		}

		newVersion, versionErr := getLatestReleaseVersion(plugin.Website)
		if versionErr != nil {
			fmt.Println(fmt.Sprintf("[%s] Get latest release version err: %s", plugin.Name, versionErr.Error()))
			continue
		}

		existVersion, existVersionErr := semver.NewVersion(plugin.Version)
		if existVersionErr != nil {
			fmt.Println(fmt.Sprintf("[%s] Parse exist version err: %s", plugin.Name, existVersionErr.Error()))
			continue
		}

		currentVersion, currentVersionErr := semver.NewVersion(newVersion)
		if currentVersionErr != nil {
			fmt.Println(fmt.Sprintf("[%s] Parse new version err: %s", plugin.Name, currentVersionErr.Error()))
			continue
		}

		if currentVersion.GreaterThan(existVersion) {
			plugins[index].Version = currentVersion.String()
			plugins[index].DateUpdated = time.Now().Format("2006-01-02 15:04:05")
			hasUpdate = true
			fmt.Println(fmt.Sprintf("[%s] Exist version: %s, New version: %s, udpate found", plugin.Name, existVersion, currentVersion))
		} else {
			fmt.Println(fmt.Sprintf("[%s] Exist version: %s, New version: %s", plugin.Name, existVersion, currentVersion))
		}
	}

	if hasUpdate {
		marshal, marshalErr := json.Marshal(plugins)
		if marshalErr != nil {
			return fmt.Errorf("marshal plugin store json err: %s", marshalErr.Error())
		}
		return os.WriteFile("../plugin-store.json", pretty.Pretty(marshal), 0644)
	}

	return nil
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
