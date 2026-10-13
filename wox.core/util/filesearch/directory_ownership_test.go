package filesearch

import (
	"path/filepath"
	"testing"
)

// TestDirectoryOwnershipRecoversInterruptedPromotion models a crash after the
// destination and root metadata commit but before the old parent is hidden.
func TestDirectoryOwnershipRecoversInterruptedPromotion(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	parent := RootRecord{ID: "parent", Path: t.TempDir(), Kind: RootKindUser, Status: RootStatusIdle}
	child := RootRecord{ID: "child", Path: filepath.Join(parent.Path, "hot"), Kind: RootKindDynamic, DynamicParentRootID: parent.ID, PolicyRootPath: parent.Path, Status: RootStatusIdle}
	mustInsertRoot(t, ctx, db, parent)
	file := filepath.Join(child.Path, "ghost.txt")
	entries := []EntryRecord{{RootID: parent.ID, Path: child.Path, ParentPath: parent.Path, IsDir: true}, {RootID: parent.ID, Path: file, ParentPath: child.Path}}
	if err := db.ReplaceRootEntries(ctx, parent, entries, nil); err != nil {
		t.Fatal(err)
	}
	if err := db.copyColumnScope(ctx, parent, child, child.Path); err != nil {
		t.Fatal(err)
	}
	if err := execRootUpsert(ctx, db.db, child); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	restored, err := NewFileSearchDB(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if err := restored.directory.applyEntries(ctx, child.ID, nil, []string{file}, "removed"); err != nil {
		t.Fatal(err)
	}
	results, err := restored.directory.search(ctx, SearchQuery{Raw: "ghost.txt"}, 20)
	if err != nil || len(results) != 0 {
		t.Fatalf("stale parent copy resurfaced: %+v %v", results, err)
	}
	if err := restored.DemoteDynamicRoot(ctx, parent, child); err != nil {
		t.Fatal(err)
	}
	results, err = restored.directory.search(ctx, SearchQuery{Raw: "ghost.txt"}, 20)
	if err != nil || len(results) != 0 {
		t.Fatalf("demotion restored deleted file: %+v %v", results, err)
	}
}

// TestDirectoryOwnershipKeepsParentUntilMetadataCommit covers the opposite
// crash boundary: an unpublished child must not steal ownership on restart.
func TestDirectoryOwnershipKeepsParentUntilMetadataCommit(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	parent := RootRecord{ID: "parent", Path: t.TempDir(), Kind: RootKindUser, Status: RootStatusIdle}
	child := RootRecord{ID: "child", Path: filepath.Join(parent.Path, "hot"), Kind: RootKindDynamic, DynamicParentRootID: parent.ID, PolicyRootPath: parent.Path}
	mustInsertRoot(t, ctx, db, parent)
	file := filepath.Join(child.Path, "retained.txt")
	if err := db.ReplaceRootEntries(ctx, parent, []EntryRecord{{RootID: parent.ID, Path: file, ParentPath: child.Path}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := db.copyColumnScope(ctx, parent, child, child.Path); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	restored, err := NewFileSearchDB(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	assertEntryRoot(t, ctx, restored, file, parent.ID)
}
