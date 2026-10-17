package filesearch

import (
	"os"
	"path/filepath"
	"testing"
)

// Deleting a root retires its row and shard but leaves overlapping roots alone.
func TestDeleteRootRemovesOnlyItsShard(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	base := t.TempDir()
	roots := []RootRecord{
		{ID: "parent", Path: base, Kind: RootKindUser},
		{ID: "nested", Path: filepath.Join(base, "nested"), Kind: RootKindUser},
	}
	for _, root := range roots {
		mustInsertRoot(t, ctx, db, root)
		if err := os.MkdirAll(db.directory.shardDirectory(root.ID), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if err := db.DeleteRoot(ctx, "parent"); err != nil {
			t.Fatal(err)
		}
	}
	remaining, err := db.ListRoots(ctx)
	if err != nil || len(remaining) != 1 || remaining[0].ID != "nested" {
		t.Fatalf("remaining roots: %#v, %v", remaining, err)
	}
	if _, err := os.Stat(db.directory.shardDirectory("parent")); !os.IsNotExist(err) {
		t.Fatalf("parent shard still exists: %v", err)
	}
	if db.directory.shards["parent"] != nil {
		t.Fatal("parent shard still attached")
	}
	if _, err := os.Stat(db.directory.shardDirectory("nested")); err != nil {
		t.Fatalf("nested shard removed: %v", err)
	}
}
