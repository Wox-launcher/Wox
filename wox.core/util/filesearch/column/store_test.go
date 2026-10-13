package column

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func fixtureStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Publish(fixture(t), "scan:9", "policy:1", s.Revision()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, dir
}

// TestJournalRecovery pins cursor advancement to complete, durable batches.
func TestJournalRecovery(t *testing.T) {
	s, dir := fixtureStore(t)
	if err := s.Apply([]Change{{Record: Record{ID: 10, ParentID: 5, Name: "renamed", Flags: Directory, Version: 10}}}, "scan:10"); err != nil {
		t.Fatal(err)
	}
	logPath := s.log.Name()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	partial, err := encodeBatch([]Change{{Record: Record{ID: 12, ParentID: 10, Name: "should-not-appear", Version: 11}}}, "scan:11")
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(partial[:len(partial)-3]); err != nil {
		t.Fatal(err)
	}
	file.Close()
	restored, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if err := restored.Read(func(v View) error {
		if v.Cursor != "scan:10" {
			t.Fatalf("cursor advanced past committed batch: %s", v.Cursor)
		}
		path, ok := v.Path(12)
		if !ok || path != filepath.Join(v.Base.Root, "renamed", "项目.txt") {
			t.Fatalf("restored path: %s, %v", path, ok)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := restored.Apply(nil, "scan:12"); err != nil {
		t.Fatal(err)
	}
	if err := restored.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	if err := again.Read(func(v View) error {
		if v.Cursor != "scan:12" {
			t.Fatal("did not truncate incomplete tail before append")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// TestCompactionPrunesDeletedSubtrees verifies rename/delete semantics survive publication.
func TestCompactionPrunesDeletedSubtrees(t *testing.T) {
	s, dir := fixtureStore(t)
	if err := s.Apply([]Change{{Record: Record{ID: 10, Version: 20}, Deleted: true}}, "scan:20"); err != nil {
		t.Fatal(err)
	}
	if err := s.Read(func(v View) error {
		if _, ok := v.Path(12); ok {
			t.Fatal("deleted directory left child visible")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Compact(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.Read(func(v View) error {
		if v.Base.Len() != 3 || len(v.Delta) != 0 {
			t.Fatalf("compacted rows/delta: %d/%d", v.Base.Len(), len(v.Delta))
		}
		if _, ok := v.Path(13); !ok {
			t.Fatal("unrelated directory lost")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	restored, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if err := restored.Read(func(v View) error {
		if v.Cursor != "scan:20" || v.Policy != "policy:1" {
			t.Fatal("lost generation metadata")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// TestStalePublicationCannotDiscardDelta protects concurrent rebuild completion.
func TestStalePublicationCannotDiscardDelta(t *testing.T) {
	s, _ := fixtureStore(t)
	before := s.Revision()
	if err := s.Apply([]Change{{Record: Record{ID: 99, ParentID: 5, Name: "new.txt"}}}, "later"); err != nil {
		t.Fatal(err)
	}
	if err := s.Publish(fixture(t), "earlier", "policy:1", before); !errors.Is(err, ErrStaleBuild) {
		t.Fatalf("publication: %v", err)
	}
	if err := s.Read(func(v View) error {
		if _, ok := v.Path(99); !ok {
			t.Fatal("stale build discarded new file")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// TestInvalidDeltaIsNotJournaled checks validation happens before any durable write.
func TestInvalidDeltaIsNotJournaled(t *testing.T) {
	s, _ := fixtureStore(t)
	before, err := s.log.Stat()
	if err != nil {
		t.Fatal(err)
	}
	for _, changes := range [][]Change{
		{{Record: Record{ID: 10, ParentID: 11, Name: "one", Flags: Directory}}, {Record: Record{ID: 11, ParentID: 10, Name: "two", Flags: Directory}}},
		{{Record: Record{ID: 50, ParentID: 12, Name: "under-file"}}},
		{{Record: Record{ID: 50, ParentID: 5, Name: "../escape"}}},
	} {
		if err := s.Apply(changes, "invalid"); err == nil {
			t.Fatal("accepted malformed changes")
		}
	}
	after, err := s.log.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if before.Size() != after.Size() {
		t.Fatal("invalid batch reached journal")
	}
}

// FuzzJournalDecoder must reject hostile lengths without allocating an unbounded buffer.
func FuzzJournalDecoder(f *testing.F) {
	seed, err := encodeBatch([]Change{{Record: Record{ID: 2, ParentID: 1, Name: "文件.txt", Version: 20}}}, "cursor")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add([]byte{255, 255, 255, 255, 0, 0, 0, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		changes, cursor, _, err := readBatch(bytes.NewReader(data))
		if err != nil {
			return
		}
		encoded, err := encodeBatch(changes, cursor)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := readBatch(bytes.NewReader(encoded)); err != nil && err != io.EOF {
			t.Fatal(err)
		}
	})
}
