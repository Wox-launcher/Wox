package filesearch

import (
	"context"
	"reflect"
	"testing"

	"wox/util/filesearch/column"
	"wox/util/fuzzymatch"
)

// TestIdleReleasePreservesPinyinAndDelta covers the actual index release boundary,
// including both pronunciations and overlay records that cannot be evicted.
func TestIdleReleasePreservesPinyinAndDelta(t *testing.T) {
	store, err := column.OpenStore(t.TempDir(), prepareColumnNames)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Publish(columnQueryFixture(t).Base, "base", "", store.Revision()); err != nil {
		t.Fatal(err)
	}
	if err := store.Apply([]column.Change{{Record: column.Record{ID: 50, ParentID: 1, Name: "重庆项目.txt"}}}, "delta"); err != nil {
		t.Fatal(err)
	}
	index := newDirectoryIndex("")
	index.shards["test"] = &directoryShard{store: store}
	engine := &Engine{db: &FileSearchDB{directory: index}}
	for _, raw := range []string{"chongqing", "zhongqing", "cq", "xiangmu", "xm", "项目", "*.txt", "Amiga/"} {
		query := SearchQuery{Raw: raw}
		before, err := index.search(context.Background(), query, 100)
		if err != nil || len(before) == 0 {
			t.Fatalf("before %q: %v %v", raw, before, err)
		}
		if err := engine.ReleaseIdleMemory(); err != nil {
			t.Fatal(err)
		}
		fuzzymatch.ReleaseIdleCaches()
		fuzzymatch.ReleasePinyinDictionary()
		after, err := index.search(context.Background(), query, 100)
		if err != nil || !reflect.DeepEqual(before, after) {
			t.Fatalf("after %q: %v != %v, %v", raw, after, before, err)
		}
	}
}
