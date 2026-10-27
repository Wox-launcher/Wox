//go:build linux

package util

import (
	"os"
	"path/filepath"
	"strings"
)

// IsOmarchy reports whether this machine is an Omarchy install.
// Omarchy-only defaults, such as the Alt+Space launcher shortcut, use this check.
func IsOmarchy() bool {
	if omarchyDirExists(os.Getenv("OMARCHY_PATH")) {
		return true
	}
	if omarchyDirExists("/usr/share/omarchy") {
		return true
	}
	info, err := os.Stat(omarchyColorsFile())
	return err == nil && info.Mode().IsRegular()
}

func omarchyDirExists(path string) bool {
	info, err := os.Stat(strings.TrimSpace(path))
	return err == nil && info.IsDir()
}

func omarchyColorsFile() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".local", "state", "omarchy", "current", "theme", "colors.toml")
}
