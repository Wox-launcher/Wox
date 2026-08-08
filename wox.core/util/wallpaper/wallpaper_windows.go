//go:build windows

package wallpaper

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

func getSystemWallpaperPath() (string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Control Panel\Desktop`, registry.QUERY_VALUE)
	if err == nil {
		defer key.Close()
		if path, _, valueErr := key.GetStringValue("WallPaper"); valueErr == nil && path != "" {
			if _, statErr := os.Stat(path); statErr == nil {
				return path, nil
			}
		}
	}
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", errors.New("desktop wallpaper is unavailable")
	}
	transcoded := filepath.Join(appData, "Microsoft", "Windows", "Themes", "TranscodedWallpaper")
	if _, statErr := os.Stat(transcoded); statErr == nil {
		return transcoded, nil
	}
	cachedDirectory := filepath.Join(appData, "Microsoft", "Windows", "Themes", "CachedFiles")
	files, readErr := os.ReadDir(cachedDirectory)
	if readErr == nil {
		for _, file := range files {
			if !file.IsDir() {
				return filepath.Join(cachedDirectory, file.Name()), nil
			}
		}
	}
	return "", errors.New("desktop wallpaper is unavailable")
}
