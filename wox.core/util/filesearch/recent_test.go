package filesearch

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"wox/util/filesearch/column"
)

// Exercise persisted metadata, filtering before limiting, and cancellation.
func TestRecentModified(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	root := RootRecord{ID: "recent", Path: t.TempDir(), Kind: RootKindUser, Status: RootStatusIdle}
	mustInsertRoot(t, ctx, db, root)
	entries := []EntryRecord{}
	for i, name := range []string{"old.txt", "new.txt", "hidden.txt", "folder", "unknown.txt"} {
		mtime := int64(i + 1)
		if name == "unknown.txt" {
			mtime = 0
		}
		entries = append(entries, EntryRecord{RootID: root.ID, Path: filepath.Join(root.Path, name), ParentPath: root.Path, Name: name, Mtime: mtime, IsDir: name == "folder"})
	}
	if err := db.ReplaceRootEntries(ctx, root, entries, nil); err != nil {
		t.Fatal(err)
	}
	engine := &Engine{db: db}
	results, err := engine.RecentModified(ctx, 2, func(item SearchResult) bool { return !item.IsDir && item.Name != "hidden.txt" })
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].Name != "new.txt" || results[1].Name != "old.txt" || results[0].Score <= results[1].Score {
		t.Fatalf("unexpected recents: %#v", results)
	}
	updated := column.Record{ID: directoryEntryID(entries[0].Path), ParentID: directoryEntryID(root.Path), Name: "old.txt", Modified: 10}
	if err := db.directory.shards[root.ID].store.Apply([]column.Change{{Record: updated}, {Record: column.Record{ID: directoryEntryID(entries[1].Path)}, Deleted: true}}, ""); err != nil {
		t.Fatal(err)
	}
	results, err = engine.RecentModified(ctx, 2, func(item SearchResult) bool { return !item.IsDir && item.Name != "hidden.txt" })
	if err != nil || len(results) != 1 || results[0].Name != "old.txt" || results[0].Mtime != 10 {
		t.Fatalf("overlay recents: %#v, %v", results, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := engine.RecentModified(canceled, 2, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}
