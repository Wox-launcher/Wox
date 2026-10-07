//go:build windows

package browser

import (
	"strings"
	"wox/util/shell"
)

// openExternalURL prefers direct browser arguments for long checkout links, retaining owner-aware shell fallback.
func openExternalURL(rawURL string, owner uintptr) error {
	return launchExternalURL(rawURL, owner, DefaultBrowserID, OpenURL, shell.OpenWithOwner)
}

// launchExternalURL keeps launch policy testable without opening applications on the desktop.
func launchExternalURL(rawURL string, owner uintptr, defaultBrowser func() string, openBrowser func(string, string) error, openSystem func(string, uintptr) error) error {
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		if browserID := defaultBrowser(); browserID != "" {
			if err := openBrowser(rawURL, browserID); err == nil {
				return nil
			}
		}
	}
	return openSystem(rawURL, owner)
}
