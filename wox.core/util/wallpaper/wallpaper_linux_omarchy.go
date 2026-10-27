//go:build linux

package wallpaper

import (
	"errors"
	"os"
	"path/filepath"
)

// omarchyWallpaperPath follows the symlink the Omarchy shell renders.
// The link itself has no extension; the theme image it points at does.
func omarchyWallpaperPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", errors.New("desktop wallpaper is unavailable")
	}
	return resolveOmarchyWallpaperPath(filepath.Join(home, ".local", "state", "omarchy", "current", "background"))
}

// resolveOmarchyWallpaperPath returns the theme image behind Omarchy's current background link.
func resolveOmarchyWallpaperPath(link string) (string, error) {
	resolved, err := filepath.EvalSymlinks(link)
	if err != nil {
		return "", errors.New("desktop wallpaper is unavailable")
	}
	if path := existingWallpaperImagePath(resolved, false); path != "" {
		return path, nil
	}
	return "", errors.New("desktop wallpaper is unavailable")
}
