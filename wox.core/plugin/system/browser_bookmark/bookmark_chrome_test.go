package browserbookmark

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadChromiumBookmarkFiles_ReadsAccountBookmarksWhenBookmarksIsEmpty(t *testing.T) {
	profileDir := t.TempDir()
	writeChromiumBookmarkJSON(t, filepath.Join(profileDir, "Bookmarks"))
	writeChromiumBookmarkJSON(t, filepath.Join(profileDir, "AccountBookmarks"),
		chromiumBookmarkEntry{Name: "GitHub", URL: "https://github.com"},
		chromiumBookmarkEntry{Name: "Wox", URL: "https://github.com/Wox-launcher/Wox"},
	)
	require.NoError(t, os.WriteFile(filepath.Join(profileDir, "EncryptedBookmarks"), []byte("not-json"), 0o600))

	plugin := &BrowserBookmarkPlugin{api: &mockAPI{}}
	bookmarks := plugin.loadChromiumBookmarkFiles(context.Background(), profileDir, string(os.PathSeparator), "Chrome")

	assert.ElementsMatch(t, []string{"GitHub", "Wox"}, bookmarkNames(bookmarks))
}

func TestLoadChromiumBookmarkFiles_MergesLocalAndAccountBookmarks(t *testing.T) {
	profileDir := t.TempDir()
	writeChromiumBookmarkJSON(t, filepath.Join(profileDir, "Bookmarks"),
		chromiumBookmarkEntry{Name: "Local Docs", URL: "https://local.example/docs"},
	)
	writeChromiumBookmarkJSON(t, filepath.Join(profileDir, "AccountBookmarks"),
		chromiumBookmarkEntry{Name: "Account Docs", URL: "https://account.example/docs"},
	)

	plugin := &BrowserBookmarkPlugin{api: &mockAPI{}}
	bookmarks := plugin.loadChromiumBookmarkFiles(context.Background(), profileDir, string(os.PathSeparator), "Chrome")

	assert.ElementsMatch(t, []string{"Local Docs", "Account Docs"}, bookmarkNames(bookmarks))
}

func TestLoadChromiumBookmarkFiles_IgnoresMissingAccountBookmarks(t *testing.T) {
	profileDir := t.TempDir()
	writeChromiumBookmarkJSON(t, filepath.Join(profileDir, "Bookmarks"),
		chromiumBookmarkEntry{Name: "Local Only", URL: "https://local.example"},
	)

	plugin := &BrowserBookmarkPlugin{api: &mockAPI{}}
	bookmarks := plugin.loadChromiumBookmarkFiles(context.Background(), profileDir, string(os.PathSeparator), "Chrome")

	assert.Equal(t, []string{"Local Only"}, bookmarkNames(bookmarks))
}

type chromiumBookmarkEntry struct {
	Name string
	URL  string
}

func writeChromiumBookmarkJSON(t *testing.T, path string, entries ...chromiumBookmarkEntry) {
	t.Helper()

	children := make([]string, 0, len(entries))
	for i, entry := range entries {
		children = append(children, fmt.Sprintf(`{
          "date_added": "13300000000000000",
          "guid": "00000000-0000-0000-0000-00000000000%d",
          "id": "%d",
          "name": "%s",
          "type": "url",
          "url": "%s"
        }`, i+1, i+5, entry.Name, entry.URL))
	}

	content := `{
  "checksum": "00000000000000000000000000000000",
  "roots": {
    "bookmark_bar": {
      "children": [` + strings.Join(children, ",") + `],
      "date_added": "13300000000000000",
      "date_modified": "0",
      "guid": "00000000-0000-4000-a000-000000000001",
      "id": "1",
      "name": "Bookmarks bar",
      "type": "folder"
    },
    "other": {
      "children": [],
      "date_added": "13300000000000000",
      "date_modified": "0",
      "guid": "00000000-0000-4000-a000-000000000002",
      "id": "2",
      "name": "Other bookmarks",
      "type": "folder"
    },
    "synced": {
      "children": [],
      "date_added": "13300000000000000",
      "date_modified": "0",
      "guid": "00000000-0000-4000-a000-000000000003",
      "id": "3",
      "name": "Mobile bookmarks",
      "type": "folder"
    }
  },
  "version": 1
}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func bookmarkNames(bookmarks []Bookmark) []string {
	names := make([]string, 0, len(bookmarks))
	for _, bookmark := range bookmarks {
		names = append(names, bookmark.Name)
	}
	return names
}
