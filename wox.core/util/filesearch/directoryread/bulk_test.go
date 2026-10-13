package directoryread

import (
	"encoding/binary"
	"os"
	"testing"
)

// bulkFixture mirrors Darwin's packed attribute order rather than Go alignment.
func bulkFixture(name string, kind uint32) []byte {
	length := (bulkFixedBytes + len(name) + 1 + 7) &^ 7
	buffer := make([]byte, length)
	binary.LittleEndian.PutUint32(buffer[0:4], uint32(length))
	binary.LittleEndian.PutUint32(buffer[4:8], bulkName|bulkReturned|bulkObjectType|bulkModified|bulkAccess|bulkError)
	binary.LittleEndian.PutUint32(buffer[16:20], bulkDataLength)
	binary.LittleEndian.PutUint32(buffer[28:32], bulkFixedBytes-28)
	binary.LittleEndian.PutUint32(buffer[32:36], uint32(len(name)+1))
	binary.LittleEndian.PutUint32(buffer[36:40], kind)
	binary.LittleEndian.PutUint64(buffer[40:48], 12345)
	binary.LittleEndian.PutUint64(buffer[48:56], 123)
	binary.LittleEndian.PutUint32(buffer[56:60], 0640)
	binary.LittleEndian.PutUint64(buffer[60:68], 42)
	copy(buffer[bulkFixedBytes:], name)
	return buffer
}

func TestBulkPackedMetadataAndLinkType(t *testing.T) {
	buffer := append(bulkFixture("项目.txt", 1), bulkFixture("link", 5)...)
	entries, err := decodeBulkEntries(t.TempDir(), buffer, 2)
	if err != nil {
		t.Fatal(err)
	}
	info, err := entries[0].Info()
	if err != nil || info.Name() != "项目.txt" || info.Size() != 42 || info.Mode() != 0640 || info.ModTime().Unix() != 12345 || info.ModTime().Nanosecond() != 123 {
		t.Fatalf("info=%+v err=%v", info, err)
	}
	info, err = entries[1].Info()
	if err != nil || entries[1].Type() != os.ModeSymlink || info.IsDir() {
		t.Fatalf("link followed target: %+v %v", info, err)
	}
	clear(buffer)
	if entries[0].Name() != "项目.txt" {
		t.Fatal("entry retained reusable buffer")
	}
}

func TestBulkMalformedRecordsAreErrors(t *testing.T) {
	for _, mutate := range []func([]byte){
		func(b []byte) { binary.LittleEndian.PutUint32(b, uint32(len(b)+1)) },
		func(b []byte) { binary.LittleEndian.PutUint32(b[28:32], 0xffffffff) },
		func(b []byte) { binary.LittleEndian.PutUint32(b[32:36], 0xffffffff) },
		func(b []byte) { binary.LittleEndian.PutUint32(b[4:8], 0) },
		func(b []byte) { b[bulkFixedBytes+1] = 0 },
		func(b []byte) { b[bulkFixedBytes] = '/' },
	} {
		buffer := bulkFixture("file", 1)
		mutate(buffer)
		if _, err := decodeBulkEntries("root", buffer, 1); err == nil {
			t.Fatal("accepted malformed native entry")
		}
	}
	if _, err := decodeBulkEntries("root", bulkFixture("file", 1), 2); err == nil {
		t.Fatal("accepted missing entry")
	}
}

func TestBulkMissingMetadataUsesLstat(t *testing.T) {
	buffer := bulkFixture("file", 1)
	binary.LittleEndian.PutUint32(buffer[16:20], 0)
	entries, err := decodeBulkEntries(t.TempDir(), buffer, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entries[0].Info(); !os.IsNotExist(err) {
		t.Fatalf("missing size was treated as valid metadata: %v", err)
	}
}

func FuzzBulkAttributes(f *testing.F) {
	f.Add(bulkFixture("项目.txt", 1), uint8(1))
	f.Fuzz(func(t *testing.T, data []byte, count uint8) { _, _ = decodeBulkEntries("root", data, int(count)) })
}
