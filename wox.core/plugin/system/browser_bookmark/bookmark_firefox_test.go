package browserbookmark

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"wox/plugin"
	"wox/plugin/plugintest"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrowserBookmarkPlugin_LoadFirefoxBookmarksFromLocalProfile(t *testing.T) {
	if os.Getenv("WOX_LIVE_BROWSER_BOOKMARKS") != "1" {
		t.Skip("set WOX_LIVE_BROWSER_BOOKMARKS=1 to read local browser data")
	}
	plugin := &BrowserBookmarkPlugin{api: &mockAPI{}}
	rootDirs := getFirefoxBookmarkRootDirs()
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
	assert.Greater(t, len(bookmarks), 0, "expected at least one local Firefox bookmark")
}

// TestFirefoxBookmarksFromProfilesIni covers relative, external and fallback profiles.
func TestFirefoxBookmarksFromProfilesIni(t *testing.T) {
	root := t.TempDir()
	external := t.TempDir()
	for _, profile := range []string{filepath.Join(root, "release"), filepath.Join(root, "Profiles", "dev"), external} {
		require.NoError(t, os.MkdirAll(profile, 0o700))
		db := createFirefoxPlacesForTest(t, filepath.Join(profile, "places.sqlite"), false)
		require.NoError(t, db.Close())
	}
	ini := "[Profile0]\nIsRelative=1\nPath=release\n[Profile1]\nIsRelative=0\nPath=" + external + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "profiles.ini"), []byte(ini), 0o600))
	p := &BrowserBookmarkPlugin{api: &mockAPI{}}
	assert.ElementsMatch(t, []string{filepath.Join(root, "release"), filepath.Join(root, "Profiles", "dev"), external},
		p.resolveFirefoxProfileDirs(context.Background(), []string{root, root}, "Firefox"))
	bookmarks := p.loadFirefoxBookmarksFromRootDirs(context.Background(), []string{root}, "Firefox")
	assert.Equal(t, []string{"Fixture Bookmark", "Fixture Bookmark", "Fixture Bookmark"}, bookmarkNames(bookmarks))
}

// TestFirefoxBookmarksFromLockedWAL ensures committed WAL data is read without changing the source.
func TestFirefoxBookmarksFromLockedWAL(t *testing.T) {
	places := filepath.Join(t.TempDir(), "places.sqlite")
	db := createFirefoxPlacesForTest(t, places, true)
	defer db.Close()
	before := make(map[string][32]byte)
	for _, suffix := range []string{"", "-wal"} {
		data, err := os.ReadFile(places + suffix)
		require.NoError(t, err)
		before[suffix] = sha256.Sum256(data)
	}

	reader, err := sql.Open("sqlite3", places+"?mode=ro&_busy_timeout=1")
	require.NoError(t, err)
	_, err = reader.Query("SELECT title FROM moz_bookmarks")
	require.ErrorContains(t, err, "database is locked")
	require.NoError(t, reader.Close())

	p := &BrowserBookmarkPlugin{api: &mockAPI{}}
	bookmarks := p.loadFirefoxBookmarkFromPlacesFile(context.Background(), places)
	require.Len(t, bookmarks, 1)
	assert.Equal(t, "Fixture Bookmark", bookmarks[0].Name)
	assert.Equal(t, "https://example.com/bookmark", bookmarks[0].Url)
	for suffix, digest := range before {
		data, err := os.ReadFile(places + suffix)
		require.NoError(t, err)
		assert.Equal(t, digest, sha256.Sum256(data), "source file changed: %s", suffix)
	}
}

// createFirefoxPlacesForTest leaves the bookmark in WAL when exclusive is requested.
func createFirefoxPlacesForTest(t *testing.T, places string, exclusive bool) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", places)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if exclusive {
		_, err = db.Exec("PRAGMA locking_mode=EXCLUSIVE; PRAGMA journal_mode=WAL; PRAGMA wal_autocheckpoint=0;")
		require.NoError(t, err)
	}
	_, err = db.Exec(`CREATE TABLE moz_places (id INTEGER PRIMARY KEY, url TEXT);
		CREATE TABLE moz_bookmarks (title TEXT, fk INTEGER, type INTEGER);`)
	require.NoError(t, err)
	if exclusive {
		_, err = db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
		require.NoError(t, err)
	}
	_, err = db.Exec(`INSERT INTO moz_places VALUES (1, 'https://example.com/bookmark');
		INSERT INTO moz_bookmarks VALUES ('Fixture Bookmark', 1, 1);`)
	require.NoError(t, err)
	return db
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
