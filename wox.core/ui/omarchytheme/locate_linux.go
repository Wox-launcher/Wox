//go:build linux

package omarchytheme

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"wox/common"
	"wox/util"
)

// Available reports whether this machine should offer the Omarchy Auto theme.
func Available() bool { return util.IsOmarchy() }

// StateDir is the directory Omarchy replaces when the theme changes.
func StateDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".local", "state", "omarchy", "current")
}

// ColorsPath is the resolved palette after stock and user theme files are merged.
func ColorsPath() string {
	dir := StateDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "theme", "colors.toml")
}

// CurrentSlug is the active Omarchy theme name, such as "hackerman".
func CurrentSlug() string {
	dir := StateDir()
	if dir == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(dir, "theme.name"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// Load reads the current Omarchy palette and builds the Wox theme.
func Load(woxVersion string) (common.Theme, error) {
	path := ColorsPath()
	if path == "" {
		return common.Theme{}, fmt.Errorf("omarchy palette path is empty")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return common.Theme{}, err
	}
	palette, err := ParsePalette(data)
	if err != nil {
		return common.Theme{}, err
	}
	return ThemeFromPalette(palette, woxVersion)
}
