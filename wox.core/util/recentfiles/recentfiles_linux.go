//go:build linux

package recentfiles

import (
	"context"
	"os"
	"sort"
)

// listRecent reads ~/.local/share/recently-used.xbel and keeps paths that still exist.
func listRecent(ctx context.Context, option ListOption) ([]File, error) {
	_ = ctx
	limit := normalizeLimit(option.Limit)
	bookmarks, err := parseRecentlyUsedBookmarks(recentlyUsedXBELPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	sort.SliceStable(bookmarks, func(i, j int) bool {
		return bookmarks[i].lastUsed.After(bookmarks[j].lastUsed)
	})

	seen := make(map[string]bool, limit)
	results := make([]File, 0, limit)
	for _, bookmark := range bookmarks {
		if len(results) >= limit {
			break
		}
		if !keepExistingRecentFile(bookmark.path, seen) {
			continue
		}
		if _, statErr := os.Lstat(bookmark.path); statErr != nil {
			continue
		}
		results = append(results, File{
			Path:     bookmark.path,
			LastUsed: bookmark.lastUsed,
		})
	}
	return results, nil
}

func recordAccess(_ context.Context, _ string) error {
	// recently-used.xbel is owned by GTK and other desktop apps. Opening through
	// xdg-open already updates it when the target application participates.
	return nil
}
