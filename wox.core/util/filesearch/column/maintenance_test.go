package column

import (
	"context"
	"fmt"
	"testing"
)

// TestCountsSurviveReopenAndParentDeletion checks the revision-zero cache case
// and descendant visibility before and after compaction.
func TestCountsSurviveReopenAndParentDeletion(t *testing.T) {
	ctx := context.Background()
	store, dir := fixtureStore(t)
	assertCounts := func(files, entries int64) {
		t.Helper()
		for i := 0; i < 2; i++ {
			f, e, err := store.Counts(ctx)
			if err != nil || f != files || e != entries {
				t.Fatalf("counts=(%d,%d) want=(%d,%d): %v", f, e, files, entries, err)
			}
		}
	}
	assertCounts(2, 4)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	var err error
	store, err = OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	assertCounts(2, 4)
	if err := store.Apply([]Change{{Record: Record{ID: 10}, Deleted: true}}, "deleted"); err != nil {
		t.Fatal(err)
	}
	assertCounts(1, 2)
	if err := store.Compact(ctx); err != nil {
		t.Fatal(err)
	}
	assertCounts(1, 2)
}

// TestCompactionThreshold tests actual bounded mutations rather than depending
// on timing or forcing large process-memory allocations.
func TestCompactionThreshold(t *testing.T) {
	store, _ := fixtureStore(t)
	if store.NeedsCompaction() {
		t.Fatal("fresh generation needs compaction")
	}
	changes := make([]Change, 10000)
	for i := range changes {
		changes[i] = Change{Record: Record{ID: uint64(100 + i), ParentID: 5, Name: fmt.Sprintf("entry-%d", i)}}
	}
	if err := store.Apply(changes[:9999], "below"); err != nil {
		t.Fatal(err)
	}
	if store.NeedsCompaction() {
		t.Fatal("compaction before threshold")
	}
	if err := store.Apply(changes[9999:], "threshold"); err != nil {
		t.Fatal(err)
	}
	if !store.NeedsCompaction() {
		t.Fatal("missed compaction threshold")
	}
	if err := store.Compact(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.NeedsCompaction() {
		t.Fatal("compaction did not clear overlay")
	}
}

// TestVisibleCountsMatchAncestry covers late parents, unreachable subtrees,
// structural ancestors and overlay changes without relying on timing.
func TestVisibleCountsMatchAncestry(t *testing.T) {
	builder := NewBuilder(t.TempDir(), 5)
	for _, record := range []Record{
		{ID: 20, ParentID: 10, Name: "child.txt"},
		{ID: 10, ParentID: 5, Name: "dir", Flags: Directory},
		{ID: 30, ParentID: 10, Name: "structural", Flags: Directory | Structural},
		{ID: 31, ParentID: 30, Name: "deep.txt"},
		{ID: 40, ParentID: 999, Name: "orphan", Flags: Directory},
		{ID: 41, ParentID: 40, Name: "hidden.txt"},
	} {
		if err := builder.Add(record); err != nil {
			t.Fatal(err)
		}
	}
	base, err := builder.Finish()
	if err != nil {
		t.Fatal(err)
	}
	view := View{Base: base, Delta: map[uint64]Change{}}
	assertCounts := func() {
		t.Helper()
		var wantFiles, wantEntries int64
		count := func(record Record) {
			if record.Flags&Structural == 0 && view.Reachable(record.ID) {
				wantEntries++
				if record.Flags&Directory == 0 {
					wantFiles++
				}
			}
		}
		for row := range base.Len() {
			if _, changed := view.Delta[base.IDs[row]]; !changed {
				count(base.Record(uint32(row)))
			}
		}
		for _, change := range view.Delta {
			if !change.Deleted {
				count(change.Record)
			}
		}
		files, entries, err := countVisibleEntries(context.Background(), view)
		if err != nil || files != wantFiles || entries != wantEntries {
			t.Fatalf("counts=(%d,%d), want=(%d,%d), err=%v", files, entries, wantFiles, wantEntries, err)
		}
	}
	assertCounts()
	view.Delta[10] = Change{Record: Record{ID: 10}, Deleted: true}
	assertCounts()
	view.Delta[10] = Change{Record: Record{ID: 10, ParentID: 5, Name: "restored", Flags: Directory}}
	view.Delta[50] = Change{Record: Record{ID: 50, ParentID: 30, Name: "new.txt"}}
	assertCounts()
	view.Delta[10] = Change{Record: Record{ID: 10, ParentID: 40, Name: "moved", Flags: Directory}}
	assertCounts()
	view.Delta[40] = Change{Record: Record{ID: 40, ParentID: 5, Name: "connected", Flags: Directory}}
	assertCounts()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := countVisibleEntries(ctx, view); err != context.Canceled {
		t.Fatalf("cancelled count: %v", err)
	}
}
