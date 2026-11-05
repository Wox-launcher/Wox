package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/imroc/req/v3"
	"github.com/tidwall/sjson"
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

	updated, err := updatePluginStore(fileStr, getLatestPluginMetadata, getPluginRelease)
	if err != nil {
		return err
	}
	if !bytes.Equal(fileStr, updated) {
		return os.WriteFile("../store-plugin.json", updated, 0644)
	}
	return nil
}

// updatePluginStore synchronizes version requirements without rewriting unrelated catalog fields.
func updatePluginStore(fileStr []byte, loadMetadata func(string) (storePluginManifest, error), loadRelease func(string) (*pluginRelease, error)) ([]byte, error) {
	var plugins []storePluginManifest
	unmarshalErr := json.Unmarshal(fileStr, &plugins)
	if unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal plugin store json err: %s", unmarshalErr.Error())
	}

	for index, plugin := range plugins {
		// Read both versions from the artifact users actually install. Release tags
		// can differ from plugin.json, especially when DownloadUrl pins a release.
		metadata, metadataErr := loadMetadata(plugin.DownloadUrl)
		if metadataErr != nil {
			fmt.Printf("[%s] Get latest plugin metadata err: %s\n", plugin.Name, metadataErr.Error())
			continue
		}
		if metadata.Id != plugin.Id {
			fmt.Printf("[%s] Plugin metadata Id %q does not match store Id %q\n", plugin.Name, metadata.Id, plugin.Id)
			continue
		}

		existVersion, existVersionErr := semver.NewVersion(plugin.Version)
		if existVersionErr != nil {
			fmt.Printf("[%s] Parse exist version err: %s", plugin.Name, existVersionErr.Error())
			continue
		}

		currentVersion, currentVersionErr := semver.NewVersion(metadata.Version)
		if currentVersionErr != nil {
			fmt.Printf("[%s] Parse new version err: %s", plugin.Name, currentVersionErr.Error())
			continue
		}

		// Match the core's compatibility default for older artifacts that omit this field.
		minimum := strings.TrimSpace(metadata.MinWoxVersion)
		if minimum == "" {
			minimum = "2.0.0"
		}
		if _, err := semver.NewVersion(minimum); err != nil {
			fmt.Printf("[%s] Parse MinWoxVersion err: %s\n", plugin.Name, err.Error())
			continue
		}

		// Equal versions must still repair compatibility metadata missed by older CI runs.
		if currentVersion.GreaterThan(existVersion) || (currentVersion.Equal(existVersion) && minimum != plugin.MinWoxVersion) {
			updatedAt := plugin.DateUpdated
			if currentVersion.GreaterThan(existVersion) {
				release, releaseErr := loadRelease(plugin.DownloadUrl)
				if releaseErr != nil {
					fmt.Printf("[%s] Get plugin release err: %s\n", plugin.Name, releaseErr)
					continue
				}
				if release == nil {
					// Downloads without a GitHub release retain their existing update-time behavior.
					updatedAt = time.Now().UTC().Format("2006-01-02 15:04:05")
				} else {
					// A latest URL can change between requests; verify the timestamp belongs to the artifact we inspected.
					pinned, pinnedErr := loadMetadata(release.DownloadURL)
					if pinnedErr != nil || pinned.Id != metadata.Id || pinned.Version != metadata.Version || pinned.MinWoxVersion != metadata.MinWoxVersion {
						fmt.Printf("[%s] Release artifact changed or could not be verified: %v\n", plugin.Name, pinnedErr)
						continue
					}
					updatedAt = release.PublishedAt.UTC().Format("2006-01-02 15:04:05")
				}
			}

			// Update only the fields we need, keeping the original JSON structure
			// (avoid marshaling the whole manifest which would introduce missing optional fields).
			updatedBytes, setErr := sjson.SetBytes(fileStr, fmt.Sprintf("%d.Version", index), currentVersion.String())
			if setErr != nil {
				return nil, fmt.Errorf("set Version err: %w", setErr)
			}
			updatedBytes, setErr = sjson.SetBytes(updatedBytes, fmt.Sprintf("%d.MinWoxVersion", index), minimum)
			if setErr != nil {
				return nil, fmt.Errorf("set MinWoxVersion err: %w", setErr)
			}
			updatedBytes, setErr = sjson.SetBytes(updatedBytes, fmt.Sprintf("%d.DateUpdated", index), updatedAt)
			if setErr != nil {
				return nil, fmt.Errorf("set DateUpdated err: %w", setErr)
			}
			fileStr = updatedBytes

			fmt.Printf("[%s] Exist version: %s, New version: %s, MinWoxVersion: %s -> %s, update found\n", plugin.Name, existVersion, currentVersion, plugin.MinWoxVersion, minimum)
		} else {
			fmt.Printf("[%s] Exist version: %s, New version: %s\n", plugin.Name, existVersion, currentVersion)
		}
	}

	return fileStr, nil
}

