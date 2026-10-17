package filesearch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A one-level directory relist must not descend into child directories the
// index already holds, yet must fully walk directories new to the index and
// prune the stored trees of children that vanished or became files.
func TestDirectoryJobRelistsOneLevel(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()

	rootPath := t.TempDir()
	parent := filepath.Join(rootPath, "parent")
	mustMkdirAll(t, filepath.Join(parent, "known"))
	mustMkdirAll(t, filepath.Join(parent, "gone", "deep"))
	mustMkdirAll(t, filepath.Join(parent, "swap"))
	mustWriteTestFile(t, filepath.Join(parent, "known", "keptleaf.txt"), "kept")
	mustWriteTestFile(t, filepath.Join(parent, "gone", "deep", "goneleaf.txt"), "gone")
	mustWriteTestFile(t, filepath.Join(parent, "swap", "swapleaf.txt"), "swap")

	root := RootRecord{ID: "root-directory-relist", Path: rootPath, Kind: RootKindUser, Status: RootStatusIdle, CreatedAt: now, UpdatedAt: now}
	mustInsertRoot(t, ctx, db, root)
	NewScanner(db).scanAllRootsWithReason(ctx, "test")
	if len(searchColumnsForTest(t, db, "goneleaf", 10)) == 0 {
		t.Fatal("expected initial scan to index goneleaf.txt")
	}

	// Without an event of its own, a file added inside a known child stays
	// unindexed: relisting parent must not rescan known children.
	mustWriteTestFile(t, filepath.Join(parent, "known", "sneakyleaf.txt"), "sneaky")
	if err := os.RemoveAll(filepath.Join(parent, "gone")); err != nil {
		t.Fatalf("remove directory: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(parent, "swap")); err != nil {
		t.Fatalf("remove directory: %v", err)
	}
	mustWriteTestFile(t, filepath.Join(parent, "swap"), "now a file")
	mustMkdirAll(t, filepath.Join(parent, "fresh", "nested"))
	mustWriteTestFile(t, filepath.Join(parent, "fresh", "nested", "freshleaf.txt"), "fresh")
	mustWriteTestFile(t, filepath.Join(parent, "siblingfile.txt"), "sibling")

	job := Job{JobID: "relist", RootID: root.ID, RootPath: rootPath, ScopePath: parent, Kind: JobKindDirectory}
	if _, err := db.ApplyJobStream(ctx, root, job, NewSnapshotBuilder(nil), nil); err != nil {
		t.Fatalf("apply directory job: %v", err)
	}

	for _, name := range []string{"keptleaf", "freshleaf", "nested", "siblingfile", "swap"} {
		if len(searchColumnsForTest(t, db, name, 10)) == 0 {
			t.Fatalf("expected %s to be indexed after relist", name)
		}
	}
	for _, name := range []string{"sneakyleaf", "goneleaf", "deep", "swapleaf"} {
		if results := searchColumnsForTest(t, db, name, 10); len(results) != 0 {
			t.Fatalf("expected %s to be absent after relist, got %#v", name, results)
		}
	}
	for _, result := range searchColumnsForTest(t, db, "swap", 10) {
		if result.Path == filepath.Join(parent, "swap") && result.IsDir {
			t.Fatalf("expected swap to be indexed as a file, got %#v", result)
		}
	}

	// A vanished directory scope takes its whole stored tree with it.
	if err := os.RemoveAll(parent); err != nil {
		t.Fatalf("remove parent: %v", err)
	}
	if _, err := db.ApplyJobStream(ctx, root, job, NewSnapshotBuilder(nil), nil); err != nil {
		t.Fatalf("apply directory job for vanished scope: %v", err)
	}
	for _, name := range []string{"parent", "keptleaf", "freshleaf"} {
		if results := searchColumnsForTest(t, db, name, 10); len(results) != 0 {
			t.Fatalf("expected %s to be pruned with its vanished parent, got %#v", name, results)
		}
	}
}
