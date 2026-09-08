package imagecache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"wox/util"
)

const (
	touchInterval = 5 * time.Hour
	retentionAge  = 14 * 24 * time.Hour

	// CleanupInterval is the scheduled cadence for image cache maintenance.
	CleanupInterval = 6 * time.Hour
)

var touchState = struct {
	mu       sync.Mutex
	attempts map[string]time.Time
}{
	attempts: map[string]time.Time{},
}

var derivedPathExistenceCache = util.NewHashMap[string, struct{}]()

// Touch records that Wox used an image cache file without reading image contents.
func Touch(ctx context.Context, cachePath string, info os.FileInfo) {
	cleanPath, ok := managedCachePath(cachePath)
	if !ok {
		return
	}
	now := time.Now()
	if info != nil {
		if info.IsDir() || now.Sub(info.ModTime()) < touchInterval {
			return
		}
	}
	if !shouldTouchFromMemory(cleanPath, now) {
		return
	}

	if err := os.Chtimes(cleanPath, now, now); err != nil {
		util.GetLogger().Debug(ctx, fmt.Sprintf("failed to touch image cache file: path=%s err=%s", cleanPath, err.Error()))
	}
}

// RememberDerivedPathExists records a generated image cache path that has already been verified on disk.
func RememberDerivedPathExists(cachePath string) {
	cleanPath, ok := managedCachePath(cachePath)
	if !ok {
		return
	}

	derivedPathExistenceCache.Store(cleanPath, struct{}{})
}

// IsKnownExistingDerivedPath checks the in-memory positive cache for generated image cache files.
func IsKnownExistingDerivedPath(cachePath string) bool {
	cleanPath, ok := managedCachePath(cachePath)
	if !ok {
		return false
	}

	return derivedPathExistenceCache.Exist(cleanPath)
}

// ClearDerivedPathExistenceCache clears the positive cache for generated image cache paths.
func ClearDerivedPathExistenceCache() {
	derivedPathExistenceCache.Clear()
}

// RemoveDirectory removes only a descendant of the image cache and forgets
// positive disk lookups even after a partially successful deletion.
func RemoveDirectory(directory string) error {
	cleanDirectory, ok := managedCachePath(directory)
	if !ok {
		return fmt.Errorf("not an image cache subdirectory: %s", directory)
	}
	directory = cleanDirectory
	err := os.RemoveAll(directory)
	for _, filename := range derivedPathExistenceCache.Keys() {
		if pathWithin(directory, filename) {
			derivedPathExistenceCache.Delete(filename)
		}
	}
	touchState.mu.Lock()
	for filename := range touchState.attempts {
		if pathWithin(directory, filename) {
			delete(touchState.attempts, filename)
		}
	}
	touchState.mu.Unlock()
	return err
}

// CleanupExpired removes unused images, including plugin-owned subdirectories.
func CleanupExpired(ctx context.Context) (int, error) {
	cacheDir := util.GetLocation().GetImageCacheDirectory()
	cutoff := time.Now().Add(-retentionAge)
	removedCount := 0
	err := filepath.WalkDir(cacheDir, func(filename string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			if filename == cacheDir {
				return walkErr
			}
			util.GetLogger().Debug(ctx, fmt.Sprintf("failed to walk image cache: path=%s err=%v", filename, walkErr))
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			util.GetLogger().Debug(ctx, fmt.Sprintf("failed to read image cache file info: path=%s err=%v", filename, err))
			return nil
		}
		if !info.ModTime().Before(cutoff) {
			return nil
		}
		if err := os.Remove(filename); err != nil {
			if os.IsNotExist(err) {
				forgetMemoryState(filename)
				return nil
			}
			util.GetLogger().Warn(ctx, fmt.Sprintf("failed to remove expired image cache file: path=%s err=%s", filename, err))
			return nil
		}
		forgetMemoryState(filename)
		removedCount++
		return nil
	})
	return removedCount, err
}

// StartCleanupRoutine runs image cache cleanup once at startup and then on a fixed schedule.
func StartCleanupRoutine(ctx context.Context) {
	util.Go(ctx, "image cache cleanup", func() {
		runCleanup := func(cleanupCtx context.Context) {
			removedCount, err := CleanupExpired(cleanupCtx)
			if err != nil {
				util.GetLogger().Error(cleanupCtx, fmt.Sprintf("failed to cleanup image cache: %s", err.Error()))
				return
			}
			if removedCount > 0 {
				util.GetLogger().Info(cleanupCtx, fmt.Sprintf("cleaned up %d expired image cache files", removedCount))
			}
		}

		runCleanup(ctx)

		ticker := time.NewTicker(CleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runCleanup(ctx)
			}
		}
	})
}

func managedCachePath(cachePath string) (string, bool) {
	if strings.TrimSpace(cachePath) == "" {
		return "", false
	}

	cleanPath := filepath.Clean(cachePath)
	cacheDir := filepath.Clean(util.GetLocation().GetImageCacheDirectory())
	if !pathWithin(cacheDir, cleanPath) {
		return "", false
	}
	return cleanPath, true
}

func shouldTouchFromMemory(cleanPath string, now time.Time) bool {
	touchState.mu.Lock()
	defer touchState.mu.Unlock()

	lastAttempt, ok := touchState.attempts[cleanPath]
	if ok && now.Sub(lastAttempt) < touchInterval {
		return false
	}
	touchState.attempts[cleanPath] = now
	return true
}

func forgetTouchAttempt(cleanPath string) {
	touchState.mu.Lock()
	defer touchState.mu.Unlock()

	delete(touchState.attempts, filepath.Clean(cleanPath))
}

func forgetMemoryState(cachePath string) {
	cleanPath := filepath.Clean(cachePath)
	forgetTouchAttempt(cleanPath)
	derivedPathExistenceCache.Delete(cleanPath)
}

// pathWithin excludes the root itself and sibling paths with the same prefix.
func pathWithin(directory, filename string) bool {
	rel, err := filepath.Rel(directory, filename)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
