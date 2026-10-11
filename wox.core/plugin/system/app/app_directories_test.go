package app

import (
	"path/filepath"
	"testing"
)

// TestMergeUserDirectoryApps covers removal ownership and overlapping discovery sources.
func TestMergeUserDirectoryApps(t *testing.T) {
	root := t.TempDir()
	oldRoot := appDirectory{Path: filepath.Join(root, "old"), Recursive: true, RecursiveDepth: 3}
	newRoot := appDirectory{Path: filepath.Join(root, "new"), Recursive: true, RecursiveDepth: 3}
	defaultRoot := appDirectory{Path: filepath.Join(oldRoot.Path, "default")}
	removed := appInfo{Name: "removed", Path: filepath.Join(oldRoot.Path, "removed.exe")}
	shared := appInfo{Name: "shared", Path: filepath.Join(defaultRoot.Path, "shared.exe")}
	extra := appInfo{Name: "extra", Path: filepath.Join(oldRoot.Path, "extra.exe")}
	unrelated := appInfo{Name: "unrelated", Path: filepath.Join(root, "other.exe")}
	stale := appInfo{Name: "stale", Path: filepath.Join(newRoot.Path, "stale.exe")}
	previous := appInfo{Name: "previous", Path: filepath.Join(newRoot.Path, "updated.exe")}
	updated := appInfo{Name: "updated", Path: previous.Path}
	added := appInfo{Name: "added", Path: filepath.Join(newRoot.Path, "added.exe")}
	a := &ApplicationPlugin{
		retriever:              appRetriever,
		apps:                   []appInfo{removed, shared, extra, unrelated, stale, previous},
		indexedUserDirectories: []appDirectory{oldRoot},
	}
	a.extraAppPathKeys = map[string]bool{a.pathCacheKey(extra.Path): true}
	merged := a.mergeUserDirectoryApps([]appInfo{updated, added}, []appDirectory{newRoot}, []appDirectory{defaultRoot})
	want := map[string]bool{"shared": true, "extra": true, "unrelated": true, "updated": true, "added": true}
	if len(merged) != len(want) {
		t.Fatalf("merged apps = %+v, want %d apps", merged, len(want))
	}
	for _, app := range merged {
		if !want[app.Name] {
			t.Fatalf("unexpected or duplicate app: %+v", app)
		}
		delete(want, app.Name)
	}
}

// TestMergeUserDirectoryAppsKeepsRemainingNestedRoot verifies parent removal preserves a remaining custom root.
func TestMergeUserDirectoryAppsKeepsRemainingNestedRoot(t *testing.T) {
	root := appDirectory{Path: t.TempDir(), Recursive: true, RecursiveDepth: 3}
	nested := appDirectory{Path: filepath.Join(root.Path, "nested"), Recursive: true, RecursiveDepth: 3}
	removed := appInfo{Name: "removed", Path: filepath.Join(root.Path, "removed.exe")}
	kept := appInfo{Name: "kept", Path: filepath.Join(nested.Path, "kept.exe")}
	a := &ApplicationPlugin{retriever: appRetriever, apps: []appInfo{removed, kept}, indexedUserDirectories: []appDirectory{root, nested}}
	merged := a.mergeUserDirectoryApps([]appInfo{kept}, []appDirectory{nested}, nil)
	if len(merged) != 1 || merged[0].Path != kept.Path {
		t.Fatalf("merged apps = %+v, want only the app in the remaining nested root", merged)
	}
}
