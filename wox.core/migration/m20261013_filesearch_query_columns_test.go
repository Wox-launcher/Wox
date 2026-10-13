package migration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"wox/util/filesearch/column"
)

// TestQueryColumnsMigrationPreservesJournal limits migration to published generations.
func TestQueryColumnsMigrationPreservesJournal(t *testing.T) {
	directory := t.TempDir()
	shard := filepath.Join(directory, "native-columns", "volume")
	store, err := column.OpenStore(shard)
	if err != nil {
		t.Fatal(err)
	}
	builder := column.NewBuilder(t.TempDir(), 1)
	if err := builder.Add(column.Record{ID: 2, ParentID: 1, Name: "项目.txt"}); err != nil {
		t.Fatal(err)
	}
	base, err := builder.Finish()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Publish(base, "before", "policy", store.Revision()); err != nil {
		t.Fatal(err)
	}
	if err := store.Apply([]column.Change{{Record: column.Record{ID: 3, ParentID: 1, Name: "new.txt"}}}, "after"); err != nil {
		t.Fatal(err)
	}
	store.Close()
	for _, name := range []string{"contentsearch.db", "orphan.columns"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("unrelated"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if err := upgradeFileSearchQueryColumns(context.Background(), directory); err != nil {
			t.Fatal(err)
		}
	}
	store, err = column.OpenStore(shard)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Read(func(v column.View) error {
		if v.Cursor != "after" || len(v.Delta) != 1 || v.Base.Len() != 2 {
			t.Fatalf("migration changed journal or base: cursor=%s delta=%d rows=%d", v.Cursor, len(v.Delta), v.Base.Len())
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
