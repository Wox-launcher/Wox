package browserbookmark

import (
	"context"
	"fmt"
	"os"
	"strings"
	"wox/plugin"
	"wox/plugin/system"
	"wox/util"
	"wox/util/browser"

	"github.com/mitchellh/go-homedir"
)

var chromeBookmarkProfiles = []string{"Default", "Profile 1", "Profile 2", "Profile 3"}

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
	return c.loadBookmarkFromFile(ctx, fmt.Sprintf("~/Library/Application Support/Google/Chrome/%s/Bookmarks", profile), "Chrome")
}

func (c *BrowserBookmarkPlugin) loadChromeBookmarkInWindows(ctx context.Context, profile string) []Bookmark {
	// Use a different approach to avoid fmt.Sprintf converting %% to %
	path := "%%LOCALAPPDATA%%\\Google\\Chrome\\User Data\\" + profile + "\\Bookmarks"
	return c.loadBookmarkFromFile(ctx, path, "Chrome")
}

func (c *BrowserBookmarkPlugin) loadChromeBookmarkInLinux(ctx context.Context, profile string) []Bookmark {
	return c.loadBookmarkFromFile(ctx, fmt.Sprintf("~/.config/google-chrome/%s/Bookmarks", profile), "Chrome")
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
		bookmarkLocation, err = homedir.Expand(bookmarkPath)
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

	// Use a more robust regex pattern that works for both Chrome and Edge bookmark formats
	var results []Bookmark
	groups := util.FindRegexGroups(`(?ms)"name": "(?P<name>[^"]*)",.*?"type": "url",.*?"url": "(?P<url>[^"]*)"`, string(file))

	for _, group := range groups {
		if name, nameOk := group["name"]; nameOk {
			if url, urlOk := group["url"]; urlOk {
				// Do not block on network here; show cached favicon if exists
				icon := browserBookmarkIcon
				if cachedIcon, ok := system.GetWebsiteIconFromCacheOnly(ctx, url); ok {
					icon = cachedIcon
				}

				results = append(results, Bookmark{
					Name: name,
					Url:  url,
					Icon: icon,
				})
			}
		}
	}

	return results
}
