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

// CleanupExpired removes image cache files that have not been touched within the retention window.
func CleanupExpired(ctx context.Context) (int, error) {
	cacheDir := util.GetLocation().GetImageCacheDirectory()
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	cutoff := time.Now().Add(-retentionAge)
	removedCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, infoErr := entry.Info()
		if infoErr != nil {
			util.GetLogger().Debug(ctx, fmt.Sprintf("failed to read image cache file info: name=%s err=%s", entry.Name(), infoErr.Error()))
			continue
		}
		if !info.ModTime().Before(cutoff) {
			continue
		}

		cachePath := filepath.Join(cacheDir, entry.Name())
		if removeErr := os.Remove(cachePath); removeErr != nil {
			if os.IsNotExist(removeErr) {
				forgetMemoryState(cachePath)
				continue
			}
			util.GetLogger().Warn(ctx, fmt.Sprintf("failed to remove expired image cache file: path=%s err=%s", cachePath, removeErr.Error()))
			continue
		}
		forgetMemoryState(cachePath)
		removedCount++
	}

	return removedCount, nil
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
	if !strings.EqualFold(filepath.Dir(cleanPath), cacheDir) {
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
