package column

import (
	"reflect"
	"testing"
)

// TestFinishPostingsAfterDeletion keeps empty interned-name groups and missing
// parents intact while the finalizer reuses offsets as scatter cursors.
func TestFinishPostingsAfterDeletion(t *testing.T) {
	for _, removeAll := range []bool{false, true} {
		b := NewBuilder(t.TempDir(), 5)
		for _, r := range []Record{
			{ID: 12, ParentID: 10, Name: "shared", Version: 1},
			{ID: 13, ParentID: 99, Name: "unused", Version: 1},
			{ID: 10, ParentID: 5, Name: "parent", Flags: Directory, Version: 1},
			{ID: 14, ParentID: 10, Name: "shared", Version: 1},
			{ID: 13, ParentID: 99, Name: "shared", Version: 2},
			{ID: 15, ParentID: 5, Name: "deleted-last-name", Version: 1},
		} {
			if err := b.Add(r); err != nil {
				t.Fatal(err)
			}
		}
		deleted := []uint64{15}
		if removeAll {
			deleted = []uint64{10, 12, 13, 14, 15}
		}
		for _, id := range deleted {
			if err := b.Remove(id, 3); err != nil {
				t.Fatal(err)
			}
		}
		s, err := b.Finish()
		if err != nil {
			t.Fatal(err)
		}
		for id := 0; id < s.Names(); id++ {
			want, got := map[uint64]bool{}, map[uint64]bool{}
			for row := 1; row < s.Len(); row++ {
				if s.NameIDs[row] == uint32(id) {
					want[s.IDs[row]] = true
				}
			}
			for _, row := range s.NameRows[s.NameStarts[id]:s.NameStarts[id+1]] {
				got[s.IDs[row]] = true
			}
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("name %d: got %v want %v", id, got, want)
			}
		}
		if !removeAll {
			row, _ := s.Row(12)
			if s.IDs[s.Parents[row]] != 10 {
				t.Fatal("late parent lost")
			}
			row, _ = s.Row(13)
			if s.Parents[row] != NoRow {
				t.Fatal("missing parent became reachable")
			}
		} else if s.Len() != 1 {
			t.Fatal("deleted rows survived")
		}
		if _, err := b.Finish(); err == nil {
			t.Fatal("finished builder reused")
		}
		if err := b.Add(Record{ID: 20}); err == nil {
			t.Fatal("finished builder accepted a record")
		}
	}
}

// TestBuilderProgressCounts handles repeated facts, stale changes and resurrection
// without counting the structural root or rescanning partially built columns.
func TestBuilderProgressCounts(t *testing.T) {
	b := NewBuilder(t.TempDir(), 5)
	steps := []struct {
		record         Record
		remove         bool
		files, entries int64
	}{
		{record: Record{ID: 10, ParentID: 5, Name: "file", Version: 10}, files: 1, entries: 1},
		{record: Record{ID: 11, ParentID: 5, Name: "dir", Flags: Directory, Version: 10}, files: 1, entries: 2},
		{record: Record{ID: 10, ParentID: 5, Name: "stale", Version: 9}, files: 1, entries: 2},
		{record: Record{ID: 10, ParentID: 5, Name: "renamed", Version: 11}, files: 1, entries: 2},
		{record: Record{ID: 10, ParentID: 5, Name: "now-dir", Flags: Directory, Version: 12}, entries: 2},
		{record: Record{ID: 10, Version: 11}, remove: true, entries: 2},
		{record: Record{ID: 10, Version: 13}, remove: true, entries: 1},
		{record: Record{ID: 10, Version: 13}, remove: true, entries: 1},
		{record: Record{ID: 10, ParentID: 5, Name: "stale", Version: 12}, entries: 1},
		{record: Record{ID: 10, ParentID: 5, Name: "returned", Version: 14}, files: 1, entries: 2},
		{record: Record{ID: 12, Version: 20}, remove: true, files: 1, entries: 2},
		{record: Record{ID: 12, ParentID: 5, Name: "stale", Version: 19}, files: 1, entries: 2},
		{record: Record{ID: 12, ParentID: 5, Name: "new", Version: 21}, files: 2, entries: 3},
	}
	for step, tc := range steps {
		var err error
		if tc.remove {
			err = b.Remove(tc.record.ID, tc.record.Version)
		} else {
			err = b.Add(tc.record)
		}
		if err != nil {
			t.Fatal(err)
		}
		if files, entries := b.Counts(); files != tc.files || entries != tc.entries {
			t.Fatalf("step %d: files=%d entries=%d, want %d/%d", step, files, entries, tc.files, tc.entries)
		}
	}
}
