package column

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
)

// TestWrittenSnapshotRoundTrip compares publication's mapped view to a normal
// reopen, including every persisted byte and the independent structural check.
func TestWrittenSnapshotRoundTrip(t *testing.T) {
	s := fixture(t)
	path := filepath.Join(t.TempDir(), "published.columns")
	mapped, err := writeAndOpen(path, s)
	if err != nil {
		t.Fatal(err)
	}
	defer mapped.Close()
	if err := mapped.Validate(); err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(t.TempDir(), "copy.columns")
	if err := Write(copyPath, mapped); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(copyPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, got) {
		t.Fatal("publication changed snapshot bytes")
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	for row := range s.IDs {
		if mapped.Record(uint32(row)) != reopened.Record(uint32(row)) {
			t.Fatalf("record %d changed across reopen", row)
		}
	}
}

// TestWrittenSnapshotRejectsChangedSections covers both mapped name columns
// and buffer-verified entry columns, rather than trusting the file after Write.
func TestWrittenSnapshotRejectsChangedSections(t *testing.T) {
	for section := 0; section < sectionCount; section++ {
		t.Run(fmt.Sprint(section), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "index.columns")
			header, err := writeSnapshot(path, fixture(t))
			if err != nil {
				t.Fatal(err)
			}
			descriptor := header[32+section*24:]
			start, length := binary.LittleEndian.Uint64(descriptor), binary.LittleEndian.Uint64(descriptor[8:])
			if length == 0 {
				t.Fatal("fixture must populate each section")
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			data[start] ^= 1
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			if s, err := openSnapshot(path, header); err == nil {
				s.Close()
				t.Fatal("changed section accepted")
			}
		})
	}
}

// TestWrittenSnapshotRejectsForgedHeader ensures checksums read from disk
// cannot substitute for the writer's trusted header or structural validation.
func TestWrittenSnapshotRejectsForgedHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.columns")
	header, err := writeSnapshot(path, fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	descriptor := data[32+2*24:] // Parent rows.
	start, length := binary.LittleEndian.Uint64(descriptor), binary.LittleEndian.Uint64(descriptor[8:])
	binary.LittleEndian.PutUint32(data[start+4:], 1) // A self-parent cycle.
	binary.LittleEndian.PutUint32(descriptor[16:], crc32.ChecksumIEEE(data[start:start+length]))
	binary.LittleEndian.PutUint32(data[24:], crc32.ChecksumIEEE(data[32:headerSize]))
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, trusted := range [][]byte{header, nil} {
		if s, err := openSnapshot(path, trusted); err == nil {
			s.Close()
			t.Fatal("forged invalid snapshot accepted")
		}
	}
}

// TestWrittenSnapshotRejectsInvalidInput retains validation of heap records and
// query-column IDs even when publication avoids validating mapped rows twice.
func TestWrittenSnapshotRejectsInvalidInput(t *testing.T) {
	for _, queryColumns := range []bool{false, true} {
		s := fixture(t)
		if queryColumns {
			s.FrequentNameIDs = []uint32{uint32(s.Names())}
		} else {
			s.Parents[1] = 1
		}
		path := filepath.Join(t.TempDir(), "invalid.columns")
		if mapped, err := writeAndOpen(path, s); err == nil {
			mapped.Close()
			t.Fatal("invalid input accepted")
		}
	}
}
