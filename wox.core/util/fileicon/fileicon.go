package fileicon

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"wox/util"
)

const fileIconPathCachePrefix = "fileicon_v5_"

// GetFileIconByPath returns the default list-size cached OS icon path for the given path.
// It first tries to resolve the application/file icon, then falls back to the file-type icon.
func GetFileIconByPath(ctx context.Context, filePath string) (string, error) {
	return GetFileIconByPathWithSize(ctx, filePath, util.ResultListIconSize)
}

func GetFileIconByPathWithSize(ctx context.Context, filePath string, size int) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	// Fileicon caches are shared by list and grid surfaces. Keep the requested
	// size in the cache key so a grid result never reuses the compact list icon.
	if size <= 0 {
		size = util.ResultListIconSize
	}

	iconPath, err := getFileIconImpl(ctx, filePath, size)
	if err == nil && strings.TrimSpace(iconPath) != "" {
		return iconPath, nil
	}

	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(filePath)))
	if ext == "" {
		// Unknown extension – treat as generic
		ext = ".__unknown"
	}
	return GetFileTypeIconWithSize(ctx, ext, size)
}

func CleanFileIconCache(ctx context.Context, filePath string) error {
	// Only remove cache entries produced by the current rendering strategy.
	// When the icon pipeline changes, bump fileIconPathCachePrefix so old
	// cache files naturally stop being referenced instead of carrying legacy
	// cleanup rules for every retired size.
	cacheSizes := []int{util.ResultListIconSize, util.ResultGridIconSize}
	seenSizes := map[int]struct{}{}
	for _, size := range cacheSizes {
		if _, ok := seenSizes[size]; ok {
			continue
		}
		seenSizes[size] = struct{}{}

		// Bug fix: path icon caches now include the source mtime so one app can
		// leave multiple historical files behind after updates. Clean every
		// current-version cache for the path+size pair because callers do not know
		// the old executable mtime they want to discard.
		cachePaths, globErr := filepath.Glob(buildPathCacheGlob(filePath, size))
		if globErr != nil {
			return globErr
		}
		for _, cachePath := range cachePaths {
			if removeErr := os.Remove(cachePath); removeErr != nil && !os.IsNotExist(removeErr) {
				return removeErr
			}
		}
	}

	return nil
}

// GetFileTypeIcon returns the default list-size cached OS file-type icon path for the given extension.
// The ext can be with or without leading dot.
func GetFileTypeIcon(ctx context.Context, ext string) (string, error) {
	return GetFileTypeIconWithSize(ctx, ext, util.ResultListIconSize)
}

func GetFileTypeIconWithSize(ctx context.Context, ext string, size int) (string, error) {
	if size <= 0 {
		size = util.ResultListIconSize
	}
	if ext == "" {
		ext = ".__unknown"
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return getFileTypeIconImpl(ctx, ext, size)
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

func buildPathCacheStem(filePath string, size int) string {
	hash := util.Md5([]byte(filePath))
	return fileIconPathCachePrefix + hash + "_" + intToString(size) + "_"
}

// buildPathCachePath returns the cache file path for a given file path, source mtime, and size (in px).
func buildPathCachePath(filePath string, size int, sourceModifiedUnix int64) string {
	// Bug fix: the old cache key used only the source path. When an executable
	// kept the same path but shipped a new embedded icon, Wox kept returning the
	// old PNG and the resize cache also stayed stale. Include the source mtime so
	// updated app binaries naturally produce a new icon path and downstream cache key.
	file := buildPathCacheStem(filePath, size) + strconv.FormatInt(sourceModifiedUnix, 10) + ".png"
	return filepath.Join(util.GetLocation().GetImageCacheDirectory(), file)
}

func buildPathCacheGlob(filePath string, size int) string {
	return filepath.Join(util.GetLocation().GetImageCacheDirectory(), buildPathCacheStem(filePath, size)+"*.png")
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
