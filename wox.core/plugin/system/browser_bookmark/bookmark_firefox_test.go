package browserbookmark

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"wox/plugin"
	"wox/plugin/plugintest"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

func getLocalFirefoxRootDirsForTest() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"~/Library/Application Support/Firefox"}
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return nil
		}
		return []string{filepath.Join(appData, "Mozilla", "Firefox")}
	case "linux":
		return []string{
			"~/.mozilla/firefox",
			"~/snap/firefox/common/.mozilla/firefox",
			"~/.var/app/org.mozilla.firefox/.mozilla/firefox",
		}
	default:
		return nil
	}
}

func TestBrowserBookmarkPlugin_LoadFirefoxBookmarksFromLocalProfile(t *testing.T) {
	plugin := &BrowserBookmarkPlugin{api: &mockAPI{}}
	rootDirs := getLocalFirefoxRootDirsForTest()
	if len(rootDirs) == 0 {
		t.Skip("no local Firefox root directories for current OS")
	}

	profileDirs := plugin.resolveFirefoxProfileDirs(context.Background(), rootDirs, "Firefox")
	if len(profileDirs) == 0 {
		t.Skipf("no Firefox profiles found under roots: %v", rootDirs)
	}

	t.Logf("Firefox roots: %v", rootDirs)
	t.Logf("Firefox profiles: %v", profileDirs)

	bookmarks := plugin.loadFirefoxBookmarks(context.Background())
	t.Logf("Loaded %d Firefox bookmarks", len(bookmarks))
	for i, bookmark := range bookmarks {
		if i >= 10 {
			break
		}
		t.Logf("[%d] %s -> %s", i+1, bookmark.Name, bookmark.Url)
	}

	assert.Greater(t, len(bookmarks), 0, "expected at least one local Firefox bookmark")
}

// Mock API for testing
type mockAPI struct {
	plugintest.StubAPI
}

func (m *mockAPI) Log(ctx context.Context, level plugin.LogLevel, msg string) {
	// Output log messages during testing
	if testing.Verbose() {
		println("LOG:", msg)
	}
}

func (m *mockAPI) GetTranslation(ctx context.Context, key string) string { return key }