type pluginRelease struct {
	DownloadURL string
	PublishedAt time.Time
}

// getPluginRelease resolves a GitHub release asset to its fixed URL and publication time.
func getPluginRelease(downloadURL string) (*pluginRelease, error) {
	return fetchPluginRelease(downloadURL, "https://api.github.com", &http.Client{Timeout: time.Minute})
}

// fetchPluginRelease returns nil for non-release URLs, which have no release date.
func fetchPluginRelease(downloadURL, apiBase string, client *http.Client) (*pluginRelease, error) {
	u, err := url.Parse(downloadURL)
	if err != nil || u.Scheme != "https" || u.Host != "github.com" {
		return nil, nil
	}
	parts := strings.Split(strings.Trim(u.EscapedPath(), "/"), "/")
	if len(parts) != 6 || parts[2] != "releases" {
		return nil, nil
	}
	var endpoint string
	switch {
	case parts[3] == "latest" && parts[4] == "download":
		endpoint = "latest"
	case parts[3] == "download":
		endpoint = "tags/" + parts[4]
	default:
		return nil, nil
	}
	assetName, err := url.PathUnescape(parts[5])
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest(http.MethodGet, apiBase+"/repos/"+parts[0]+"/"+parts[1]+"/releases/"+endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "Wox-plugin-store")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub release: HTTP %d", response.StatusCode)
	}
	var release struct {
		TagName     string `json:"tag_name"`
		PublishedAt string `json:"published_at"`
		Draft       bool   `json:"draft"`
		Assets      []struct {
			Name string `json:"name"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(response.Body).Decode(&release); err != nil {
		return nil, err
	}
	published, err := time.Parse(time.RFC3339, release.PublishedAt)
	if err != nil || release.Draft || release.TagName == "" {
		return nil, fmt.Errorf("release has no valid publication time or tag")
	}
	for _, asset := range release.Assets {
		if strings.EqualFold(asset.Name, assetName) {
			return &pluginRelease{
				DownloadURL: "https://github.com/" + parts[0] + "/" + parts[1] + "/releases/download/" + url.PathEscape(release.TagName) + "/" + url.PathEscape(asset.Name),
				PublishedAt: published,
			}, nil
		}
	}
	return nil, fmt.Errorf("release does not contain asset %q", assetName)
}

// getLatestPluginMetadata reads packaged or inline metadata from the catalog's download URL.
func getLatestPluginMetadata(downloadURL string) (storePluginManifest, error) {
	parsedURL, err := url.Parse(downloadURL)
	if err != nil || (parsedURL.Scheme != "https" && parsedURL.Scheme != "http") || parsedURL.Host == "" {
		return storePluginManifest{}, fmt.Errorf("invalid plugin download URL %q", downloadURL)
	}
	response, err := req.C().SetTimeout(time.Minute).R().Get(downloadURL)
	if err != nil {
		return storePluginManifest{}, err
	}
	if !response.IsSuccessState() {
		return storePluginManifest{}, fmt.Errorf("download plugin: HTTP %d", response.StatusCode)
	}
	content := response.Bytes()
	var metadata storePluginManifest
	if strings.HasSuffix(strings.ToLower(parsedURL.Path), ".wox") {
		archive, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
		if err != nil {
			return metadata, fmt.Errorf("open plugin package: %w", err)
		}
		for _, file := range archive.File {
			if file.Name != "plugin.json" {
				continue
			}
			reader, err := file.Open()
			if err != nil {
				return metadata, err
			}
			defer reader.Close()
			content, err = io.ReadAll(reader)
			if err != nil {
				return metadata, err
			}
			if err := json.Unmarshal(content, &metadata); err != nil {
				return metadata, fmt.Errorf("parse plugin.json: %w", err)
			}
			return metadata, nil
		}
		return metadata, fmt.Errorf("plugin.json not found in package")
	}

	// Only the leading comment block is metadata; later source code can contain
	// unrelated JSON. Read the whole header because translations can exceed 20 lines.
	var header strings.Builder
headerLines:
	for _, line := range strings.Split(strings.TrimPrefix(string(content), "\uFEFF"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || (header.Len() == 0 && strings.HasPrefix(line, "#!")) {
			continue
		}
		switch {
		case strings.HasPrefix(line, "#"):
			line = strings.TrimPrefix(line, "#")
		case strings.HasPrefix(line, "//"):
			line = strings.TrimPrefix(line, "//")
		default:
			break headerLines
		}
		header.WriteString(line)
		header.WriteByte('\n')
	}

	start := strings.Index(header.String(), "{")
	if start < 0 {
		return metadata, fmt.Errorf("JSON metadata not found in plugin header")
	}
	if err := json.NewDecoder(strings.NewReader(header.String()[start:])).Decode(&metadata); err != nil {
		return metadata, fmt.Errorf("parse inline plugin metadata: %w", err)
	}
	return metadata, nil
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
