package column

import (
	"fmt"
	"math"
	"testing"
)

// TestBuilderRowProbeWrapAndGrowth exercises a collision chain across the end
// of the table, then checks that growth preserves every row and a missing key.
func TestBuilderRowProbeWrapAndGrowth(t *testing.T) {
	var ids []uint64
	for id := uint64(0); len(ids) < 20; id++ {
		if rowHash(id)&15 == 15 {
			ids = append(ids, id)
		}
	}
	b := NewBuilder(t.TempDir(), ids[0])
	for i, id := range ids[1:19] {
		if err := b.Add(Record{ID: id, ParentID: ids[0], Name: fmt.Sprint(id)}); err != nil {
			t.Fatal(err)
		}
		for row, key := range ids[:i+2] {
			got, ok := b.lookupRow(key)
			if !ok || got != uint32(row) {
				t.Fatalf("after insertion %d, ID %d: row=%d found=%v", i, key, got, ok)
			}
		}
		if b.Contains(ids[19]) {
			t.Fatal("missing colliding ID found")
		}
	}
	if len(b.rows) != 32 {
		t.Fatalf("expected growth to 32 slots, got %d", len(b.rows))
	}
}

// TestBuilderRowsSurvivePackingAndReplacement covers arbitrary 64-bit keys,
// tombstones, resurrection and row relocation after deletion.
func TestBuilderRowsSurvivePackingAndReplacement(t *testing.T) {
	b := NewBuilder(t.TempDir(), 0)
	ids := []uint64{math.MaxUint64}
	for i := uint64(1); i < 4096; i++ {
		ids = append(ids, (i<<48)|i)
	}
	for _, id := range ids {
		if err := b.Add(Record{ID: id, ParentID: 0, Name: "old", Version: 10}); err != nil {
			t.Fatal(err)
		}
	}
	for i, id := range ids {
		if i%3 == 0 {
			if err := b.Remove(id, 11); err != nil {
				t.Fatal(err)
			}
		}
	}
	b.pruneDeleted()
	for i, id := range ids {
		row, ok := b.lookupRow(id)
		if ok != (i%3 != 0) || ok && b.base.IDs[row] != id {
			t.Fatalf("packed ID %d: row=%d found=%v", id, row, ok)
		}
		// Old events cannot resurrect a removed row; newer ones can append it
		// at a different row and must remain findable after subsequent growth.
		if err := b.Add(Record{ID: id, ParentID: 0, Name: "stale", Version: 9}); err != nil {
			t.Fatal(err)
		}
		if b.Contains(id) != ok {
			t.Fatal("stale event changed presence")
		}
		if err := b.Add(Record{ID: id, ParentID: 0, Name: "new", Version: 12}); err != nil {
			t.Fatal(err)
		}
	}
	s, err := b.Finish()
	if err != nil {
		t.Fatal(err)
	}
	if s.Len() != len(ids)+1 {
		t.Fatalf("rows=%d", s.Len())
	}
	for _, id := range ids {
		row, found := s.Row(id)
		if !found || s.Record(row).Name != "new" || s.Record(row).Version != 12 {
			t.Fatalf("replacement lost for ID %d", id)
		}
	}
	if b.Contains(0) {
		t.Fatal("finished table retained root")
	}
}
