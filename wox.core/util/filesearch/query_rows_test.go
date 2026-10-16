package filesearch

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"wox/util/filesearch/column"
)

// TestColumnPathOrderingAndOverlay compares lazy tie-breaking with joined paths,
// including prefix punctuation and ancestry changed after snapshot publication.
func TestColumnPathOrderingAndOverlay(t *testing.T) {
	b := column.NewBuilder(t.TempDir(), 1)
	id := uint64(2)
	for _, name := range []string{"a", "a-b", "a.b", "a_", "A", "中", "z"} {
		parent := id
		id++
		if err := b.Add(column.Record{ID: parent, ParentID: 1, Name: name, Flags: column.Directory}); err != nil {
			t.Fatal(err)
		}
		for _, file := range []string{"readme", "README", "ss"} {
			if err := b.Add(column.Record{ID: id, ParentID: parent, Name: file}); err != nil {
				t.Fatal(err)
			}
			id++
		}
	}
	base, err := b.Finish()
	if err != nil {
		t.Fatal(err)
	}
	v := column.View{Base: base}
	for _, overlay := range []bool{false, true} {
		if overlay {
			v.Delta = map[uint64]column.Change{2: {Record: column.Record{ID: 2, ParentID: 1, Name: "zz", Flags: column.Directory}}, 6: {Record: column.Record{ID: 6}, Deleted: true}}
		}
		for i := 1; i < base.Len(); i++ {
			for j := 1; j < base.Len(); j++ {
				a, okA := v.Path(base.IDs[i])
				bb, okB := v.Path(base.IDs[j])
				if !okA || !okB {
					continue
				}
				want := strings.Compare(a, bb)
				if len(a) != len(bb) {
					if len(a) < len(bb) {
						want = -1
					} else {
						want = 1
					}
				}
				if got := compareColumnPaths(v, uint32(i), uint32(j)); got != want {
					t.Fatalf("%q vs %q: %d != %d", a, bb, got, want)
				}
			}
		}
		for _, q := range []string{"readme", "ss"} {
			query := normalizeSearchQuery(SearchQuery{Raw: q})
			var want []SearchResult
			for i := 1; i < base.Len(); i++ {
				record, ok := v.Lookup(base.IDs[i])
				if !ok {
					continue
				}
				path, ok := v.Path(record.ID)
				if !ok {
					continue
				}
				if matched, score := scoreDocAgainstQuery(query, docRecord{Path: path, IsDir: record.Flags&column.Directory != 0}); matched {
					want = append(want, SearchResult{Path: path, Name: record.Name, ParentPath: filepath.Dir(path), IsDir: record.Flags&column.Directory != 0, Score: score})
				}
			}
			for _, limit := range []int{1, 3, 100} {
				got, err := searchColumnView(context.Background(), v, query, limit)
				expected := sortAndLimitResults(append([]SearchResult(nil), want...), limit)
				if err != nil || !reflect.DeepEqual(got, expected) {
					t.Fatalf("overlay=%v q=%q limit=%d: %v\n%+v\n%+v", overlay, q, limit, err, got, expected)
				}
			}
		}
	}
}

// TestColumnParallelCandidatesAreComplete exercises word/worker boundaries and
// bounded group merging against exhaustive scoring, including repeated names.
func TestColumnParallelCandidatesAreComplete(t *testing.T) {
	b := column.NewBuilder(t.TempDir(), 1)
	for i := 0; i < 33017; i++ {
		name := fmt.Sprintf("item_%05d_ss.txt", i)
		if i%19 == 0 {
			name = "ss"
		}
		if err := b.Add(column.Record{ID: uint64(i + 2), ParentID: 1, Name: name}); err != nil {
			t.Fatal(err)
		}
	}
	base, err := b.Finish()
	if err != nil {
		t.Fatal(err)
	}
	v := column.View{Base: base}
	for _, raw := range []string{"ss", "item .txt", "*17*", "absent"} {
		q := normalizeSearchQuery(SearchQuery{Raw: raw})
		var want []SearchResult
		for row := 1; row < base.Len(); row++ {
			r := base.Record(uint32(row))
			path, _ := base.Path(uint32(row))
			if ok, score := scoreDocAgainstQuery(q, docRecord{Path: path}); ok && columnExtensionEligible(q, r) {
				want = append(want, SearchResult{Path: path, Name: r.Name, ParentPath: filepath.Dir(path), Score: score})
			}
		}
		got, err := searchColumnView(context.Background(), v, q, 100)
		want = sortAndLimitResults(want, 100)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("%q: err=%v got=%d want=%d", raw, err, len(got), len(want))
		}
	}
}
