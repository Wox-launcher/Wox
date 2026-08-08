//go:build !windows && !darwin && !linux

package wallpaper

import "errors"

func getSystemWallpaperPath() (string, error) {
	return "", errors.New("desktop wallpaper is unsupported")
}
