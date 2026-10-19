package app

import (
	"context"
	"path/filepath"
	"testing"

	"wox/util/filesearch"
)

// TestAppChangeScopesKeepOverlappingDepthAndExclusions checks the scan after a
// directory event, not just which events are admitted by the subscription.
func TestAppChangeScopesKeepOverlappingDepthAndExclusions(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	app := &ApplicationPlugin{api: emptyAPIImpl{}, retriever: appRetriever}
	directories := []appDirectory{
		{Path: root, Recursive: true, RecursiveDepth: 2, RecursiveExcludes: []string{"child/excluded"}, trackChanges: true},
		{Path: child, Recursive: true, RecursiveDepth: 3, trackChanges: true},
	}
	scopes := app.getLocalAppDirectoriesForChange(child, directories)
	if len(scopes) != 2 || scopes[0].RecursiveDepth != 1 || scopes[1].RecursiveDepth != 3 {
		t.Fatalf("wrong overlap scopes: %+v", scopes)
	}
	if app.isAppPathInDirectoryScope(filepath.Join(child, "excluded", "app.desktop"), scopes[0]) {
		t.Fatal("original exclusion lost")
	}
	if !app.isAppPathInDirectoryScope(filepath.Join(child, "deep", "app.desktop"), scopes[1]) {
		t.Fatal("explicit deeper scope lost")
	}
}

// TestAppWatchScopeMatchesScanDepth verifies depth, exclusions and overlapping
// root policies without relying on the host's native filesystem notification API.
func TestAppWatchScopeMatchesScanDepth(t *testing.T) {
	root := t.TempDir()
	app := &ApplicationPlugin{api: emptyAPIImpl{}, retriever: appRetriever}
	directories := []appDirectory{{Path: root, Recursive: true, RecursiveDepth: 1, RecursiveExcludes: []string{"excluded"}, trackChanges: true}}
	for _, test := range []struct {
		path string
		want bool
	}{
		{".", true}, {"child", true}, {"child/deep", false}, {"excluded", false}, {"excluded-sibling", true}, {"Example.app", false}, {"Example.app/Contents", false},
	} {
		path := filepath.Join(root, filepath.FromSlash(test.path))
		if got := app.appWatchDirectoryAllowed(path, directories, []string{"app", "desktop"}); got != test.want {
			t.Errorf("%s: got %t want %t", test.path, got, test.want)
		}
	}
	directories = append(directories, appDirectory{Path: filepath.Join(root, "child"), Recursive: true, RecursiveDepth: 2, trackChanges: true})
	if !app.appWatchDirectoryAllowed(filepath.Join(root, "child", "deep"), directories, []string{"desktop"}) {
		t.Fatal("nested explicit root was shadowed")
	}
}

// TestAppBundleEventsRefreshTheBundle keeps resource deletion distinct from
// uninstall and suppresses application events outside the configured scan scope.
func TestAppBundleEventsRefreshTheBundle(t *testing.T) {
	root := t.TempDir()
	app := &ApplicationPlugin{api: emptyAPIImpl{}, retriever: appRetriever}
	directories := []appDirectory{{Path: root, Recursive: true, RecursiveDepth: 1, trackChanges: true}}
	bundle := filepath.Join(root, "Example.app")
	for _, path := range []string{bundle, filepath.Join(bundle, "Contents", "Info.plist")} {
		signal := filesearch.ChangeSignal{Kind: filesearch.ChangeSignalKindDirtyPath, Path: path, PathIsDir: path == bundle, PathTypeKnown: true, SemanticKind: filesearch.ChangeSemanticKindRemove}
		got, ok := app.scopedAppSignal(signal, directories, []string{"app"})
		if !ok || got.Path != bundle || got.PathIsDir {
			t.Fatalf("not a bundle update: %+v accepted=%t", got, ok)
		}
		want := filesearch.ChangeSemanticKindRemove
		if path != bundle {
			want = filesearch.ChangeSemanticKindModify
		}
		if got.SemanticKind != want {
			t.Fatalf("wrong bundle operation: %+v", got)
		}
	}
	signal := filesearch.ChangeSignal{Kind: filesearch.ChangeSignalKindDirtyPath, Path: filepath.Join(root, "child", "deep", "TooDeep.app"), PathIsDir: true}
	if _, ok := app.scopedAppSignal(signal, directories, []string{"app"}); ok {
		t.Fatal("accepted app beyond scan depth")
	}
}

// TestAppFSEventsScopedLossIsActionable checks the adapter for the native
// must-scan marker, whose operation is deliberately unknown to the file index.
func TestAppFSEventsScopedLossIsActionable(t *testing.T) {
	root := t.TempDir()
	app := &ApplicationPlugin{api: emptyAPIImpl{}, retriever: appRetriever}
	dirs := []appDirectory{{Path: root, Recursive: true, RecursiveDepth: 2, trackChanges: true}}
	signal, ok := app.scopedAppSignal(filesearch.ChangeSignal{Kind: filesearch.ChangeSignalKindDirtyPath, FeedType: filesearch.RootFeedTypeFSEvents, Path: filepath.Join(root, "child"), PathIsDir: true, PathTypeKnown: true, SemanticKind: filesearch.ChangeSemanticKindUnknown}, dirs, []string{"app"})
	if !ok {
		t.Fatal("scoped recovery was filtered out")
	}
	change, ok := app.getActionableAppChange(signal, []string{"app"}, nil)
	if !ok || !change.PathIsDir || change.SemanticKind != filesearch.ChangeSemanticKindModify {
		t.Fatalf("recovery not actionable: %+v", change)
	}
}

// TestAppWatchScopeHonorsIgnoredAncestors rejects descendants even when only the
// parent directory name, rather than each descendant name, matches an ignore rule.
func TestAppWatchScopeHonorsIgnoredAncestors(t *testing.T) {
	root := t.TempDir()
	api := newRecordingAppAPI()
	api.settings[ignoreRulesSettingKey] = `[{"Pattern":"*ignored*","IncludeFuture":true}]`
	app := &ApplicationPlugin{api: api, retriever: appRetriever}
	app.rebuildIgnoreRuleMatchers(context.Background())
	dirs := []appDirectory{{Path: root, Recursive: true, RecursiveDepth: 4, trackChanges: true}}
	if app.appWatchDirectoryAllowed(filepath.Join(root, "ignored", "nested"), dirs, []string{"desktop"}) {
		t.Fatal("ignored subtree was watched")
	}
}
