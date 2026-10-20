package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
	"wox/common"
	"wox/util"
	"wox/util/filesearch"
)

// Loss markers may contain the last dropped file path, but recovery must cover
// the subscribed root to repair every event discarded by the bounded queue.
func TestAppChangeHistoryLossRepairsSubscribedRoot(t *testing.T) {
	plugin := &ApplicationPlugin{api: emptyAPIImpl{}}
	root := t.TempDir()
	change, ok := plugin.getActionableAppChange(filesearch.ChangeSignal{
		Kind: filesearch.ChangeSignalKindHistoryLost, RootID: "apps", Path: filepath.Join(root, "last.lnk"),
	}, []string{".lnk"}, map[string]string{"apps": root})
	if !ok || change.Path != root || !change.PathIsDir || change.SemanticKind != filesearch.ChangeSemanticKindModify {
		t.Fatalf("missing root repair: %+v accepted=%t", change, ok)
	}
}

type skipParseRetriever struct {
	Retriever
	calls atomic.Int32
}

// ParseAppInfo reports a permanent indexing skip and counts how often reconciliation asks.
func (r *skipParseRetriever) ParseAppInfo(context.Context, string) (appInfo, error) {
	r.calls.Add(1)
	return appInfo{}, fmt.Errorf("%w: launcher is hidden", errSkipAppIndexing)
}

// TestUpsertSkipsPermanentParseFailureWithoutRetrySleep keeps hidden launchers from
// blocking the change feed. Each retry used to sleep for six seconds.
func TestUpsertSkipsPermanentParseFailureWithoutRetrySleep(t *testing.T) {
	appPath := filepath.Join(t.TempDir(), "hidden.desktop")
	if err := os.WriteFile(appPath, []byte("[Desktop Entry]\nNoDisplay=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	retriever := &skipParseRetriever{Retriever: appRetriever}
	plugin := &ApplicationPlugin{
		api:            emptyAPIImpl{},
		retriever:      retriever,
		unparsableApps: util.NewHashMap[string, int64](),
	}

	started := time.Now()
	if plugin.upsertIndexedAppByPath(context.Background(), appPath) {
		t.Fatal("hidden launcher was indexed")
	}
	if elapsed := time.Since(started); elapsed >= 500*time.Millisecond {
		t.Fatalf("permanent skip slept for %s", elapsed)
	}
	if calls := retriever.calls.Load(); calls != 1 {
		t.Fatalf("parse calls = %d, want 1", calls)
	}

	if plugin.upsertIndexedAppByPath(context.Background(), appPath) {
		t.Fatal("unchanged hidden launcher was indexed on the next reconcile")
	}
	if calls := retriever.calls.Load(); calls != 1 {
		t.Fatalf("parse calls after remembering the skip = %d, want 1", calls)
	}
}

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
	changedClassification := base
	changedClassification.HasDedicatedIcon = true
	if base.equals(changedClassification) {
		t.Fatal("expected native icon classification change to be detected")
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
