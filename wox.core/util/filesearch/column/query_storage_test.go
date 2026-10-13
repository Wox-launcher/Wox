package column

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"unsafe"
)

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
