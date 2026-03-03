package browserbookmark

import (
	"context"
	"fmt"
	"wox/util"
	"wox/util/browser"
)

var edgeBookmarkProfiles = []string{"Default", "Profile 1", "Profile 2", "Profile 3"}

func init() {
	registerBrowserBookmarkLoader(browser.BrowserIDEdge, func(c *BrowserBookmarkPlugin, ctx context.Context) []Bookmark {
		var bookmarks []Bookmark
		for _, profile := range edgeBookmarkProfiles {
			bookmarks = append(bookmarks, c.loadEdgeBookmark(ctx, profile)...)
		}
		return bookmarks
	})
}

func (c *BrowserBookmarkPlugin) loadEdgeBookmark(ctx context.Context, profile string) []Bookmark {
	switch {
	case util.IsMacOS():
		return c.loadEdgeBookmarkInMacos(ctx, profile)
	case util.IsWindows():
		return c.loadEdgeBookmarkInWindows(ctx, profile)
	case util.IsLinux():
		return c.loadEdgeBookmarkInLinux(ctx, profile)
	default:
		return []Bookmark{}
	}
}

func (c *BrowserBookmarkPlugin) loadEdgeBookmarkInMacos(ctx context.Context, profile string) []Bookmark {
	return c.loadBookmarkFromFile(ctx, fmt.Sprintf("~/Library/Application Support/Microsoft Edge/%s/Bookmarks", profile), "Edge")
}

func (c *BrowserBookmarkPlugin) loadEdgeBookmarkInWindows(ctx context.Context, profile string) []Bookmark {
	// Use a different approach to avoid fmt.Sprintf converting %% to %
	path := "%%LOCALAPPDATA%%\\Microsoft\\Edge\\User Data\\" + profile + "\\Bookmarks"
	return c.loadBookmarkFromFile(ctx, path, "Edge")
}

func (c *BrowserBookmarkPlugin) loadEdgeBookmarkInLinux(ctx context.Context, profile string) []Bookmark {
	return c.loadBookmarkFromFile(ctx, fmt.Sprintf("~/.config/microsoft-edge/%s/Bookmarks", profile), "Edge")
}
