package column

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"unsafe"
)

// encodeQueryColumns retains the original contiguous encoder as an independent
// format oracle for the segmented production writer.
func (s *Snapshot) encodeQueryColumns() []byte {
	parts := [][]byte{bytesOf(s.NameScoreHints), bytesOf(s.NameSecondScoreHints), bytesOf(s.NameBitmaps), bytesOf(s.NameRepeatBitmaps), bytesOf(s.NonASCIINameBitmap), bytesOf(s.FrequentNameIDs)}
	size := 8
	for _, part := range parts {
		size += (len(part) + 7) &^ 7
	}
	data := make([]byte, size)
	binary.LittleEndian.PutUint64(data, uint64(len(s.FrequentNameIDs)))
	offset := 8
	for _, part := range parts {
		copy(data[offset:], part)
		offset += (len(part) + 7) &^ 7
	}
	return data
}

// TestSegmentedQuerySectionMatchesEncoding covers internal padding when the
// frequent-name column has an odd element count, as well as empty columns.
func TestSegmentedQuerySectionMatchesEncoding(t *testing.T) {
	for _, frequentNames := range []int{0, 1, 2, 3} {
		t.Run(fmt.Sprint(frequentNames), func(t *testing.T) {
			b := NewBuilder(t.TempDir(), 1)
			for i := 0; i < frequentNames*16; i++ {
				if err := b.Add(Record{ID: uint64(i + 2), ParentID: 1, Name: fmt.Sprintf("name-%d", i/16)}); err != nil {
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
			descriptor := data[32+15*24:]
			start, length := binary.LittleEndian.Uint64(descriptor), binary.LittleEndian.Uint64(descriptor[8:])
			if !bytes.Equal(data[start:start+length], s.encodeQueryColumns()) {
				t.Fatal("segmented write changed query section bytes")
			}
			mapped, err := Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer mapped.Close()
			if len(mapped.FrequentNameIDs) != frequentNames {
				t.Fatalf("frequent names: %d", len(mapped.FrequentNameIDs))
			}
		})
	}
}

// TestQueryColumnsMappedAndTrimmed checks both ownership and post-eviction reads.
func TestQueryColumnsMappedAndTrimmed(t *testing.T) {
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
	arrays := [][]byte{bytesOf(mapped.NameScoreHints), bytesOf(mapped.NameSecondScoreHints), bytesOf(mapped.NameBitmaps), bytesOf(mapped.NameRepeatBitmaps), bytesOf(mapped.NonASCIINameBitmap), bytesOf(mapped.NameRows)}
	start := uintptr(unsafe.Pointer(&mapped.mapping.data[0]))
	for _, array := range arrays {
		address := uintptr(unsafe.Pointer(&array[0]))
		if address < start || address+uintptr(len(array)) > start+uintptr(len(mapped.mapping.data)) {
			t.Fatal("query array is outside the mapping")
		}
	}
	if !bytes.Equal(s.encodeQueryColumns(), mapped.encodeQueryColumns()) {
		t.Fatal("query columns changed on disk")
	}
	first := &mapped.NameScoreHints[0]
	mapped.PrepareSearchHints()
	if first != &mapped.NameScoreHints[0] {
		t.Fatal("mapped query columns were rebuilt on the heap")
	}
	if err := mapped.mapping.releasePages(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(s.encodeQueryColumns(), mapped.encodeQueryColumns()) {
		t.Fatal("eviction changed query columns")
	}
}

// TestDiscardedDevelopmentSortColumnsRemainReadable keeps developer snapshots
// usable after removing the metadata-sort experiment, without rebuilding arrays.
func TestDiscardedDevelopmentSortColumnsRemainReadable(t *testing.T) {
	s := fixture(t)
	path := filepath.Join(t.TempDir(), "development.columns")
	if err := Write(path, s); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	descriptor := data[32+15*24:]
	start, length := binary.LittleEndian.Uint64(descriptor), binary.LittleEndian.Uint64(descriptor[8:])
	extra := 2 * ((s.Len()*4 + 7) &^ 7)
	data = append(data, make([]byte, extra)...)
	// append may move the backing array; retain the descriptor in the new one.
	descriptor = data[32+15*24:]
	binary.LittleEndian.PutUint64(descriptor[8:], length+uint64(extra))
	binary.LittleEndian.PutUint32(descriptor[16:], crc32.ChecksumIEEE(data[start:]))
	binary.LittleEndian.PutUint32(data[8:], 4)
	binary.LittleEndian.PutUint64(data[16:], uint64(len(data)))
	binary.LittleEndian.PutUint32(data[24:], crc32.ChecksumIEEE(data[32:headerSize]))
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	mapped, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer mapped.Close()
	if !bytes.Equal(s.encodeQueryColumns(), mapped.encodeQueryColumns()) {
		t.Fatal("filename query columns changed")
	}
	newPath := filepath.Join(t.TempDir(), "current.columns")
	if err := Write(newPath, mapped); err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatal(err)
	}
	if binary.LittleEndian.Uint32(current[8:]) != 3 {
		t.Fatal("new snapshot retained discarded sort format")
	}
}

// TestIdleTrimConcurrentReaders retains borrowed strings through eviction and shutdown.
func TestIdleTrimConcurrentReaders(t *testing.T) {
	store, _ := fixtureStore(t)
	var workers sync.WaitGroup
	for range 4 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for range 30 {
				if err := store.Read(func(v View) error {
					row, ok := v.Base.Row(12)
					if !ok || v.Base.Record(row).Name != "项目.txt" {
						t.Error("mapped record changed during idle trim")
					}
					return nil
				}); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	for range 30 {
		if err := store.ReleaseIdleMemory(); err != nil {
			t.Fatal(err)
		}
	}
	workers.Wait()
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.ReleaseIdleMemory(); err != nil {
		t.Fatal(err)
	}
}

// TestUpgradeQueryColumnsPreservesFacts proves old-format reads remain write-free
// and the explicit migration is idempotent without changing journal ownership.
func TestUpgradeQueryColumnsPreservesFacts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.columns")
	if err := Write(path, fixture(t)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	descriptor := data[32+15*24:]
	size := binary.LittleEndian.Uint64(descriptor)
	binary.LittleEndian.PutUint64(descriptor[8:], 0)
	binary.LittleEndian.PutUint32(descriptor[16:], 0)
	binary.LittleEndian.PutUint32(data[8:], 1)
	binary.LittleEndian.PutUint64(data[16:], size)
	binary.LittleEndian.PutUint32(data[24:], crc32.ChecksumIEEE(data[32:headerSize]))
	data = data[:size]
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	old, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	want := append([]byte(nil), old.encodeQueryColumns()...)
	old.Close()
	unchanged, _ := os.ReadFile(path)
	if !bytes.Equal(data, unchanged) {
		t.Fatal("compatibility read rewrote the file")
	}
	for range 2 {
		if err := UpgradeQueryColumns(path); err != nil {
			t.Fatal(err)
		}
		upgraded, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(want, upgraded.encodeQueryColumns()) || upgraded.Len() != 5 {
			t.Fatal("migration changed query data or records")
		}
		upgraded.Close()
	}
}
