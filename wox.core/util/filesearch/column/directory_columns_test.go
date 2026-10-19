package column

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"unicode/utf8"
	"unsafe"
)

// TestDirectoryColumnsLegacyReadAndMapping covers write-free old-section reads,
// borrowed ownership, orphan ancestry and page eviction in fresh publications.
func TestDirectoryColumnsLegacyReadAndMapping(t *testing.T) {
	b := NewBuilder(t.TempDir(), 1)
	for _, record := range []Record{{ID: 2, ParentID: 1, Name: "Café", Flags: Directory}, {ID: 3, ParentID: 2, Name: "Report.md"}, {ID: 4, ParentID: 999, Name: "orphan", Flags: Directory}} {
		if err := b.Add(record); err != nil {
			t.Fatal(err)
		}
	}
	s, err := b.Finish()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "index.columns")
	if err := Write(path, s); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	descriptor := data[32+directoryColumnSection*24:]
	start := binary.LittleEndian.Uint64(descriptor)
	clear(descriptor[:24])
	data = data[:start]
	binary.LittleEndian.PutUint32(data[12:], directoryColumnSection)
	binary.LittleEndian.PutUint64(data[16:], uint64(len(data)))
	binary.LittleEndian.PutUint32(data[24:], crc32.ChecksumIEEE(data[32:headerSize]))
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	legacy, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	prepared := filepath.Join(t.TempDir(), "prepared.columns")
	if err := Write(prepared, legacy); err != nil {
		t.Fatal(err)
	}
	legacy.Close()
	unchanged, _ := os.ReadFile(path)
	if !bytes.Equal(data, unchanged) {
		t.Fatal("legacy read wrote the snapshot")
	}
	for range 2 {
		mapped, err := Open(prepared)
		if err != nil {
			t.Fatal(err)
		}
		for row, flags := range mapped.Flags {
			if flags&Directory == 0 {
				continue
			}
			id := mapped.RowDirectory[row]
			want, _ := mapped.Path(uint32(row))
			if mapped.DirectoryPath(id) != want || mapped.DirectoryRuneCounts[id] != uint32(utf8.RuneCountInString(mapped.DirectoryKey(id))) {
				t.Fatal("directory path changed")
			}
			if !reflect.DeepEqual(s.Children(uint32(row)), mapped.Children(uint32(row))) {
				t.Fatal("children changed")
			}
		}
		for _, array := range [][]byte{mapped.DirectoryBytes, mapped.DirectoryKeys, bytesOf(mapped.DirectoryRows), bytesOf(mapped.RowDirectory), bytesOf(mapped.DirectoryStarts), bytesOf(mapped.DirectoryBitmaps), bytesOf(mapped.DirectoryPairBitmaps), bytesOf(mapped.DirectoryRuneCounts)} {
			base := uintptr(unsafe.Pointer(&mapped.mapping.data[0]))
			address := uintptr(unsafe.Pointer(&array[0]))
			if address < base || address+uintptr(len(array)) > base+uintptr(len(mapped.mapping.data)) {
				t.Fatal("directory array is not mapped")
			}
		}
		if err := mapped.mapping.releasePages(); err != nil {
			t.Fatal(err)
		}
		if mapped.DirectoryPath(mapped.RowDirectory[1]) == "" {
			t.Fatal("eviction lost the path")
		}
		mapped.Close()
	}
}

// TestQueryOverlayMetadataAndMoves verifies that path invalidation follows
// actual ancestry changes while metadata edits preserve borrowed directory keys.
func TestQueryOverlayMetadataAndMoves(t *testing.T) {
	s, _ := fixtureStore(t)
	defer s.Close()
	if err := s.Apply([]Change{{Record: Record{ID: 10, ParentID: 5, Name: "one", Flags: Directory | ModifiedKnown, Modified: 42}}}, "metadata"); err != nil {
		t.Fatal(err)
	}
	if err := s.ReadQuery(func(v View) error {
		row, _ := v.Base.Row(10)
		if !v.DirectoryUnchanged(row) || len(v.Query.StaleDirectories) != 0 {
			t.Fatal("metadata invalidated directory paths")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply([]Change{{Record: Record{ID: 10, ParentID: 5, Name: "moved", Flags: Directory}}}, "move"); err != nil {
		t.Fatal(err)
	}
	if err := s.ReadQuery(func(v View) error {
		row, _ := v.Base.Row(10)
		if v.DirectoryUnchanged(row) || !v.Query.PathsCached || len(v.Query.LiveDirectories) != 1 {
			t.Fatal("move did not refresh directory paths")
		}
		if filepath.Base(v.Query.LiveDirectories[0].Path) != "moved" {
			t.Fatal("cached old directory name")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
