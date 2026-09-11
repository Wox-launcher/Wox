package browser

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

// DefaultBrowserID returns the canonical ID of the OS default web browser when
// it can be detected as a supported browser. Empty means unknown.
func DefaultBrowserID() string {
	return detectDefaultBrowserID()
}

func browserIDFromWindowsProgID(progID string) string {
	id := strings.ToLower(strings.TrimSpace(progID))
	switch {
	case strings.HasPrefix(id, "chromehtml"):
		return BrowserIDChrome
	case strings.HasPrefix(id, "msedgehtm"):
		return BrowserIDEdge
	case strings.HasPrefix(id, "firefoxurl"):
		return BrowserIDFirefox
	case strings.HasPrefix(id, "bravehtml"):
		return BrowserIDBrave
	case strings.HasPrefix(id, "operastable"), strings.HasPrefix(id, "opera"):
		return BrowserIDOpera
	case strings.HasPrefix(id, "chromiumhtm"):
		return BrowserIDChromium
	default:
		return ""
	}
}

func browserIDFromLinuxDesktopFile(desktop string) string {
	name := strings.ToLower(strings.TrimSpace(desktop))
	name = strings.TrimSuffix(name, ".desktop")
	switch {
	case strings.Contains(name, "chromium"):
		return BrowserIDChromium
	case strings.Contains(name, "google-chrome"), strings.Contains(name, "com.google.chrome"):
		return BrowserIDChrome
	case strings.Contains(name, "microsoft-edge"), strings.Contains(name, "msedge"):
		return BrowserIDEdge
	case strings.Contains(name, "firefox"):
		return BrowserIDFirefox
	case strings.Contains(name, "brave"):
		return BrowserIDBrave
	case strings.Contains(name, "opera"):
		return BrowserIDOpera
	default:
		return ""
	}
}

func browserIDFromMacAppPath(appPath string) string {
	base := strings.ToLower(filepath.Base(strings.TrimRight(strings.TrimSpace(appPath), `/\`)))
	switch base {
	case "google chrome.app":
		return BrowserIDChrome
	case "microsoft edge.app":
		return BrowserIDEdge
	case "firefox.app":
		return BrowserIDFirefox
	case "brave browser.app":
		return BrowserIDBrave
	case "opera.app":
		return BrowserIDOpera
	case "chromium.app":
		return BrowserIDChromium
	case "safari.app":
		return BrowserIDSafari
	default:
		return ""
	}
}

type launchServicesPlist struct {
	LSHandlers []struct {
		LSHandlerURLScheme  string `json:"LSHandlerURLScheme"`
		LSHandlerRoleAll    string `json:"LSHandlerRoleAll"`
		LSHandlerRoleViewer string `json:"LSHandlerRoleViewer"`
	} `json:"LSHandlers"`
}

func browserIDFromLaunchServicesJSON(data []byte) string {
	var plist launchServicesPlist
	if err := json.Unmarshal(data, &plist); err != nil {
		return ""
	}

	bundleID := ""
	for _, handler := range plist.LSHandlers {
		scheme := strings.ToLower(strings.TrimSpace(handler.LSHandlerURLScheme))
		if scheme != "http" && scheme != "https" {
			continue
		}
		if role := strings.TrimSpace(handler.LSHandlerRoleAll); role != "" {
			bundleID = role
			continue
		}
		if role := strings.TrimSpace(handler.LSHandlerRoleViewer); role != "" {
			bundleID = role
		}
	}
	return macIdentityToBrowserID(strings.ToLower(bundleID))
}
