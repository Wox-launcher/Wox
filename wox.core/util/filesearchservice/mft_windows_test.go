//go:build windows

package filesearchservice

import (
	"encoding/binary"
	"testing"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

func TestParseAndResolveUSNRecords(t *testing.T) {
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
	nodeMap := make(map[uint64]mftNode, len(nodes))
	for _, node := range nodes {
		nodeMap[node.Reference] = node
	}
	entries, err := resolveMFTEntries(`C:\Users`, `C:\`, nodeMap)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[1].Path != `C:\Users\notes.txt` || entries[1].IsDir {
		t.Fatalf("entries=%+v", entries)
	}
	multiEntries, err := resolveMFTEntriesForRoots([]string{`C:\Users`, `C:\Windows`}, `C:\`, nodeMap)
	if err != nil {
		t.Fatal(err)
	}
	if len(multiEntries) != 4 || multiEntries[3].Path != `C:\Windows\system.ini` {
		t.Fatalf("multi entries=%+v", multiEntries)
	}
	delete(nodeMap, 5)
	entriesWithoutRootRecord, err := resolveMFTEntries(`C:\Users`, `C:\`, nodeMap)
	if err != nil {
		t.Fatal(err)
	}
	if len(entriesWithoutRootRecord) != 2 || entriesWithoutRootRecord[1].Path != `C:\Users\notes.txt` {
		t.Fatalf("entries without root record=%+v", entriesWithoutRootRecord)
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
