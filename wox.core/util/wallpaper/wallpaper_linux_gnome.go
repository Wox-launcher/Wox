//go:build linux

package wallpaper

import (
	"errors"
	"os/exec"
	"strings"
)

// gnomeWallpaperPath reads the image GNOME stores in org.gnome.desktop.background.
func gnomeWallpaperPath() (string, error) {
	for _, key := range []string{"picture-uri-dark", "picture-uri"} {
		output, err := exec.Command("gsettings", "get", "org.gnome.desktop.background", key).Output()
		if err != nil {
			continue
		}
		if value := resolveWallpaperPath(strings.TrimSpace(string(output)), key == "picture-uri-dark"); value != "" {
			return value, nil
		}
	}
	return "", errors.New("desktop wallpaper is unavailable")
}
