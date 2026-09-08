package recentfiles

import (
	"encoding/binary"
	"strings"
	"time"
	"unicode/utf16"
)

const (
	destListHeaderSize              = 32
	destListEntryFiletimeOffset     = 100
	destListEntryPinStatusOffset    = 108
	destListEntryV1PathSizeOffset   = 112
	destListEntryV1PathOffset       = 114
	destListEntryV2PathSizeOffset   = 128
	destListEntryV2PathOffset       = 130
	destListEntryV2TrailingAlign    = 4
	windowsToUnixFiletimeEpochTicks = 116444736000000000
)

type destListEntry struct {
	Path     string
	LastUsed time.Time
}

// parseDestList reads AutomaticDestinations DestList paths and last-access times.
func parseDestList(data []byte) []destListEntry {
	if len(data) < destListHeaderSize {
		return nil
	}

	version := binary.LittleEndian.Uint32(data[0:4])
	extended := version >= 2
	entries := make([]destListEntry, 0)
	offset := destListHeaderSize
	for {
		entry, next, ok := parseDestListEntry(data, offset, extended)
		if !ok {
			break
		}
		if strings.TrimSpace(entry.Path) != "" {
			entries = append(entries, entry)
		}
		if next <= offset {
			break
		}
		offset = next
	}
	return entries
}

// parseDestListEntry decodes one DestList row and returns the next entry offset.
func parseDestListEntry(data []byte, offset int, extended bool) (destListEntry, int, bool) {
	pathSizeOffset := destListEntryV1PathSizeOffset
	pathOffset := destListEntryV1PathOffset
	trailing := 0
	if extended {
		pathSizeOffset = destListEntryV2PathSizeOffset
		pathOffset = destListEntryV2PathOffset
		trailing = destListEntryV2TrailingAlign
	}
	if offset < 0 || offset+pathOffset+2 > len(data) || offset+destListEntryFiletimeOffset+8 > len(data) {
		return destListEntry{}, offset, false
	}

	pathChars := int(binary.LittleEndian.Uint16(data[offset+pathSizeOffset : offset+pathSizeOffset+2]))
	pathBytes := pathChars * 2
	pathStart := offset + pathOffset
	pathEnd := pathStart + pathBytes
	if pathChars < 0 || pathEnd > len(data) {
		return destListEntry{}, offset, false
	}

	units := make([]uint16, pathChars)
	for i := 0; i < pathChars; i++ {
		units[i] = binary.LittleEndian.Uint16(data[pathStart+i*2 : pathStart+i*2+2])
	}
	path := strings.TrimRight(string(utf16.Decode(units)), "\x00")
	next := pathEnd + trailing
	if next > len(data) {
		next = len(data)
	}

	filetime := binary.LittleEndian.Uint64(data[offset+destListEntryFiletimeOffset : offset+destListEntryFiletimeOffset+8])
	return destListEntry{
		Path:     strings.TrimSpace(path),
		LastUsed: filetimeToTime(filetime),
	}, next, true
}

func filetimeToTime(filetime uint64) time.Time {
	if filetime < windowsToUnixFiletimeEpochTicks {
		return time.Time{}
	}
	return time.Unix(0, int64(filetime-windowsToUnixFiletimeEpochTicks)*100)
}
