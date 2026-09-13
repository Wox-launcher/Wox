package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"wox/common"
	"wox/util/filesearch"
)

// A shortcut whose target disappeared must keep reusing its cache entry; otherwise the
// fallback reconciliation reparses it every cycle and rebuilds the whole query index.
func TestReuseAppFromCacheKeepsEntryWhileIconSourceStaysMissing(t *testing.T) {
	appPath := filepath.Join(t.TempDir(), "Cursor.lnk")
	if err := os.WriteFile(appPath, []byte("shortcut"), 0o644); err != nil {
		t.Fatal(err)
	}
	fileInfo, err := os.Stat(appPath)
	if err != nil {
		t.Fatal(err)
	}
	plugin := &ApplicationPlugin{retriever: appRetriever, api: emptyAPIImpl{}}
	missingSource := filepath.Join(t.TempDir(), "Cursor.exe")
	cached := appInfo{
		Path:             appPath,
		Identity:         "cursor",
		LastModifiedUnix: plugin.getAppModifiedUnix(appPath, fileInfo),
		IconSourcePath:   missingSource,
	}
	cache := map[string]appInfo{plugin.pathCacheKey(appPath): cached}

	if _, reused := plugin.reuseAppFromCache(context.Background(), appPath, fileInfo, cache); !reused {
		t.Fatal("expected cache reuse while the icon source is still missing")
	}

	if err := os.WriteFile(missingSource, []byte("exe"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, reused := plugin.reuseAppFromCache(context.Background(), appPath, fileInfo, cache); reused {
		t.Fatal("expected reparse once the icon source reappears")
	}

	sourceInfo, err := os.Stat(missingSource)
	if err != nil {
		t.Fatal(err)
	}
	cached.IconSourceModifiedUnix = sourceInfo.ModTime().UnixNano()
	cache[plugin.pathCacheKey(appPath)] = cached
	if err := os.Remove(missingSource); err != nil {
		t.Fatal(err)
	}
	if _, reused := plugin.reuseAppFromCache(context.Background(), appPath, fileInfo, cache); reused {
		t.Fatal("expected reparse when a previously present icon source disappears")
	}
}

func TestAppInfoEqualsIgnoresRuntimeOnlyState(t *testing.T) {
	base := appInfo{
		Name:            "Cursor",
		SearchableNames: []string{"cursor", "editor"},
		Path:            `C:\apps\Cursor.lnk`,
		Icon:            common.WoxImage{ImageType: common.WoxImageTypeAbsolutePath, ImageData: `C:\cache\cursor.png`},
		IconSourcePath:  `C:\apps\Cursor.exe`,
		Pid:             10,
	}
	same := base
	same.Pid = 99
	if !base.equals(same) {
		t.Fatal("expected entries differing only by pid to be equal")
	}
	changedIcon := base
	changedIcon.Icon.ImageData = `C:\cache\cursor-2.png`
	if base.equals(changedIcon) {
		t.Fatal("expected icon change to be detected")
	}
	changedNames := base
	changedNames.SearchableNames = []string{"cursor"}
	if base.equals(changedNames) {
		t.Fatal("expected searchable name change to be detected")
	}
}

func TestAppPathInDirectoryScopeMatchesRecursiveDepth(t *testing.T) {
	rootPath := t.TempDir()
	plugin := &ApplicationPlugin{}
	directory := appDirectory{Path: rootPath, Recursive: true, RecursiveDepth: 1}

	if !plugin.isAppPathInDirectoryScope(filepath.Join(rootPath, "Obsidian.lnk"), directory) {
		t.Fatal("expected root app to be in scope")
	}
	if !plugin.isAppPathInDirectoryScope(filepath.Join(rootPath, "Scoop Apps", "Obsidian.lnk"), directory) {
		t.Fatal("expected child app to be in scope")
	}
	if plugin.isAppPathInDirectoryScope(filepath.Join(rootPath, "Scoop Apps", "Nested", "Obsidian.lnk"), directory) {
		t.Fatal("did not expect app beyond recursive depth")
	}
}

func TestFallbackReconcileDirectoriesSelectsOnlyTrackedFallbackRoots(t *testing.T) {
	trackedPath := t.TempDir()
	untrackedPath := t.TempDir()
	plugin := &ApplicationPlugin{retriever: appRetriever}
	directories := []appDirectory{
		{Path: trackedPath, trackChanges: true},
		{Path: untrackedPath},
	}
	roots := []filesearch.RootRecord{{ID: "tracked", Path: trackedPath}}
	feed := filesearch.NewFallbackChangeFeed()
	defer feed.Close()

	selected := plugin.getFallbackReconcileDirectories(context.Background(), feed, roots, directories)
	if len(selected) != 1 || selected[0].Path != trackedPath {
		t.Fatalf("expected tracked fallback root, got %#v", selected)
	}
}
