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
	return testUSNRecordWithReason(reference, parent, name, isDir, 0)
}

func testUSNRecordWithReason(reference uint64, parent uint64, name string, isDir bool, reason uint32) []byte {
	nameWords := utf16.Encode([]rune(name))
	record := make([]byte, 60+len(nameWords)*2)
	binary.LittleEndian.PutUint32(record, uint32(len(record)))
	binary.LittleEndian.PutUint16(record[4:], 2)
	binary.LittleEndian.PutUint64(record[8:], reference)
	binary.LittleEndian.PutUint64(record[16:], parent)
	binary.LittleEndian.PutUint32(record[40:], reason)
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

func TestShouldEmitLiveUSNReason(t *testing.T) {
	cases := []struct {
		name   string
		reason uint32
		want   bool
	}{
		{"create", usnReasonFileCreate, true},
		{"delete", usnReasonFileDelete, true},
		{"rename_old", usnReasonRenameOldName, true},
		{"rename_new", usnReasonRenameNewName, true},
		{"hard_link", usnReasonHardLinkChange, true},
		{"data_extend_open", usnReasonDataExtend, false},
		{"data_overwrite_open", usnReasonDataOverwrite, false},
		{"data_truncate_open", usnReasonDataTruncation, false},
		{"data_extend_close", usnReasonDataExtend | usnReasonClose, true},
		{"data_overwrite_close", usnReasonDataOverwrite | usnReasonClose, true},
		{"close_only", usnReasonClose, false},
		{"basic_info_close", 0x00008000 | usnReasonClose, false},
	}
	for _, tc := range cases {
		if got := shouldEmitLiveUSNReason(tc.reason); got != tc.want {
			t.Errorf("%s (0x%x): got %t, want %t", tc.name, tc.reason, got, tc.want)
		}
	}
}

func TestParseUSNRecordsIntoReusesScratchAndFilters(t *testing.T) {
	buffer := make([]byte, 8)
	binary.LittleEndian.PutUint64(buffer, 123)
	buffer = append(buffer, testUSNRecordWithReason(10, 5, "streaming.log", false, usnReasonDataExtend)...)
	buffer = append(buffer, testUSNRecordWithReason(11, 5, "created.txt", false, usnReasonFileCreate)...)
	buffer = append(buffer, testUSNRecordWithReason(12, 5, "saved.txt", false, usnReasonDataOverwrite|usnReasonClose)...)

	nodes := make([]mftNode, 0, 8)
	nameWords := make([]uint16, 0, 32)
	var err error
	var next uint64
	for range 2 {
		next, nodes, nameWords, err = parseUSNRecordsInto(buffer, nodes, nameWords, shouldEmitLiveUSNReason)
		if err != nil {
			t.Fatal(err)
		}
		if next != 123 || len(nodes) != 2 {
			t.Fatalf("next=%d nodes=%+v", next, nodes)
		}
		if nodes[0].Name != "created.txt" || nodes[1].Name != "saved.txt" {
			t.Fatalf("unexpected filtered nodes=%+v", nodes)
		}
		if cap(nodes) != 8 {
			t.Fatalf("nodes slice reallocated: cap=%d", cap(nodes))
		}
	}
}
