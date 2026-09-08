package recentfiles

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const DefaultLimit = 50

// File is one OS-tracked recently accessed path.
type File struct {
	Path     string
	LastUsed time.Time
}

// ListOption caps how many existing recent paths are returned.
type ListOption struct {
	Limit int
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return DefaultLimit
	}
	return limit
}

func recentFileKey(path string) string {
	cleaned := filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(cleaned)
	}
	return cleaned
}

// keepExistingRecentFile drops blanks and duplicate paths after OS listing.
func keepExistingRecentFile(path string, seen map[string]bool) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	key := recentFileKey(path)
	if seen[key] {
		return false
	}
	seen[key] = true
	return true
}

// List returns recently accessed files from the current OS.
func List(ctx context.Context, option ListOption) ([]File, error) {
	return listRecent(ctx, option)
}

// RecordAccess tells the OS that path was opened so later List calls can include it.
func RecordAccess(ctx context.Context, path string) error {
	return recordAccess(ctx, path)
}
