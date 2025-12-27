package fileicon

import (
	"context"
	"path/filepath"
	"strings"
	"wox/util"
)

// GetFileIconByPath returns a WoxImage representing the OS file icon for the given path.
// It first tries to resolve the application/file icon, then falls back to the file-type icon.
func GetFileIconByPath(ctx context.Context, filePath string) (string, error) {
	iconPath, err := getFileIconImpl(ctx, filePath)
	if err == nil && strings.TrimSpace(iconPath) != "" {
		return iconPath, nil
	}

	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(filePath)))
	if ext == "" {
		// Unknown extension – treat as generic
		ext = ".__unknown"
	}
	return GetFileTypeIcon(ctx, ext)
}

// GetFileTypeIcon returns a WoxImage representing the OS file-type icon for the given extension.
// The ext can be with or without leading dot.
func GetFileTypeIcon(ctx context.Context, ext string) (string, error) {
	if ext == "" {
		ext = ".__unknown"
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return getFileTypeIconImpl(ctx, ext)
}

// buildCachePath returns the cache file path for a given extension and size (in px).
func buildCachePath(ext string, size int) string {
	// sanitize ext for filename (remove dot)
	safe := strings.TrimPrefix(ext, ".")
	if safe == "" {
		safe = "__unknown"
	}
	file := "filetype_" + safe + "_" + intToString(size) + ".png"
	return filepath.Join(util.GetLocation().GetImageCacheDirectory(), file)
}

// buildPathCachePath returns the cache file path for a given file path and size (in px).
func buildPathCachePath(filePath string, size int) string {
	hash := util.Md5([]byte(filePath))
	file := "fileicon_" + hash + "_" + intToString(size) + ".png"
	return filepath.Join(util.GetLocation().GetImageCacheDirectory(), file)
}

// intToString avoids fmt for tiny helper to keep deps minimal here
func intToString(v int) string {
	// very small helper
	if v == 0 {
		return "0"
	}
	neg := false
	if v < 0 {
		neg = true
		v = -v
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
