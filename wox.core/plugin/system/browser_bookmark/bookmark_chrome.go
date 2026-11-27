package browserbookmark

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"wox/plugin"
	"wox/plugin/system"
	"wox/util"
	"wox/util/browser"
)

var chromeBookmarkProfiles = []string{"Default", "Profile 1", "Profile 2", "Profile 3"}

// chromiumBookmarkFileNames are the plaintext JSON files Chromium browsers write in a profile.
// Chrome 146+ with account bookmark sync stores the real bookmarks in AccountBookmarks and
// leaves Bookmarks as an empty skeleton. EncryptedBookmarks is OS-encrypted and is not readable here.
var chromiumBookmarkFileNames = []string{"Bookmarks", "AccountBookmarks"}

type chromiumBookmarkNodeType string

const (
	chromiumBookmarkFolder chromiumBookmarkNodeType = "folder"
	chromiumBookmarkURL    chromiumBookmarkNodeType = "url"
)

type chromiumBookmarkNode struct {
	Type     chromiumBookmarkNodeType `json:"type"`
	Name     string                   `json:"name"`
	URL      string                   `json:"url"`
	Children []chromiumBookmarkNode   `json:"children"`
}

func init() {
	registerBrowserBookmarkLoader(browser.BrowserIDChrome, func(c *BrowserBookmarkPlugin, ctx context.Context) []Bookmark {
		var bookmarks []Bookmark
		for _, profile := range chromeBookmarkProfiles {
			bookmarks = append(bookmarks, c.loadChromeBookmark(ctx, profile)...)
		}
		return bookmarks
	})
}

func (c *BrowserBookmarkPlugin) loadChromeBookmark(ctx context.Context, profile string) []Bookmark {
	switch {
	case util.IsMacOS():
		return c.loadChromeBookmarkInMacos(ctx, profile)
	case util.IsWindows():
		return c.loadChromeBookmarkInWindows(ctx, profile)
	case util.IsLinux():
		return c.loadChromeBookmarkInLinux(ctx, profile)
	default:
		return []Bookmark{}
	}
}

func (c *BrowserBookmarkPlugin) loadChromeBookmarkInMacos(ctx context.Context, profile string) []Bookmark {
	return c.loadChromiumBookmarkFiles(ctx, fmt.Sprintf("~/Library/Application Support/Google/Chrome/%s", profile), "/", "Chrome")
}

func (c *BrowserBookmarkPlugin) loadChromeBookmarkInWindows(ctx context.Context, profile string) []Bookmark {
	// Use a different approach to avoid fmt.Sprintf converting %% to %
	profileDir := "%%LOCALAPPDATA%%\\Google\\Chrome\\User Data\\" + profile
	return c.loadChromiumBookmarkFiles(ctx, profileDir, "\\", "Chrome")
}

func (c *BrowserBookmarkPlugin) loadChromeBookmarkInLinux(ctx context.Context, profile string) []Bookmark {
	var bookmarks []Bookmark
	for _, root := range getLinuxBookmarkRootDirs(browser.BrowserIDChrome) {
		bookmarks = append(bookmarks, c.loadChromiumBookmarkFiles(ctx, filepath.Join(root, profile), "/", "Chrome")...)
	}
	return bookmarks
}

// loadChromiumBookmarkFiles reads plaintext Chromium bookmark JSON files from a profile directory.
func (c *BrowserBookmarkPlugin) loadChromiumBookmarkFiles(ctx context.Context, profileDir string, separator string, browserName string) []Bookmark {
	var bookmarks []Bookmark
	for _, fileName := range chromiumBookmarkFileNames {
		bookmarks = append(bookmarks, c.loadBookmarkFromFile(ctx, profileDir+separator+fileName, browserName)...)
	}
	return bookmarks
}

func (c *BrowserBookmarkPlugin) loadBookmarkFromFile(ctx context.Context, bookmarkPath string, browserName string) []Bookmark {
	var bookmarkLocation string
	var err error

	if strings.Contains(bookmarkPath, "%%LOCALAPPDATA%%") {
		// Windows path with environment variable
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			return []Bookmark{}
		}
		bookmarkLocation = strings.Replace(bookmarkPath, "%%LOCALAPPDATA%%", localAppData, 1)
	} else {
		// Unix-style path
		bookmarkLocation, err = util.ExpandHomeDir(bookmarkPath)
		if err != nil {
			c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error expanding %s bookmark path: %s", browserName, err.Error()))
			return []Bookmark{}
		}
	}

	if _, err := os.Stat(bookmarkLocation); os.IsNotExist(err) {
		return []Bookmark{}
	}

	file, readErr := os.ReadFile(bookmarkLocation)
	if readErr != nil {
		c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error reading %s bookmark file: %s", browserName, readErr.Error()))
		return []Bookmark{}
	}

	// Decode the tree so a folder name cannot be paired with a later bookmark's URL.
	var tree struct {
		Roots map[string]chromiumBookmarkNode `json:"roots"`
	}
	if err := json.Unmarshal(file, &tree); err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("error parsing %s bookmark file %s: %v", browserName, bookmarkLocation, err))
		return nil
	}

	var results []Bookmark
	for _, root := range slices.Sorted(maps.Keys(tree.Roots)) {
		results = tree.Roots[root].appendBookmarks(ctx, results)
	}
	return results
}

// appendBookmarks collects URL nodes depth-first and reuses cached favicons.
func (node chromiumBookmarkNode) appendBookmarks(ctx context.Context, results []Bookmark) []Bookmark {
	switch node.Type {
	case chromiumBookmarkFolder:
		for _, child := range node.Children {
			results = child.appendBookmarks(ctx, results)
		}
	case chromiumBookmarkURL:
		icon := browserBookmarkIcon
		if cachedIcon, ok := system.GetWebsiteIconFromCacheOnly(ctx, node.URL); ok {
			icon = cachedIcon
		}
		results = append(results, Bookmark{Name: node.Name, Url: node.URL, Icon: icon})
	}
	return results
}
