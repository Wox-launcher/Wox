package filesearch

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var (
	cachedUserHomeDirOnce sync.Once
	cachedUserHomeDirPath string
)

// cachedUserHomeDir returns the process-stable home path used by hot traversal
// filters without calling os.UserHomeDir for every scanned entry.
func cachedUserHomeDir() string {
	cachedUserHomeDirOnce.Do(func() {
		homeDir, err := os.UserHomeDir()
		if err != nil || strings.TrimSpace(homeDir) == "" {
			return
		}
		cachedUserHomeDirPath = filepath.Clean(homeDir)
	})
	return cachedUserHomeDirPath
}

func shouldSkipSystemPathForRoot(root RootRecord, fullPath string, isDir bool) bool {
	if shouldSkipSystemPath(fullPath, isDir) {
		return true
	}
	if shouldSkipWindowsDriveRootSystemPath(root, fullPath, isDir) {
		return true
	}
	if !isDir {
		return false
	}
	return shouldSkipDarwinHomeNoisePath(root, fullPath)
}

// shouldSkipWindowsDriveRootSystemPath ignores OS-owned entries that Windows
// creates at drive roots and that normal user processes cannot reliably read.
func shouldSkipWindowsDriveRootSystemPath(root RootRecord, fullPath string, isDir bool) bool {
	if runtime.GOOS != "windows" {
		return false
	}

	cleanRoot := filepath.Clean(strings.TrimSpace(root.Path))
	cleanPath := filepath.Clean(strings.TrimSpace(fullPath))
	if cleanRoot == "" || cleanRoot == "." || cleanPath == "" || cleanPath == "." {
		return false
	}

	rootVolume := filepath.VolumeName(cleanRoot)
	if rootVolume == "" {
		return false
	}
	driveRoot := filepath.Clean(rootVolume + string(filepath.Separator))
	if !strings.EqualFold(cleanRoot, driveRoot) {
		return false
	}

	relPath, err := filepath.Rel(driveRoot, cleanPath)
	if err != nil || relPath == "." || relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return false
	}

	segments := strings.Split(relPath, string(filepath.Separator))
	if len(segments) != 1 {
		return false
	}

	name := strings.ToLower(segments[0])
	if isDir {
		switch name {
		case "system volume information", "$recycle.bin":
			return true
		}
		return false
	}

	switch name {
	case "hiberfil.sys", "pagefile.sys", "swapfile.sys", "dumpstack.log.tmp":
		return true
	}
	return false
}

func shouldSkipDarwinHomeNoisePath(root RootRecord, fullPath string) bool {
	if runtime.GOOS != "darwin" {
		return false
	}

	cleanHome := cachedUserHomeDir()
	if cleanHome == "" {
		return false
	}

	cleanRoot := filepath.Clean(strings.TrimSpace(root.Path))
	cleanPath := filepath.Clean(strings.TrimSpace(fullPath))
	if cleanRoot != cleanHome || cleanPath == cleanRoot {
		return false
	}

	relPath, err := filepath.Rel(cleanHome, cleanPath)
	if err != nil || relPath == "." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) || relPath == ".." {
		return false
	}

	segments := strings.Split(relPath, string(filepath.Separator))
	if len(segments) == 0 {
		return false
	}

	// Optimization: a configured home root should behave like launcher file
	// search, not a full `find ~/` crawl. macOS keeps high-churn, protected app
	// state under ~/Library, and traversing it dominated real-index captures while
	// producing noisy launcher results. If the user explicitly adds ~/Library as
	// its own root, cleanRoot no longer equals the home directory and this pruning
	// does not apply.
	if segments[0] == "Library" {
		return true
	}

	if len(segments) == 2 && segments[0] == "Music" && segments[1] == "Music" {
		return true
	}

	if len(segments) >= 2 && segments[0] == "Pictures" && strings.HasSuffix(strings.ToLower(filepath.Base(cleanPath)), ".photoslibrary") {
		return true
	}

	return false
}
