package column

import "testing"

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
