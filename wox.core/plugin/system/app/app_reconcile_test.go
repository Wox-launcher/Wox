package app

import (
	"context"
	"path/filepath"
	"testing"
	"wox/util/filesearch"
)

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
