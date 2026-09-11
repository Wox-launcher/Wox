//go:build darwin

package browser

import (
	"os"
	"os/exec"
	"path/filepath"
)

func detectDefaultBrowserID() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	for _, relative := range []string{
		filepath.Join("Library", "Preferences", "com.apple.LaunchServices", "com.apple.launchservices.secure.plist"),
		filepath.Join("Library", "Preferences", "com.apple.LaunchServices.plist"),
	} {
		if id := browserIDFromLaunchServicesPlist(filepath.Join(home, relative)); id != "" {
			return id
		}
	}
	return ""
}

func browserIDFromLaunchServicesPlist(path string) string {
	output, err := exec.Command("plutil", "-convert", "json", "-o", "-", path).Output()
	if err != nil {
		return ""
	}
	return browserIDFromLaunchServicesJSON(output)
}
