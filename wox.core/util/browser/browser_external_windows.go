//go:build windows

package browser

import (
	"strings"
	"wox/util/shell"
)

// Use a conservative byte threshold to retain direct launch for long checkout URLs.
const externalURLShellLengthLimit = 2048

// openExternalURL uses system routing except for long web links that need direct browser arguments.
func openExternalURL(rawURL string, owner uintptr) error {
	return launchExternalURL(rawURL, owner, DefaultBrowserID, OpenURL, shell.OpenWithOwner)
}

// launchExternalURL keeps launch policy testable without opening applications on the desktop.
func launchExternalURL(rawURL string, owner uintptr, defaultBrowser func() string, openBrowser func(string, string) error, openSystem func(string, uintptr) error) error {
	// Ordinary links must use the effective Windows association: UserChoice can be stale.
	// Keep direct arguments for long web links because shell dispatch can truncate checkout fragments.
	if len(rawURL) > externalURLShellLengthLimit && (strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://")) {
		if browserID := defaultBrowser(); browserID != "" {
			if err := openBrowser(rawURL, browserID); err == nil {
				return nil
			}
		}
	}
	return openSystem(rawURL, owner)
}
