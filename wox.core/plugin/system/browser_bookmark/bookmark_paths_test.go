package browserbookmark

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"wox/util/browser"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLinuxBookmarkRootDirs covers XDG defaults, overrides and sandbox paths.
func TestLinuxBookmarkRootDirs(t *testing.T) {
	for _, configHome := range []string{"", t.TempDir(), "relative-config"} {
		t.Run(configHome, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", configHome)
			expectedConfig := configHome
			if !filepath.IsAbs(expectedConfig) {
				expectedConfig = "~/.config"
			}
			assert.Contains(t, getLinuxBookmarkRootDirs(browser.BrowserIDChrome), filepath.Join(expectedConfig, "google-chrome"))
			assert.Contains(t, getLinuxBookmarkRootDirs(browser.BrowserIDChrome), "~/.var/app/com.google.Chrome/config/google-chrome")
			firefoxRoots := getLinuxBookmarkRootDirs(browser.BrowserIDFirefox)
			for _, root := range []string{
				filepath.Join(expectedConfig, "mozilla", "firefox"),
				"~/.mozilla/firefox",
				"~/snap/firefox/common/.mozilla/firefox",
				"~/snap/firefox/common/.config/mozilla/firefox",
				"~/.var/app/org.mozilla.firefox/.mozilla/firefox",
				"~/.var/app/org.mozilla.firefox/config/mozilla/firefox",
			} {
				assert.Contains(t, firefoxRoots, root)
			}
		})
	}
}

// TestLinuxBookmarksWithoutBrowserCommands exercises discovery and loading with no host executables.
func TestLinuxBookmarksWithoutBrowserCommands(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux browser discovery")
	}
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("PATH", t.TempDir())
	chromeProfile := filepath.Join(root, "google-chrome", "Default")
	require.NoError(t, os.MkdirAll(chromeProfile, 0o700))
	writeChromiumBookmarkJSON(t, filepath.Join(chromeProfile, "AccountBookmarks"), chromiumBookmarkEntry{Name: "Chrome Fixture", URL: "https://chrome.example"})
	firefoxRoot := filepath.Join(root, "mozilla", "firefox")
	firefoxProfile := filepath.Join(firefoxRoot, "release")
	require.NoError(t, os.MkdirAll(firefoxProfile, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(firefoxRoot, "profiles.ini"), []byte("[Profile0]\nIsRelative=1\nPath=release\n"), 0o600))
	db := createFirefoxPlacesForTest(t, filepath.Join(firefoxProfile, "places.sqlite"), false)
	require.NoError(t, db.Close())
	p := &BrowserBookmarkPlugin{api: &mockAPI{}}
	selected := p.getSelectedBookmarkBrowsers(context.Background())
	assert.Contains(t, selected, browser.BrowserIDChrome)
	assert.Contains(t, selected, browser.BrowserIDFirefox)
	// Read only fixtures in the regular suite; live profile loading is opt-in.
	bookmarks := p.loadChromiumBookmarkFiles(context.Background(), chromeProfile, "/", "Chrome")
	bookmarks = append(bookmarks, p.loadFirefoxBookmarksFromRootDirs(context.Background(), []string{firefoxRoot}, "Firefox")...)
	names := bookmarkNames(bookmarks)
	assert.Contains(t, names, "Chrome Fixture")
	assert.Contains(t, names, "Fixture Bookmark")
}

// TestBrowserBookmarkPlugin_LoadChromeBookmarksFromLocalProfile verifies real discovery and loading.
func TestBrowserBookmarkPlugin_LoadChromeBookmarksFromLocalProfile(t *testing.T) {
	if os.Getenv("WOX_LIVE_BROWSER_BOOKMARKS") != "1" {
		t.Skip("set WOX_LIVE_BROWSER_BOOKMARKS=1 to read local browser data")
	}
	p := &BrowserBookmarkPlugin{api: &mockAPI{}}
	if !assert.Contains(t, p.getSelectedBookmarkBrowsers(context.Background()), browser.BrowserIDChrome) {
		return
	}
	bookmarks := browserBookmarkLoaders[browser.BrowserIDChrome](p, context.Background())
	t.Logf("Loaded %d Chrome bookmarks", len(bookmarks))
	assert.NotEmpty(t, bookmarks)
}
