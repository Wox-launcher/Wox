package wallpaper

// GetSystemWallpaperPath resolves the active desktop image through the current platform implementation.
func GetSystemWallpaperPath() (string, error) {
	return getSystemWallpaperPath()
}
