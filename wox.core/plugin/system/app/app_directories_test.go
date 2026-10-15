package app

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

type directoryScanRetriever struct {
	Retriever
}

func (r directoryScanRetriever) GetAppExtensions(context.Context) []string {
	return []string{"exe", "lnk"}
}

// TestAppDirectoryScanPreservesScope covers depth, exclusions, ignore rules and
// app directories whose extension is itself a supported entry point.
func TestAppDirectoryScanPreservesScope(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"main.EXE", "notes.txt", "child/child.lnk", "child/deep/deep.exe", "excluded/skip.exe", "ignored/skip.exe"} {
		fullPath := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(root, "bundle.exe"), 0o755); err != nil {
		t.Fatal(err)
	}
	api := newRecordingAppAPI()
	api.settings[ignoreRulesSettingKey] = `[{"Pattern":"*ignored*","IncludeFuture":true}]`
	app := &ApplicationPlugin{api: api, retriever: directoryScanRetriever{appRetriever}}
	app.rebuildIgnoreRuleMatchers(context.Background())
	for _, test := range []struct {
		name      string
		recursive bool
		depth     int
		want      []string
	}{
		{"flat", false, 3, []string{"bundle.exe", "main.EXE"}},
		{"zero depth", true, 0, []string{"bundle.exe", "main.EXE"}},
		{"one level", true, 1, []string{"bundle.exe", "child/child.lnk", "main.EXE"}},
		{"two levels", true, 2, []string{"bundle.exe", "child/child.lnk", "child/deep/deep.exe", "main.EXE"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := app.getAppPaths(context.Background(), []appDirectory{{Path: root, Recursive: test.recursive, RecursiveDepth: test.depth, RecursiveExcludes: []string{"excluded"}}})
			want := make([]string, len(test.want))
			for i, path := range test.want {
				want[i] = filepath.Join(root, filepath.FromSlash(path))
			}
			slices.Sort(got)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Fatalf("paths = %v, want %v", got, want)
			}
		})
	}
}

// TestAppDirectoryScanFollowsDirectoryLinks preserves the previous Stat-based traversal.
func TestAppDirectoryScanFollowsDirectoryLinks(t *testing.T) {
	root, target := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "linked.exe"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "linked")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}
	app := &ApplicationPlugin{api: emptyAPIImpl{}, retriever: directoryScanRetriever{appRetriever}}
	got := app.getAppPaths(context.Background(), []appDirectory{{Path: root, Recursive: true, RecursiveDepth: 1}})
	want := filepath.Join(link, "linked.exe")
	if len(got) != 1 || got[0] != want {
		t.Fatalf("paths = %v, want %s", got, want)
	}
}

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
