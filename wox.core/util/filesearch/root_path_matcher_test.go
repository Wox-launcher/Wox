package filesearch

import (
	"path/filepath"
	"testing"
)

func TestRootPathMatcherFindsExactAndChildPaths(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "root")
	root := RootRecord{ID: "root-main", Path: rootPath}
	matcher := newRootPathMatcher([]RootRecord{root})

	for _, path := range []string{
		rootPath,
		filepath.Join(rootPath, "child.txt"),
		filepath.Join(rootPath, "nested", "child.txt"),
		rootPath + string(filepath.Separator),
	} {
		cleanPath := filepath.Clean(path)
		matched, ok := matcher.findClean(cleanPath)
		if !ok || matched.ID != root.ID {
			t.Fatalf("expected %q to match %q, got %#v ok=%t", path, root.ID, matched, ok)
		}
	}
}

func TestRootPathMatcherRejectsSiblingPrefix(t *testing.T) {
	base := t.TempDir()
	root := RootRecord{ID: "root-bar", Path: filepath.Join(base, "bar")}
	matcher := newRootPathMatcher([]RootRecord{root})

	sibling := filepath.Join(base, "barista", "child.txt")
	if matched, ok := matcher.findClean(filepath.Clean(sibling)); ok {
		t.Fatalf("expected sibling prefix not to match, got %#v", matched)
	}
}

func TestRootPathMatcherChoosesLongestRoot(t *testing.T) {
	base := t.TempDir()
	parent := RootRecord{ID: "root-parent", Path: filepath.Join(base, "workspace")}
	dynamic := RootRecord{ID: "root-dynamic", Path: filepath.Join(base, "workspace", "src"), Kind: RootKindDynamic, DynamicParentRootID: parent.ID}
	matcher := newRootPathMatcher([]RootRecord{parent, dynamic})

	matched, ok := matcher.findClean(filepath.Clean(filepath.Join(dynamic.Path, "main.go")))
	if !ok || matched.ID != dynamic.ID {
		t.Fatalf("expected longest dynamic root, got %#v ok=%t", matched, ok)
	}
}

func TestCleanPathsOverlapUsesPathBoundaries(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")

	if !cleanPathsOverlap(root, filepath.Join(root, "nested", "file.txt")) {
		t.Fatalf("expected child path to overlap root")
	}
	if cleanPathsOverlap(root, filepath.Join(base, "root-sibling", "file.txt")) {
		t.Fatalf("expected sibling prefix not to overlap root")
	}
}

func BenchmarkRootPathMatcherFind(b *testing.B) {
	base := filepath.Join("/tmp", "wox-root-matcher-bench")
	roots := make([]RootRecord, 0, 17)
	roots = append(roots, RootRecord{ID: "root-user", Path: filepath.Join(base, "user")})
	for index := 0; index < 16; index++ {
		roots = append(roots, RootRecord{
			ID:                  "root-dynamic",
			Path:                filepath.Join(base, "user", "project", "module", "dynamic", string(rune('a'+index))),
			Kind:                RootKindDynamic,
			DynamicParentRootID: "root-user",
		})
	}
	matcher := newRootPathMatcher(roots)
	paths := []string{
		filepath.Clean(filepath.Join(base, "user", "project", "module", "dynamic", "a", "main.go")),
		filepath.Clean(filepath.Join(base, "user", "project", "module", "dynamic", "h", "cache.db")),
		filepath.Clean(filepath.Join(base, "user", "Downloads", "note.txt")),
		filepath.Clean(filepath.Join(base, "outside", "event.txt")),
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = matcher.findClean(paths[i%len(paths)])
	}
}
