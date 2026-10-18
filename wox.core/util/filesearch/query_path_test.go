package filesearch

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"wox/util/filesearch/column"
)

// TestColumnPathQueryLiveTree compares candidate retrieval against exhaustive
// live-path scoring, including cross-component fuzzy matches and moved edges.
func TestColumnPathQueryLiveTree(t *testing.T) {
	builder := column.NewBuilder(filepath.Join(t.TempDir(), "root"), 1)
	records := []column.Record{
		{ID: 2, ParentID: 1, Name: "a", Flags: column.Directory},
		{ID: 3, ParentID: 2, Name: "b", Flags: column.Directory},
		{ID: 4, ParentID: 3, Name: "café", Flags: column.Directory},
		{ID: 5, ParentID: 4, Name: "report.txt", Flags: column.SizeKnown | column.ModifiedKnown, Size: 123, Modified: 456},
		{ID: 6, ParentID: 1, Name: "目标 folder", Flags: column.Directory},
		{ID: 7, ParentID: 6, Name: "old", Flags: column.Directory},
		{ID: 8, ParentID: 7, Name: "notes.md"},
		{ID: 9, ParentID: 6, Name: "empty", Flags: column.Directory},
		{ID: 10, ParentID: 1, Name: " white space ", Flags: column.Directory},
		{ID: 11, ParentID: 10, Name: " report .txt"},
		{ID: 20, ParentID: 3, Name: "ca", Flags: column.Directory},
		{ID: 21, ParentID: 20, Name: "fe", Flags: column.Directory},
		{ID: 22, ParentID: 21, Name: "cafe", Flags: column.Directory},
		{ID: 23, ParentID: 22, Name: "report.txt"},
		{ID: 24, ParentID: 3, Name: "q", Flags: column.Directory},
		{ID: 25, ParentID: 24, Name: "ae", Flags: column.Directory},
		{ID: 26, ParentID: 25, Name: "report.txt"},
	}
	for _, record := range records {
		if err := builder.Add(record); err != nil {
			t.Fatal(err)
		}
	}
	base, err := builder.Finish()
	if err != nil {
		t.Fatal(err)
	}
	view := column.View{Base: base}
	for _, overlay := range []bool{false, true} {
		if overlay {
			view.Delta = map[uint64]column.Change{
				3:  {Record: column.Record{ID: 3, ParentID: 6, Name: "moved", Flags: column.Directory}},
				7:  {Record: column.Record{ID: 7}, Deleted: true},
				12: {Record: column.Record{ID: 12, ParentID: 2, Name: "new", Flags: column.Directory}},
				13: {Record: column.Record{ID: 13, ParentID: 12, Name: "new.txt"}},
				14: {Record: column.Record{ID: 14, ParentID: 999, Name: "orphan.txt"}},
			}
		}
		for _, raw := range []string{
			"ab/cafe", "a/b/cafe", "a/b/café", "a/b/café .txt", "a/b/café report", "ab/cafe missing", "ab/qae",
			"目标/moved", "目标/moved/cafe report", "目标/old", "a/new", "a/new/ .txt", "a/new/*",
			"white/", "white/ .txt", "*/report.txt", `"café/report"`, `"café/" .txt`, "/", `\`, "a/", "a/b/", "a/b/cafe/",
			base.Root, filepath.VolumeName(base.Root) + string(filepath.Separator),
			filepath.Join(base.Root, "a", "b", "café"),
		} {
			query := normalizeSearchQuery(SearchQuery{Raw: raw})
			for _, limit := range []int{1, 3, 100, 0} {
				got, err := searchColumnView(context.Background(), view, query, limit)
				want := exhaustiveColumnPaths(view, query, limit)
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("overlay=%v raw=%q limit=%d: got=%+v want=%+v error=%v", overlay, raw, limit, got, want, err)
				}
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := searchColumnView(ctx, view, SearchQuery{Raw: "a/b"}, 5); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled path search: %v", err)
	}
}

// TestColumnParallelPathRetrieval crosses the worker-partition threshold and
// checks both selective retrieval and the bounded broad-query fallback.
func TestColumnParallelPathRetrieval(t *testing.T) {
	builder := column.NewBuilder(t.TempDir(), 1)
	for i := range 40000 {
		directory := uint64(2 + i*2)
		if err := builder.Add(column.Record{ID: directory, ParentID: 1, Name: fmt.Sprintf("folder-%05d", i), Flags: column.Directory}); err != nil {
			t.Fatal(err)
		}
		if err := builder.Add(column.Record{ID: directory + 1, ParentID: directory, Name: "report.txt"}); err != nil {
			t.Fatal(err)
		}
	}
	base, err := builder.Finish()
	if err != nil {
		t.Fatal(err)
	}
	view := column.View{Base: base}
	for _, raw := range []string{"folder-01234/ .txt", "folder/", base.Root} {
		query := normalizeSearchQuery(SearchQuery{Raw: raw})
		got, err := searchColumnView(context.Background(), view, query, 5)
		want := exhaustiveColumnPaths(view, query, 5)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("%q: got=%+v want=%+v error=%v", raw, got, want, err)
		}
	}
}

// exhaustiveColumnPaths is an independent retrieval oracle: every live record
// resolves its full path before applying the pre-existing shared scorer.
func exhaustiveColumnPaths(view column.View, query SearchQuery, limit int) []SearchResult {
	var records []column.Record
	for row := range view.Base.Len() {
		record := view.Base.Record(uint32(row))
		if _, changed := view.Delta[record.ID]; !changed {
			records = append(records, record)
		}
	}
	for _, change := range view.Delta {
		if !change.Deleted {
			records = append(records, change.Record)
		}
	}
	var results []SearchResult
	for _, record := range records {
		if !columnExtensionEligible(query, record) {
			continue
		}
		path, ok := view.Path(record.ID)
		if !ok {
			continue
		}
		isDir := record.Flags&column.Directory != 0
		matched, score := scoreDocAgainstQuery(query, docRecord{Path: path, IsDir: isDir})
		if !matched {
			continue
		}
		if pathInWindowsRecycleBin(path) {
			score -= recycleBinRankPenalty
		}
		result := SearchResult{Path: path, Name: record.Name, ParentPath: filepath.Dir(path), IsDir: isDir, Score: score}
		if record.ID == view.Base.IDs[0] {
			result.Name = filepath.Base(view.Base.Root)
		}
		if record.Flags&column.SizeKnown != 0 {
			result.Size = record.Size
		}
		if record.Flags&column.ModifiedKnown != 0 {
			result.Mtime = record.Modified
		}
		results = append(results, result)
	}
	return sortAndLimitResults(results, limit)
}
