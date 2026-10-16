package filesearch

import (
	"os"
	"path/filepath"
	"testing"
)

// Parent deletion must retire descendants and their shards, not explicit child roots.
func TestDeleteRootCascadesDynamicDescendants(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	base := t.TempDir()
	roots := []RootRecord{
		{ID: "parent", Path: base, Kind: RootKindUser},
		{ID: "child", Path: filepath.Join(base, "child"), Kind: RootKindDynamic, DynamicParentRootID: "parent"},
		{ID: "nested", Path: filepath.Join(base, "child", "nested"), Kind: RootKindDynamic, DynamicParentRootID: "child"},
		{ID: "explicit", Path: filepath.Join(base, "explicit"), Kind: RootKindUser},
	}
	for _, root := range roots {
		mustInsertRoot(t, ctx, db, root)
		if _, err := db.db.ExecContext(ctx, `INSERT INTO directories (path,root_id,parent_path,last_scan_time,"exists") VALUES (?,?,?,0,1)`, root.Path, root.ID, base); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(db.directory.shardDirectory(root.ID), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := db.DeleteRoot(ctx, "parent"); err != nil {
			t.Fatal(err)
		}
	}
	remaining, err := db.ListRoots(ctx)
	if err != nil || len(remaining) != 1 || remaining[0].ID != "explicit" {
		t.Fatalf("remaining roots: %#v, %v", remaining, err)
	}
	var count int
	if err := db.db.QueryRowContext(ctx, "SELECT count(*) FROM directories").Scan(&count); err != nil || count != 1 {
		t.Fatalf("directories count=%d err=%v", count, err)
	}
	for _, root := range roots {
		_, err := os.Stat(db.directory.shardDirectory(root.ID))
		if root.ID == "explicit" {
			if err != nil {
				t.Fatal(err)
			}
			continue
		}
		if !os.IsNotExist(err) {
			t.Fatalf("shard still exists: %s %v", root.ID, err)
		}
		if db.directory.shards[root.ID] != nil {
			t.Fatalf("shard still attached: %s", root.ID)
		}
	}
}
