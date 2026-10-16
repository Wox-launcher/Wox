package filesearch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
	"wox/util/filesearch/column"
)

// TestDirectoryIndexScopePublication exercises scan ownership and interrupted
// generations against the public live view rather than storage implementation.
func TestDirectoryIndexScopePublication(t *testing.T) {
	ctx := context.Background()
	index := newDirectoryIndex(filepath.Join(t.TempDir(), "index"))
	defer index.close()
	root := RootRecord{ID: "root", Path: filepath.Join(t.TempDir(), "root")}
	if err := index.attach(root); err != nil {
		t.Fatal(err)
	}
	entry := func(relative string, directory bool) EntryRecord {
		path := filepath.Join(root.Path, relative)
		return EntryRecord{RootID: root.ID, Path: path, ParentPath: filepath.Dir(path), Name: filepath.Base(path), IsDir: directory}
	}
	batch := func(scope string, entries ...EntryRecord) SubtreeSnapshotBatch {
		return SubtreeSnapshotBatch{RootID: root.ID, ScopePath: scope, Entries: entries}
	}
	write := func(scope string, direct bool, b SubtreeSnapshotBatch) {
		t.Helper()
		if err := index.writeScope(ctx, root.ID, scope, direct, func(yield func(SubtreeSnapshotBatch) error) error { return yield(b) }); err != nil {
			t.Fatal(err)
		}
	}
	assertPaths := func(want ...string) {
		t.Helper()
		found := map[string]bool{}
		if err := index.visit(ctx, root.ID, func(e EntryRecord) error {
			relative, _ := filepath.Rel(root.Path, e.Path)
			found[relative] = true
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if len(found) != len(want) {
			t.Fatalf("paths=%v want=%v", found, want)
		}
		for _, path := range want {
			if !found[filepath.FromSlash(path)] {
				t.Fatalf("missing %q: %v", path, found)
			}
		}
	}
	write(root.Path, false, batch(root.Path, entry("a", true), entry("a/old.txt", false), entry("b", true), entry("b/keep.txt", false), entry("top.txt", false)))
	write(root.Path, true, batch(root.Path, entry("new-top.txt", false)))
	assertPaths("a", "a/old.txt", "b", "b/keep.txt", "new-top.txt")
	scope := filepath.Join(root.Path, "a")
	write(scope, false, batch(scope, entry("a", true), entry("a/new.txt", false)))
	assertPaths("a", "a/new.txt", "b", "b/keep.txt", "new-top.txt")
	interrupted := errors.New("interrupted scan")
	if err := index.writeScope(ctx, root.ID, scope, false, func(yield func(SubtreeSnapshotBatch) error) error {
		if err := yield(batch(scope, entry("a/partial.txt", false))); err != nil {
			return err
		}
		return interrupted
	}); !errors.Is(err, interrupted) {
		t.Fatal(err)
	}
	assertPaths("a", "a/new.txt", "b", "b/keep.txt", "new-top.txt")
	if err := index.beginBuild(root.ID); err != nil {
		t.Fatal(err)
	}
	write(root.Path, false, batch(root.Path, entry("replacement.txt", false)))
	assertPaths("a", "a/new.txt", "b", "b/keep.txt", "new-top.txt")
	index.abortBuilds()
	assertPaths("a", "a/new.txt", "b", "b/keep.txt", "new-top.txt")
	if err := index.beginBuild(root.ID); err != nil {
		t.Fatal(err)
	}
	write(root.Path, false, batch(root.Path, entry("complete.txt", false)))
	if err := index.finishBuild(ctx, root); err != nil {
		t.Fatal(err)
	}
	assertPaths("complete.txt")
}

// TestDirectoryIndexDeletedParentDoesNotResurrectChildren covers path-ID reuse
// after a deleted directory has passed through another scope publication.
func TestDirectoryIndexDeletedParentDoesNotResurrectChildren(t *testing.T) {
	ctx := context.Background()
	index := newDirectoryIndex(filepath.Join(t.TempDir(), "index"))
	defer index.close()
	root := RootRecord{ID: "root", Path: t.TempDir()}
	if err := index.attach(root); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root.Path, "deleted")
	entries := []EntryRecord{{RootID: root.ID, Path: dir, ParentPath: root.Path, IsDir: true}, {RootID: root.ID, Path: filepath.Join(dir, "stale.txt"), ParentPath: dir}}
	if err := index.replaceRoot(ctx, root, func(yield func(SubtreeSnapshotBatch) error) error {
		return yield(SubtreeSnapshotBatch{RootID: root.ID, ScopePath: root.Path, Entries: entries})
	}); err != nil {
		t.Fatal(err)
	}
	if err := index.applyEntries(ctx, root.ID, nil, []string{dir}, ""); err != nil {
		t.Fatal(err)
	}
	if err := index.writeScope(ctx, root.ID, root.Path, true, func(yield func(SubtreeSnapshotBatch) error) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := index.applyEntries(ctx, root.ID, entries[:1], nil, ""); err != nil {
		t.Fatal(err)
	}
	results, err := index.search(ctx, SearchQuery{Raw: "stale"}, 10)
	if err != nil || len(results) != 0 {
		t.Fatalf("resurrected: %+v %v", results, err)
	}
}

// TestDirectoryIndexUnreadableScopePreservesFacts covers temporary access loss
// during incremental and full scans, followed by a confirmed empty directory.
func TestDirectoryIndexUnreadableScopePreservesFacts(t *testing.T) {
	for _, full := range []bool{false, true} {
		t.Run(fmt.Sprint(full), func(t *testing.T) {
			ctx := context.Background()
			index := newDirectoryIndex(filepath.Join(t.TempDir(), "index"))
			defer index.close()
			root := RootRecord{ID: "root", Path: t.TempDir()}
			if err := index.attach(root); err != nil {
				t.Fatal(err)
			}
			scope := filepath.Join(root.Path, "protected")
			file := filepath.Join(scope, "keep.txt")
			batch := SubtreeSnapshotBatch{RootID: root.ID, ScopePath: root.Path, Entries: []EntryRecord{
				{RootID: root.ID, Path: scope, ParentPath: root.Path, Name: "protected", IsDir: true},
				{RootID: root.ID, Path: file, ParentPath: scope, Name: "keep.txt"},
			}}
			write := func(b SubtreeSnapshotBatch) {
				t.Helper()
				if err := index.writeScope(ctx, root.ID, root.Path, false, func(yield func(SubtreeSnapshotBatch) error) error { return yield(b) }); err != nil {
					t.Fatal(err)
				}
			}
			write(batch)
			if full {
				if err := index.beginBuild(root.ID); err != nil {
					t.Fatal(err)
				}
			}
			write(SubtreeSnapshotBatch{RootID: root.ID, ScopePath: root.Path, UnreadableScopes: []string{scope}})
			if full {
				if err := index.finishBuild(ctx, root); err != nil {
					t.Fatal(err)
				}
			}
			found := map[string]bool{}
			if err := index.visit(ctx, root.ID, func(entry EntryRecord) error { found[entry.Path] = true; return nil }); err != nil {
				t.Fatal(err)
			}
			if !found[file] || !found[scope] {
				t.Fatalf("lost inaccessible facts: %v", found)
			}
			write(SubtreeSnapshotBatch{RootID: root.ID, ScopePath: root.Path})
			if err := index.visit(ctx, root.ID, func(entry EntryRecord) error { t.Errorf("retained confirmed deletion: %s", entry.Path); return nil }); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// TestDirectoryScopeDeltaKeepsGeneration verifies that a small reconcile only
// journals changed facts, survives reopen and does not rewrite on an identical scan.
func TestDirectoryScopeDeltaKeepsGeneration(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "index")
	index := newDirectoryIndex(dir)
	root := RootRecord{ID: "delta-root", Path: t.TempDir()}
	if err := index.attach(root); err != nil {
		t.Fatal(err)
	}
	defer func() { index.close() }()
	scope := filepath.Join(root.Path, "changed")
	entries := make([]EntryRecord, 0, 100004)
	for i := 0; i < 100000; i++ {
		path := filepath.Join(scope, fmt.Sprintf("retained-%d.txt", i))
		entries = append(entries, EntryRecord{RootID: root.ID, Path: path, ParentPath: scope, Name: filepath.Base(path)})
	}
	old := EntryRecord{RootID: root.ID, Path: filepath.Join(scope, "old.txt"), ParentPath: scope, Name: "old.txt"}
	entries = append(entries, EntryRecord{RootID: root.ID, Path: scope, ParentPath: root.Path, Name: "changed", IsDir: true}, old)
	unreadable := make([]string, 5000)
	for i := range unreadable {
		unreadable[i] = filepath.Join(scope, fmt.Sprintf("denied-%d", i))
	}
	retained := filepath.Join(unreadable[0], "keep.txt")
	entries = append(entries,
		EntryRecord{RootID: root.ID, Path: unreadable[0], ParentPath: scope, Name: "denied-0", IsDir: true},
		EntryRecord{RootID: root.ID, Path: retained, ParentPath: unreadable[0], Name: "keep.txt"})

	if err := index.replaceRoot(ctx, root, func(yield func(SubtreeSnapshotBatch) error) error {
		return yield(SubtreeSnapshotBatch{RootID: root.ID, ScopePath: root.Path, Entries: entries})
	}); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(index.shardDirectory(root.ID), "current.json")
	before, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	replacements := make([]EntryRecord, 63)
	for i := range replacements {
		replacements[i] = old
		replacements[i].Name = fmt.Sprintf("new-%d.txt", i)
		replacements[i].Path = filepath.Join(scope, replacements[i].Name)
	}
	observedEntries := append(append([]EntryRecord(nil), entries[:100000]...), replacements...)
	write := func() {
		t.Helper()
		if err := index.writeScope(ctx, root.ID, scope, false, func(yield func(SubtreeSnapshotBatch) error) error {
			return yield(SubtreeSnapshotBatch{RootID: root.ID, ScopePath: scope, Entries: observedEntries, UnreadableScopes: unreadable})
		}); err != nil {
			t.Fatal(err)
		}
	}
	started := time.Now()
	write()
	t.Logf("100,000 retained scoped files, 5,000 unreadable scopes, 63 inserted and one deleted: %s", time.Since(started))
	after, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("small reconcile replaced the immutable generation")
	}
	if err := index.shards[root.ID].store.Read(func(view column.View) error {
		if len(view.Delta) != 64 {
			t.Fatalf("expected only 63 insertions and one deletion, got %v changes", view.Delta)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	revision := index.shards[root.ID].store.Revision()
	write()
	if index.shards[root.ID].store.Revision() != revision {
		t.Fatal("identical reconcile wrote a journal frame")
	}
	if err := index.close(); err != nil {
		t.Fatal(err)
	}
	index = newDirectoryIndex(dir)
	if err := index.attach(root); err != nil {
		t.Fatal(err)
	}
	count := 0
	if err := index.visit(ctx, root.ID, func(entry EntryRecord) error {
		if entry.Path == old.Path {
			t.Fatal("deleted fact survived journal replay")
		}
		count++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if count != 100066 {
		t.Fatalf("replayed count=%d", count)
	}
}
