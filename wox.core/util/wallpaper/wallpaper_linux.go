//go:build linux

package wallpaper

import (
	"errors"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

func getSystemWallpaperPath() (string, error) {
	for _, key := range []string{"picture-uri-dark", "picture-uri"} {
		output, err := exec.Command("gsettings", "get", "org.gnome.desktop.background", key).Output()
		if err != nil {
			continue
		}
		value := strings.Trim(strings.TrimSpace(string(output)), "'\"")
		if strings.HasPrefix(value, "file://") {
			if parsed, parseErr := url.Parse(value); parseErr == nil {
				value = parsed.Path
			}
		}
		if value != "" {
			if _, statErr := os.Stat(value); statErr == nil {
				return value, nil
			}
		}
	}
	return "", errors.New("desktop wallpaper is unavailable")
}
