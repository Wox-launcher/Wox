package browserbookmark

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"wox/plugin"
	"wox/util/browser"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"howett.net/plist"
)

// TestSafariBookmarkIndexing exercises Safari discovery, selection, and listing with the b keyword.
func TestSafariBookmarkIndexing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Ensure discovery also works on runners without Safari in a system app directory.
	require.NoError(t, os.MkdirAll(filepath.Join(home, "Applications", "Safari.app"), 0o700))
	safariDir := filepath.Join(home, "Library", "Safari")
	require.NoError(t, os.MkdirAll(safariDir, 0o700))
	bookmarkPath := filepath.Join(safariDir, "Bookmarks.plist")

	url := "https://example.com/safari-docs"
	untitledURL := "https://example.com/no-title"
	leaf := func(title string, url string) map[string]any {
		return map[string]any{
			"WebBookmarkType": "WebBookmarkTypeLeaf",
			"URLString":       url,
			"URIDictionary":   map[string]string{"title": title},
		}
	}
	folder := func(title string, children ...map[string]any) map[string]any {
		return map[string]any{"WebBookmarkType": "WebBookmarkTypeList", "Title": title, "Children": children}
	}
	fixture := map[string]any{
		"WebBookmarkFileVersion": 1,
		"Children": []map[string]any{
			folder("BookmarksBar", folder("Work", leaf("Safari Docs", url))),
			folder("BookmarksMenu", leaf("", untitledURL), leaf("Missing URL", "")),
			folder("com.apple.ReadingList", leaf("Read Later", "https://example.com/read-later")),
			{"WebBookmarkType": "WebBookmarkTypeProxy", "Title": "History"},
		},
	}

	ctx := context.Background()
	p := &BrowserBookmarkPlugin{api: &safariBookmarkTestAPI{}}
	require.Equal(t, []string{browser.BrowserIDSafari}, p.getSelectedBookmarkBrowsers(ctx))
	for _, format := range []struct {
		name string
		id   int
	}{
		{name: "XML", id: plist.XMLFormat},
		{name: "binary", id: plist.BinaryFormat},
	} {
		t.Run(format.name, func(t *testing.T) {
			data, err := plist.Marshal(fixture, format.id)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(bookmarkPath, data, 0o600))

			p.bookmarks = p.loadBookmarks(ctx)
			results := p.Query(ctx, plugin.Query{Type: plugin.QueryTypeInput, TriggerKeyword: "b"}).Results
			require.Len(t, results, 2)
			assert.Equal(t, "Safari Docs", results[0].Title)
			assert.Equal(t, url, results[0].SubTitle)
			assert.Equal(t, untitledURL, results[1].Title)
		})
	}
}

type safariBookmarkTestAPI struct{ mockAPI }

func (*safariBookmarkTestAPI) GetSetting(context.Context, string) string {
	return browser.BrowserIDSafari
}
