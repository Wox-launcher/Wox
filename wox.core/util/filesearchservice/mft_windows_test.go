//go:build windows

package filesearchservice

import (
	"encoding/binary"
	"testing"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

func TestParseUSNRecords(t *testing.T) {
	buffer := make([]byte, 8)
	binary.LittleEndian.PutUint64(buffer, 99)
	buffer = append(buffer, testUSNRecord(5, 5, "", true)...)
	buffer = append(buffer, testUSNRecord(10, 5, "Users", true)...)
	buffer = append(buffer, testUSNRecord(11, 10, "notes.txt", false)...)
	buffer = append(buffer, testUSNRecord(12, 5, "Windows", true)...)
	buffer = append(buffer, testUSNRecord(13, 12, "system.ini", false)...)

	next, nodes, err := parseUSNRecords(buffer)
	if err != nil {
		t.Fatal(err)
	}
	if next != 99 || len(nodes) != 5 {
		t.Fatalf("next=%d nodes=%d", next, len(nodes))
	}
	if nodes[2].Reference != 11 || nodes[2].ParentReference != 10 || nodes[2].Name != "notes.txt" || nodes[2].IsDir {
		t.Fatalf("decoded file=%+v", nodes[2])
	}
	if !nodes[1].IsDir || nodes[1].Name != "Users" {
		t.Fatalf("decoded directory=%+v", nodes[1])
	}
}
func TestParseUSNRecordsDoesNotAllocateEmptyBatch(t *testing.T) {
	buffer := make([]byte, 8)
	binary.LittleEndian.PutUint64(buffer, 99)

	next, nodes, err := parseUSNRecords(buffer)
	if err != nil {
		t.Fatal(err)
	}
	if next != 99 || nodes != nil {
		t.Fatalf("next=%d nodes=%v", next, nodes)
	}
}

func testUSNRecord(reference uint64, parent uint64, name string, isDir bool) []byte {
	nameWords := utf16.Encode([]rune(name))
	record := make([]byte, 60+len(nameWords)*2)
	binary.LittleEndian.PutUint32(record, uint32(len(record)))
	binary.LittleEndian.PutUint16(record[4:], 2)
	binary.LittleEndian.PutUint64(record[8:], reference)
	binary.LittleEndian.PutUint64(record[16:], parent)
	if isDir {
		binary.LittleEndian.PutUint32(record[52:], windows.FILE_ATTRIBUTE_DIRECTORY)
	}
	binary.LittleEndian.PutUint16(record[56:], uint16(len(nameWords)*2))
	binary.LittleEndian.PutUint16(record[58:], 60)
	for index, word := range nameWords {
		binary.LittleEndian.PutUint16(record[60+index*2:], word)
	}
	return record
}
