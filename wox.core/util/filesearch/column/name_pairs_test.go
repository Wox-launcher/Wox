package column

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
	"unsafe"
)

// TestNamePairsMappedAndLegacyRead verifies read-only ownership, necessary
// ordering across gaps, and the write-free fallback for previous snapshots.
func TestNamePairsMappedAndLegacyRead(t *testing.T) {
	b := NewBuilder(t.TempDir(), 1)
	for i, name := range []string{"maibn.go", "main.go", "m-a-i-b-n.go", "Café.md", "重庆.md"} {
		if err := b.Add(Record{ID: uint64(i + 2), ParentID: 1, Name: name}); err != nil {
			t.Fatal(err)
		}
	}
	s, err := b.Finish()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "pairs.columns")
	if err := Write(path, s); err != nil {
		t.Fatal(err)
	}
	mapped, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	dataStart := uintptr(unsafe.Pointer(&mapped.mapping.data[0]))
	arrayStart := uintptr(unsafe.Pointer(&mapped.NamePairBitmaps[0]))
	if arrayStart < dataStart || arrayStart+uintptr(len(mapped.NamePairBitmaps)*8) > dataStart+uintptr(len(mapped.mapping.data)) {
		t.Fatal("name pairs are not mapped")
	}
	words := (mapped.Names() + 63) / 64
	pairs := NamePairClasses("maibn.go", nil)
	for row := 1; row <= 3; row++ {
		id := mapped.NameIDs[row]
		admitted := true
		for _, pair := range pairs {
			admitted = admitted && mapped.NamePairBitmaps[pair*words+int(id)/64]&(1<<(id%64)) != 0
		}
		if admitted != (row != 2) {
			t.Fatalf("unexpected order filter for %q", mapped.Name(id))
		}
	}
	mapped.Close()
	data, _ := os.ReadFile(path)
	descriptor := data[32+namePairSection*24:]
	start := binary.LittleEndian.Uint64(descriptor)
	clear(descriptor[:24])
	data = data[:start]
	binary.LittleEndian.PutUint32(data[12:], namePairSection)
	binary.LittleEndian.PutUint64(data[16:], uint64(len(data)))
	binary.LittleEndian.PutUint32(data[24:], crc32.ChecksumIEEE(data[32:headerSize]))
	legacyPath := filepath.Join(t.TempDir(), "legacy.columns")
	if err := os.WriteFile(legacyPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	legacy, err := Open(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(legacy.NamePairBitmaps) != 0 {
		t.Fatal("legacy read allocated pair columns")
	}
	legacy.Close()
	unchanged, err := os.ReadFile(legacyPath)
	if err != nil || !bytes.Equal(data, unchanged) {
		t.Fatal("legacy read modified the snapshot")
	}

	// Verify 28-class digit ordering ("dsc2" vs "2dsc") and legacy 27-class section compatibility.
	db := NewBuilder(t.TempDir(), 1)
	for i, name := range []string{"docx.scale-200.png", "200_dsc.png"} {
		if err := db.Add(Record{ID: uint64(i + 2), ParentID: 1, Name: name}); err != nil {
			t.Fatal(err)
		}
	}
	ds, err := db.Finish()
	if err != nil {
		t.Fatal(err)
	}
	digitPath := filepath.Join(t.TempDir(), "digits.columns")
	if err := Write(digitPath, ds); err != nil {
		t.Fatal(err)
	}
	digitMapped, err := Open(digitPath)
	if err != nil {
		t.Fatal(err)
	}
	if digitMapped.NamePairStride != NamePairAlphabet {
		t.Fatalf("expected stride %d, got %d", NamePairAlphabet, digitMapped.NamePairStride)
	}
	dwords := (digitMapped.Names() + 63) / 64
	dpairs := NamePairClasses("dsc2", nil)
	for row := 1; row <= 2; row++ {
		id := digitMapped.NameIDs[row]
		admitted := true
		for _, pair := range dpairs {
			admitted = admitted && digitMapped.NamePairBitmaps[pair*dwords+int(id)/64]&(1<<(id%64)) != 0
		}
		if admitted != (row == 1) {
			t.Fatalf("unexpected digit order filter for %q: admitted=%v", digitMapped.Name(id), admitted)
		}
	}
	digitMapped.Close()

	// Rewrite the namePairSection with a 27-class payload to verify legacy 27-class compatibility.
	rawBytes, err := os.ReadFile(digitPath)
	if err != nil {
		t.Fatal(err)
	}
	npDesc := rawBytes[32+namePairSection*24:]
	npStart := binary.LittleEndian.Uint64(npDesc)
	legacyPayload := make([]byte, legacyNamePairAlphabet*legacyNamePairAlphabet*dwords*8)
	rawBytes = append(rawBytes[:npStart], legacyPayload...)
	npDesc = rawBytes[32+namePairSection*24:]
	binary.LittleEndian.PutUint64(npDesc[8:], uint64(len(legacyPayload)))
	binary.LittleEndian.PutUint32(npDesc[16:], crc32.ChecksumIEEE(legacyPayload))
	binary.LittleEndian.PutUint64(rawBytes[16:], uint64(len(rawBytes)))
	binary.LittleEndian.PutUint32(rawBytes[24:], crc32.ChecksumIEEE(rawBytes[32:headerSize]))
	legacy27Path := filepath.Join(t.TempDir(), "legacy27.columns")
	if err := os.WriteFile(legacy27Path, rawBytes, 0600); err != nil {
		t.Fatal(err)
	}
	legacy27, err := Open(legacy27Path)
	if err != nil {
		t.Fatal(err)
	}
	if legacy27.NamePairStride != legacyNamePairAlphabet {
		t.Fatalf("expected legacy stride %d, got %d", legacyNamePairAlphabet, legacy27.NamePairStride)
	}
	legacy27.Close()
}
