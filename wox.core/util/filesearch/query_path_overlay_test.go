package filesearch

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
	"wox/util/filesearch/column"
)

// TestPathLargeMoveAvoidsUnboundedCache checks the uncached invalidation path
// above the cache cap, including descendants whose own records never changed.
func TestPathLargeMoveAvoidsUnboundedCache(t *testing.T) {
	b := column.NewBuilder(t.TempDir(), 1)
	if err := b.Add(column.Record{ID: 2, ParentID: 1, Name: "old", Flags: column.Directory}); err != nil {
		t.Fatal(err)
	}
	for i := range 4100 {
		id := uint64(3 + i*2)
		for _, r := range []column.Record{{ID: id, ParentID: 2, Name: fmt.Sprintf("folder-%04d", i), Flags: column.Directory}, {ID: id + 1, ParentID: id, Name: "report.txt"}} {
			if err := b.Add(r); err != nil {
				t.Fatal(err)
			}
		}
	}
	base, err := b.Finish()
	if err != nil {
		t.Fatal(err)
	}
	v := (column.View{Base: base, Delta: map[uint64]column.Change{2: {Record: column.Record{ID: 2, ParentID: 1, Name: "moved", Flags: column.Directory}}}}).PrepareQueryView()
	if v.Query.PathsCached {
		t.Fatal("large move retained every directory path")
	}
	for _, raw := range []string{"moved/folder-0042", "old/folder-0042", "moved/folder report"} {
		q := normalizeSearchQuery(SearchQuery{Raw: raw})
		got, err := searchColumnView(context.Background(), v, q, 5)
		if err != nil || !reflect.DeepEqual(got, exhaustiveColumnPaths(v, q, 5)) {
			t.Fatalf("%s: got=%+v error=%v", raw, got, err)
		}
	}
}

// TestIndexedPathResultsOwnStrings keeps results valid after unmapping a store.
func TestIndexedPathResultsOwnStrings(t *testing.T) {
	store, err := column.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	b := column.NewBuilder(root, 1)
	for _, r := range []column.Record{{ID: 2, ParentID: 1, Name: "folder", Flags: column.Directory}, {ID: 3, ParentID: 2, Name: "report.txt"}} {
		if err := b.Add(r); err != nil {
			t.Fatal(err)
		}
	}
	base, err := b.Finish()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Publish(base, "", "", store.Revision()); err != nil {
		t.Fatal(err)
	}
	var results []SearchResult
	if err := store.Read(func(v column.View) error {
		var err error
		results, err = searchColumnView(context.Background(), v, SearchQuery{Raw: "folder/"}, 5)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].Path != filepath.Join(root, "folder") || results[1].Name != "report.txt" || results[1].ParentPath != filepath.Join(root, "folder") {
		t.Fatalf("results lost owned strings: %+v", results)
	}
}
