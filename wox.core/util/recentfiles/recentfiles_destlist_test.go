package recentfiles

import (
	"encoding/binary"
	"testing"
	"time"
	"unicode/utf16"
)

func TestParseDestListVersion3ReadsPathAndLastUsed(t *testing.T) {
	path := `C:\Users\me\notes.txt`
	units := utf16.Encode([]rune(path))
	data := make([]byte, destListHeaderSize+destListEntryV2PathOffset+len(units)*2+destListEntryV2TrailingAlign)
	binary.LittleEndian.PutUint32(data[0:4], 3)
	binary.LittleEndian.PutUint32(data[4:8], 1)

	const filetime = windowsToUnixFiletimeEpochTicks + 10_000_000 // 1 second after Unix epoch
	binary.LittleEndian.PutUint64(data[destListHeaderSize+destListEntryFiletimeOffset:], filetime)
	binary.LittleEndian.PutUint32(data[destListHeaderSize+destListEntryPinStatusOffset:], 0xffffffff)
	binary.LittleEndian.PutUint16(data[destListHeaderSize+destListEntryV2PathSizeOffset:], uint16(len(units)))
	for i, unit := range units {
		binary.LittleEndian.PutUint16(data[destListHeaderSize+destListEntryV2PathOffset+i*2:], unit)
	}

	entries := parseDestList(data)
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if entries[0].Path != path {
		t.Fatalf("path = %q, want %q", entries[0].Path, path)
	}
	want := time.Unix(1, 0)
	if !entries[0].LastUsed.Equal(want) {
		t.Fatalf("last used = %s, want %s", entries[0].LastUsed, want)
	}
}

func TestParseDestListRejectsShortBuffer(t *testing.T) {
	if entries := parseDestList([]byte{1, 2, 3}); len(entries) != 0 {
		t.Fatalf("short destlist = %#v", entries)
	}
}
