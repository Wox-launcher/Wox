package column

import (
	"os"
	"path/filepath"
	"testing"
)

// fixture deliberately emits children before parents, as a native MFT stream may.
func fixture(t *testing.T) *Snapshot {
	t.Helper()
	b := NewBuilder(t.TempDir(), 5)
	for _, r := range []Record{
		{ID: 12, ParentID: 10, Name: "项目.txt", Flags: SizeKnown | ModifiedKnown, Size: 42, Modified: 123, Version: 9},
		{ID: 13, ParentID: 11, Name: "项目.txt"},
		{ID: 10, ParentID: 5, Name: "one", Flags: Directory},
		{ID: 11, ParentID: 5, Name: "two", Flags: Directory},
	} {
		if err := b.Add(r); err != nil {
			t.Fatal(err)
		}
	}
	s, err := b.Finish()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TestMappedRoundTrip guards deduplication, late parents, and native metadata together.
func TestMappedRoundTrip(t *testing.T) {
	s := fixture(t)
	path := filepath.Join(t.TempDir(), "index.columns")
	if err := Write(path, s); err != nil {
		t.Fatal(err)
	}
	mapped, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer mapped.Close()
	if mapped.Len() != 5 || mapped.Names() != 4 {
		t.Fatalf("rows/names: %d/%d", mapped.Len(), mapped.Names())
	}
	row, ok := mapped.Row(12)
	if !ok {
		t.Fatal("missing stable ID")
	}
	got, ok := mapped.Path(row)
	if !ok || got != filepath.Join(s.Root, "one", "项目.txt") {
		t.Fatalf("path: %q %v", got, ok)
	}
	r := mapped.Record(row)
	if r.Size != 42 || r.Modified != 123 || r.Version != 9 || r.ParentID != 10 {
		t.Fatalf("record: %+v", r)
	}
	other, _ := mapped.Row(13)
	if mapped.NameIDs[row] != mapped.NameIDs[other] {
		t.Fatal("duplicate names were not interned")
	}
}

// TestInvalidSnapshotFails rejects corruption before mapped arrays reach readers.
func TestInvalidSnapshotFails(t *testing.T) {
	for _, offset := range []int64{0, 32, headerSize + 16} {
		t.Run(string(rune('a'+offset%26)), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "index.columns")
			if err := Write(path, fixture(t)); err != nil {
				t.Fatal(err)
			}
			file, err := os.OpenFile(path, os.O_RDWR, 0600)
			if err != nil {
				t.Fatal(err)
			}
			var data [1]byte
			if _, err := file.ReadAt(data[:], offset); err != nil {
				t.Fatal(err)
			}
			data[0] ^= 0xff
			if _, err := file.WriteAt(data[:], offset); err != nil {
				t.Fatal(err)
			}
			file.Close()
			if s, err := Open(path); err == nil {
				s.Close()
				t.Fatal("accepted corrupt file")
			}
		})
	}
}

// TestValidateMalformedReferences also protects callers building heap snapshots.
func TestValidateMalformedReferences(t *testing.T) {
	for name, corrupt := range map[string]func(*Snapshot){
		"cycle":             func(s *Snapshot) { a, _ := s.Row(10); b, _ := s.Row(11); s.Parents[a], s.Parents[b] = b, a },
		"posting-range":     func(s *Snapshot) { s.NameStarts[1] = 1000 },
		"name-range":        func(s *Snapshot) { s.NameOffsets[1] = 1000 },
		"posting-duplicate": func(s *Snapshot) { s.NameRows[1] = s.NameRows[0] },
		"file-parent":       func(s *Snapshot) { a, _ := s.Row(10); b, _ := s.Row(12); s.Parents[a] = b },
	} {
		t.Run(name, func(t *testing.T) {
			s := fixture(t)
			corrupt(s)
			if err := s.Validate(); err == nil {
				t.Fatal("accepted malformed columns")
			}
		})
	}
}

// TestVersionReplacement ensures a replay cannot undo a newer scan observation.
func TestVersionReplacement(t *testing.T) {
	b := NewBuilder(t.TempDir(), 1)
	for _, r := range []Record{{ID: 2, ParentID: 1, Name: "new", Version: 10}, {ID: 2, ParentID: 1, Name: "old", Version: 9}} {
		if err := b.Add(r); err != nil {
			t.Fatal(err)
		}
	}
	s, err := b.Finish()
	if err != nil {
		t.Fatal(err)
	}
	if s.Record(1).Name != "new" {
		t.Fatal("older event replaced new observation")
	}
	if err := b.Add(Record{}); err == nil {
		t.Fatal("finished builder accepted write")
	}
}
