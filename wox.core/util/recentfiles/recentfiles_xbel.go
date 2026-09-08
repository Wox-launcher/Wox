package recentfiles

import (
	"encoding/xml"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type xbelDocument struct {
	XMLName   xml.Name       `xml:"xbel"`
	Bookmarks []xbelBookmark `xml:"bookmark"`
}

type xbelBookmark struct {
	Href     string `xml:"href,attr"`
	Added    string `xml:"added,attr"`
	Modified string `xml:"modified,attr"`
	Visited  string `xml:"visited,attr"`
}

type parsedRecentBookmark struct {
	path     string
	lastUsed time.Time
}

func parseRecentlyUsedBookmarks(path string) ([]parsedRecentBookmark, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseRecentlyUsedXBEL(data)
}

// parseRecentlyUsedXBEL reads Freedesktop desktop-bookmark entries from recently-used.xbel.
func parseRecentlyUsedXBEL(data []byte) ([]parsedRecentBookmark, error) {
	var document xbelDocument
	if err := xml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("parse recently-used.xbel: %w", err)
	}

	bookmarks := make([]parsedRecentBookmark, 0, len(document.Bookmarks))
	for _, bookmark := range document.Bookmarks {
		path, ok := fileURIToPath(bookmark.Href)
		if !ok {
			continue
		}
		bookmarks = append(bookmarks, parsedRecentBookmark{
			path:     path,
			lastUsed: bookmarkLastUsed(bookmark),
		})
	}
	return bookmarks, nil
}

func recentlyUsedXBELPath() string {
	if dir := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); dir != "" {
		return filepath.Join(dir, "recently-used.xbel")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "recently-used.xbel")
}

// fileURIToPath converts a local file:// bookmark href into a filesystem path.
func fileURIToPath(href string) (string, bool) {
	trimmed := strings.TrimSpace(href)
	if trimmed == "" {
		return "", false
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || !strings.EqualFold(parsed.Scheme, "file") {
		return "", false
	}
	if parsed.Host != "" && !strings.EqualFold(parsed.Host, "localhost") {
		return "", false
	}
	path := parsed.Path
	if path == "" {
		return "", false
	}
	if runtime.GOOS == "windows" && strings.HasPrefix(path, "/") && len(path) >= 3 && path[2] == ':' {
		path = path[1:]
	}
	return filepath.Clean(path), true
}

func bookmarkLastUsed(bookmark xbelBookmark) time.Time {
	for _, raw := range []string{bookmark.Modified, bookmark.Visited, bookmark.Added} {
		if parsed, ok := parseXBELTime(raw); ok {
			return parsed
		}
	}
	return time.Time{}
}

func parseXBELTime(raw string) (time.Time, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, trimmed); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}
