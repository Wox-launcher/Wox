package browserbookmark

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"wox/plugin"
	"wox/plugin/system"
	"wox/util"
	"wox/util/browser"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mitchellh/go-homedir"
)

func init() {
	registerBrowserBookmarkLoader(browser.BrowserIDFirefox, func(c *BrowserBookmarkPlugin, ctx context.Context) []Bookmark {
		return c.loadFirefoxBookmarks(ctx)
	})
}

func (c *BrowserBookmarkPlugin) loadFirefoxBookmarks(ctx context.Context) []Bookmark {
	var roots []string

	switch {
	case util.IsMacOS():
		roots = []string{"~/Library/Application Support/Firefox"}
	case util.IsWindows():
		appData := os.Getenv("APPDATA")
		if appData != "" {
			roots = append(roots, filepath.Join(appData, "Mozilla", "Firefox"))
		}
	case util.IsLinux():
		roots = []string{
			"~/.mozilla/firefox",
			"~/snap/firefox/common/.mozilla/firefox",
			"~/.var/app/org.mozilla.firefox/.mozilla/firefox",
		}
	default:
		return []Bookmark{}
	}

	return c.loadFirefoxBookmarksFromRootDirs(ctx, roots, "Firefox")
}

func (c *BrowserBookmarkPlugin) loadFirefoxBookmarksFromRootDirs(ctx context.Context, rootDirs []string, browserName string) []Bookmark {
	profileDirs := c.resolveFirefoxProfileDirs(ctx, rootDirs, browserName)
	if len(profileDirs) == 0 {
		c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("%s profiles not found in candidate roots", browserName))
		return []Bookmark{}
	}

	var bookmarks []Bookmark
	for _, profileDir := range profileDirs {
		placesFile := filepath.Join(profileDir, "places.sqlite")
		bookmarks = append(bookmarks, c.loadFirefoxBookmarkFromPlacesFile(ctx, placesFile)...)
	}

	return bookmarks
}

func (c *BrowserBookmarkPlugin) resolveFirefoxProfileDirs(ctx context.Context, rootDirs []string, browserName string) []string {
	seen := map[string]struct{}{}
	var profileDirs []string

	addProfileDir := func(dir string) {
		if dir == "" {
			return
		}
		if _, ok := seen[dir]; ok {
			return
		}
		seen[dir] = struct{}{}
		profileDirs = append(profileDirs, dir)
	}

	for _, rootDir := range rootDirs {
		resolvedRoot, err := homedir.Expand(rootDir)
		if err != nil {
			c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error expanding %s root path: %s", browserName, err.Error()))
			continue
		}

		for _, profileDir := range c.resolveFirefoxProfileDirsByRoot(ctx, resolvedRoot) {
			addProfileDir(profileDir)
		}
	}

	return profileDirs
}

func (c *BrowserBookmarkPlugin) resolveFirefoxProfileDirsByRoot(ctx context.Context, rootDir string) []string {
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		if !os.IsNotExist(err) {
			c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error reading Firefox root directory: %s", err.Error()))
		}
		return []string{}
	}

	_ = entries

	var profileDirs []string
	seen := map[string]struct{}{}
	add := func(dir string) {
		if dir == "" {
			return
		}
		if _, ok := seen[dir]; ok {
			return
		}
		seen[dir] = struct{}{}
		profileDirs = append(profileDirs, dir)
	}

	// 1) Prefer profiles.ini because it contains the exact active profile locations.
	profilesIni := filepath.Join(rootDir, "profiles.ini")
	if dirs, err := c.parseFirefoxProfilesIni(profilesIni, rootDir); err == nil {
		for _, dir := range dirs {
			add(dir)
		}
	}

	// 2) Fallback scan common locations for profile folders that contain places.sqlite.
	for _, baseDir := range []string{rootDir, filepath.Join(rootDir, "Profiles")} {
		baseEntries, err := os.ReadDir(baseDir)
		if err != nil {
			continue
		}
		for _, entry := range baseEntries {
			if !entry.IsDir() {
				continue
			}
			dir := filepath.Join(baseDir, entry.Name())
			if util.IsFileExists(filepath.Join(dir, "places.sqlite")) {
				add(dir)
			}
		}
	}

	return profileDirs
}

func (c *BrowserBookmarkPlugin) parseFirefoxProfilesIni(profilesIniPath string, rootDir string) ([]string, error) {
	file, err := os.Open(profilesIniPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	type profileConfig struct {
		path       string
		isRelative bool
	}

	var result []string
	seen := map[string]struct{}{}
	currentSection := ""
	current := profileConfig{isRelative: true}

	flushCurrent := func() {
		if !strings.HasPrefix(currentSection, "Profile") {
			return
		}
		if strings.TrimSpace(current.path) == "" {
			return
		}

		profileDir := strings.TrimSpace(current.path)
		if current.isRelative && !filepath.IsAbs(profileDir) {
			profileDir = filepath.Join(rootDir, profileDir)
		}
		profileDir = filepath.Clean(profileDir)

		if _, ok := seen[profileDir]; ok {
			return
		}
		seen[profileDir] = struct{}{}
		result = append(result, profileDir)
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			flushCurrent()
			currentSection = strings.TrimSpace(line[1 : len(line)-1])
			current = profileConfig{isRelative: true}
			continue
		}

		if !strings.HasPrefix(currentSection, "Profile") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		switch strings.TrimSpace(strings.ToLower(key)) {
		case "path":
			current.path = strings.TrimSpace(value)
		case "isrelative":
			current.isRelative = strings.TrimSpace(value) != "0"
		}
	}

	if scannerErr := scanner.Err(); scannerErr != nil {
		return result, scannerErr
	}

	flushCurrent()
	return result, nil
}

func (c *BrowserBookmarkPlugin) loadFirefoxBookmarkFromPlacesFile(ctx context.Context, placesFile string) []Bookmark {
	if _, err := os.Stat(placesFile); os.IsNotExist(err) {
		return []Bookmark{}
	}

	db, err := sql.Open("sqlite3", placesFile+"?mode=ro&_busy_timeout=2000")
	if err != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error opening Firefox places database: %s", err.Error()))
		return []Bookmark{}
	}
	defer db.Close()

	rows, queryErr := db.Query(`
		SELECT b.title, p.url
		FROM moz_bookmarks b
		INNER JOIN moz_places p ON b.fk = p.id
		WHERE b.type = 1
		  AND p.url IS NOT NULL
		  AND p.url <> ''
	`)
	if queryErr != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error querying Firefox places database: %s", queryErr.Error()))
		return []Bookmark{}
	}
	defer rows.Close()

	var bookmarks []Bookmark
	for rows.Next() {
		var title sql.NullString
		var url sql.NullString
		if scanErr := rows.Scan(&title, &url); scanErr != nil {
			continue
		}

		if !url.Valid {
			continue
		}

		trimmedURL := strings.TrimSpace(url.String)
		if trimmedURL == "" {
			continue
		}

		name := strings.TrimSpace(title.String)
		if name == "" {
			name = trimmedURL
		}

		icon := browserBookmarkIcon
		if cachedIcon, ok := system.GetWebsiteIconFromCacheOnly(ctx, trimmedURL); ok {
			icon = cachedIcon
		}

		bookmarks = append(bookmarks, Bookmark{
			Name: name,
			Url:  trimmedURL,
			Icon: icon,
		})
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error iterating Firefox places rows: %s", rowsErr.Error()))
	}

	return bookmarks
}
