package browserbookmark

import (
	"os"
	"path/filepath"
	"wox/util/browser"
)

// getLinuxBookmarkRootDirs includes native and sandboxed browser data locations.
func getLinuxBookmarkRootDirs(browserID string) []string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if !filepath.IsAbs(configHome) {
		configHome = "~/.config"
	}

	switch browserID {
	case browser.BrowserIDChrome:
		return []string{
			filepath.Join(configHome, "google-chrome"),
			"~/.var/app/com.google.Chrome/config/google-chrome",
		}
	case browser.BrowserIDFirefox:
		return []string{
			"~/.mozilla/firefox",
			filepath.Join(configHome, "mozilla", "firefox"),
			"~/snap/firefox/common/.mozilla/firefox",
			"~/snap/firefox/common/.config/mozilla/firefox",
			"~/.var/app/org.mozilla.firefox/.mozilla/firefox",
			"~/.var/app/org.mozilla.firefox/config/mozilla/firefox",
		}
	default:
		return nil
	}
}
