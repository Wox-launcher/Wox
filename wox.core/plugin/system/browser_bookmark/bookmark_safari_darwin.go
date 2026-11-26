package browserbookmark

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"wox/util"
	"wox/util/browser"

	"howett.net/plist"
)

type safariBookmarkType string

const (
	safariBookmarkList safariBookmarkType = "WebBookmarkTypeList"
	safariBookmarkLeaf safariBookmarkType = "WebBookmarkTypeLeaf"
)

type safariBookmarkNode struct {
	Type     safariBookmarkType   `plist:"WebBookmarkType"`
	Title    string               `plist:"Title"`
	URL      string               `plist:"URLString"`
	Children []safariBookmarkNode `plist:"Children"`
	URI      struct {
		Title string `plist:"title"`
	} `plist:"URIDictionary"`
}

func init() {
	registerBrowserBookmarkLoader(browser.BrowserIDSafari, func(_ *BrowserBookmarkPlugin, ctx context.Context) []Bookmark {
		return loadSafariBookmarks(ctx)
	})
}

// loadSafariBookmarks reads the current user's Safari bookmarks and reports macOS access failures.
func loadSafariBookmarks(ctx context.Context) []Bookmark {
	home, err := os.UserHomeDir()
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("cannot locate Safari bookmarks: %v", err))
		return nil
	}

	bookmarks, err := readSafariBookmarks(filepath.Join(home, "Library", "Safari", "Bookmarks.plist"))
	switch {
	case err == nil:
		return bookmarks
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case errors.Is(err, fs.ErrPermission):
		util.GetLogger().Warn(ctx, fmt.Sprintf("cannot read Safari bookmarks: %v. Grant Wox Full Disk Access and restart it", err))
	default:
		util.GetLogger().Warn(ctx, fmt.Sprintf("cannot load Safari bookmarks: %v", err))
	}
	return nil
}

// readSafariBookmarks flattens the bookmark tree while skipping Safari's system Reading List folder.
func readSafariBookmarks(path string) ([]Bookmark, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var root safariBookmarkNode
	if _, err := plist.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse Safari bookmark plist: %w", err)
	}

	var bookmarks []Bookmark
	for _, child := range root.Children {
		if child.Type == safariBookmarkList && child.Title == "com.apple.ReadingList" {
			continue
		}
		child.appendBookmarks(&bookmarks)
	}
	return bookmarks, nil
}

// appendBookmarks visits folders in Safari order and uses the URL for untitled bookmarks.
func (node safariBookmarkNode) appendBookmarks(bookmarks *[]Bookmark) {
	switch node.Type {
	case safariBookmarkList:
		for _, child := range node.Children {
			child.appendBookmarks(bookmarks)
		}
	case safariBookmarkLeaf:
		url := strings.TrimSpace(node.URL)
		if url == "" {
			return
		}
		name := strings.TrimSpace(node.URI.Title)
		if name == "" {
			name = url
		}
		*bookmarks = append(*bookmarks, Bookmark{Name: name, Url: url, Icon: browserBookmarkIcon})
	}
}
