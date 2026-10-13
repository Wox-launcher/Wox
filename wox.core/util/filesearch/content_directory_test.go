package filesearch

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"wox/util/filesearch/column"
)

// TestSharedContentDirectoryPrefersNativeOwnership exercises keyset pages and
// reconciliation when settings root IDs differ from native volume identities.
func TestSharedContentDirectoryPrefersNativeOwnership(t *testing.T) {
	ctx := context.Background()
	configured := newDirectoryIndex(filepath.Join(t.TempDir(), "configured"))
	native := newDirectoryIndex(filepath.Join(t.TempDir(), "native"))
	defer configured.close()
	defer native.close()
	root := RootRecord{ID: "same-id", Path: t.TempDir()}
	other := RootRecord{ID: "outside", Path: t.TempDir()}
	for _, directory := range []*directoryIndex{configured, native} {
		if err := directory.attach(root); err != nil {
			t.Fatal(err)
		}
	}
	if err := configured.attach(other); err != nil {
		t.Fatal(err)
	}
	fixture := func(directory *directoryIndex, root RootRecord, names ...string) {
		t.Helper()
		builder := column.NewBuilder(root.Path, 5)
		for id, name := range names {
			if err := builder.Add(column.Record{ID: uint64(10 + id), ParentID: 5, Name: name}); err != nil {
				t.Fatal(err)
			}
		}
		base, err := builder.Finish()
		if err != nil {
			t.Fatal(err)
		}
		if err := directory.shards[root.ID].store.Publish(base, "fixture", "test", 0); err != nil {
			t.Fatal(err)
		}
	}
	fixture(configured, root, "same.txt", "stale-configured.txt")
	fixture(configured, other, "external.txt")
	view := sharedContentDirectory{configured: configured, native: native, nativeRoots: []string{root.Path}}
	if _, err := view.ListContentIndexCandidates(ctx, ContentCandidateCursor{}, map[string]bool{"txt": true}, 1); !errors.Is(err, column.ErrNotReady) {
		t.Fatalf("unready native generation looked empty: %v", err)
	}
	fixture(native, root, "same.txt", "native-only.txt")
	seen := make(map[string]bool)
	var cursor ContentCandidateCursor
	for page := 0; page < 5; page++ {
		candidates, err := view.ListContentIndexCandidates(ctx, cursor, map[string]bool{"txt": true}, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(candidates) == 0 {
			break
		}
		candidate := candidates[0]
		if seen[candidate.Path] {
			t.Fatalf("duplicate candidate %s", candidate.Path)
		}
		seen[candidate.Path] = true
		cursor = candidate.Cursor
	}
	if len(seen) != 3 || seen[filepath.Join(root.Path, "stale-configured.txt")] {
		t.Fatalf("merged paths=%v", seen)
	}
	paths, err := view.ListEntryPathsUnderScope(ctx, "content-root-17", root.Path)
	if err != nil || len(paths) != 2 {
		t.Fatalf("scope paths=%v error=%v", paths, err)
	}
	if err := native.close(); err != nil {
		t.Fatal(err)
	}
	if _, err := view.ListEntryPathsUnderScope(ctx, "", root.Path); err == nil {
		t.Fatal("closed source looked empty")
	}
}
