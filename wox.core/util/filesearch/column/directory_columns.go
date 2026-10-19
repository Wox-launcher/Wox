package column

import (
	"encoding/binary"
	"fmt"
	"math/bits"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
	"unsafe"

	"wox/util/fuzzymatch"
)

// DirectoryPath borrows a display path; an empty path denotes orphan ancestry.
func (s *Snapshot) DirectoryPath(id uint32) string {
	return borrowDirectoryString(s.DirectoryBytes, s.DirectoryOffsets, id)
}

// DirectoryKey borrows the normalized path used by the path scorer.
func (s *Snapshot) DirectoryKey(id uint32) string {
	return borrowDirectoryString(s.DirectoryKeys, s.DirectoryKeyOffsets, id)
}

// borrowDirectoryString keeps directory dictionaries inside their read lease.
func borrowDirectoryString(data []byte, offsets []uint32, id uint32) string {
	start, end := offsets[id], offsets[id+1]
	if start == end {
		return ""
	}
	return unsafe.String(&data[start], int(end-start))
}

// prepareDirectoryColumns runs only before publication or on a legacy read.
// Full paths are stored once per directory, never once per file or per query.
func (s *Snapshot) prepareDirectoryColumns() {
	if len(s.RowDirectory) != 0 {
		return
	}
	type directory struct {
		row  uint32
		path string
	}
	var directories []directory
	for row, flags := range s.Flags {
		if flags&Directory != 0 {
			path, _ := s.Path(uint32(row))
			directories = append(directories, directory{uint32(row), path})
		}
	}
	sort.Slice(directories, func(i, j int) bool {
		if len(directories[i].path) != len(directories[j].path) {
			return len(directories[i].path) < len(directories[j].path)
		}
		return directories[i].row < directories[j].row
	})
	s.RowDirectory = make([]uint32, s.Len())
	for row := range s.RowDirectory {
		s.RowDirectory[row] = NoRow
	}
	s.DirectoryRows = make([]uint32, len(directories))
	s.DirectoryRuneCounts = make([]uint32, len(directories))
	s.DirectoryOffsets = make([]uint32, len(directories)+1)
	s.DirectoryKeyOffsets = make([]uint32, len(directories)+1)
	words := (len(directories) + 63) / 64
	s.DirectoryBitmaps = make([]uint64, 64*words)
	s.DirectoryPairBitmaps = make([]uint64, 27*27*words)
	bytes := 0
	for _, directory := range directories {
		bytes += len(directory.path)
	}
	s.DirectoryBytes, s.DirectoryKeys = make([]byte, 0, bytes), make([]byte, 0, bytes)
	for id, entry := range directories {
		s.DirectoryRows[id], s.RowDirectory[entry.row] = entry.row, uint32(id)
		s.DirectoryBytes = append(s.DirectoryBytes, entry.path...)
		s.DirectoryOffsets[id+1] = uint32(len(s.DirectoryBytes))
		key := strings.Trim(strings.ReplaceAll(strings.ToLower(filepath.ToSlash(strings.TrimSpace(entry.path))), `\`, "/"), "/")
		s.DirectoryKeys = append(s.DirectoryKeys, key...)
		s.DirectoryKeyOffsets[id+1] = uint32(len(s.DirectoryKeys))
		s.DirectoryRuneCounts[id] = uint32(utf8.RuneCountInString(key))
		mask, nonASCII := CharacterMask(key)
		if nonASCII {
			// Unicode normalization can introduce ASCII letters; exact subsequence
			// and scoring remain responsible for rejecting these candidates.
			mask = ^uint64(0)
		}
		var pairs [27]uint32
		var seen uint32
		for i := range len(key) {
			class := pathPairClass(key[i])
			if class < 0 {
				continue
			}
			for remaining := seen; remaining != 0; remaining &= remaining - 1 {
				pairs[bits.TrailingZeros32(remaining)] |= uint32(1) << uint(class)
			}
			seen |= uint32(1) << uint(class)
		}
		for first, remaining := range pairs {
			if nonASCII {
				remaining = (1 << 27) - 1
			}
			for remaining != 0 {
				second := bits.TrailingZeros32(remaining)
				remaining &= remaining - 1
				s.DirectoryPairBitmaps[(first*27+second)*words+id/64] |= uint64(1) << uint(id%64)
			}
		}
		for mask != 0 {
			class := bits.TrailingZeros64(mask)
			s.DirectoryBitmaps[class*words+id/64] |= uint64(1) << uint(id%64)
			mask &= mask - 1
		}
	}
	// Each directory owns two independent child bounds, since path order differs
	// from parent-row order. Keep them interleaved in one mapped scalar column.
	s.DirectoryStarts = make([]uint32, len(directories)*2)
	for id, row := range s.DirectoryRows {
		start := sort.Search(len(s.ChildRows), func(i int) bool { return s.Parents[s.ChildRows[i]] >= row })
		end := start
		for end < len(s.ChildRows) && s.Parents[s.ChildRows[end]] == row {
			end++
		}
		s.DirectoryStarts[id*2], s.DirectoryStarts[id*2+1] = uint32(start), uint32(end)
	}
}

// directoryColumnParts keeps the extended section segmented during publication.
func (s *Snapshot) directoryColumnParts() [][]byte {
	var header [24]byte
	binary.LittleEndian.PutUint64(header[:], uint64(len(s.DirectoryRows)))
	binary.LittleEndian.PutUint64(header[8:], uint64(len(s.DirectoryBytes)))
	binary.LittleEndian.PutUint64(header[16:], uint64(len(s.DirectoryKeys)))
	parts := [][]byte{header[:]}
	var padding [8]byte
	for _, data := range [][]byte{bytesOf(s.DirectoryRows), bytesOf(s.RowDirectory), bytesOf(s.DirectoryStarts), bytesOf(s.DirectoryOffsets), s.DirectoryBytes, bytesOf(s.DirectoryKeyOffsets), s.DirectoryKeys, bytesOf(s.DirectoryBitmaps), bytesOf(s.DirectoryRuneCounts), bytesOf(s.DirectoryPairBitmaps)} {
		parts = append(parts, data, padding[:(-len(data))&7])
	}
	return parts
}

// mapDirectoryColumns validates references before lending persisted query data.
func (s *Snapshot) mapDirectoryColumns(data []byte) error {
	if len(data) < 24 {
		return fmt.Errorf("missing directory columns")
	}
	n, pathBytes, keyBytes := binary.LittleEndian.Uint64(data), binary.LittleEndian.Uint64(data[8:]), binary.LittleEndian.Uint64(data[16:])
	if n == 0 || n > uint64(s.Len()) || pathBytes > uint64(len(data)) || keyBytes > uint64(len(data)) {
		return fmt.Errorf("invalid directory column counts")
	}
	lengths := []uint64{n * 4, uint64(s.Len()) * 4, n * 8, (n + 1) * 4, pathBytes, (n + 1) * 4, keyBytes, ((n + 63) / 64) * 64 * 8, n * 4, ((n + 63) / 64) * 27 * 27 * 8}
	parts := make([][]byte, len(lengths))
	offset := uint64(24)
	for i, length := range lengths {
		aligned := (length + 7) &^ 7
		if offset > uint64(len(data)) || aligned > uint64(len(data))-offset {
			return fmt.Errorf("invalid directory column bounds")
		}
		parts[i] = data[offset : offset+length]
		offset += aligned
	}
	if offset != uint64(len(data)) {
		return fmt.Errorf("invalid directory trailing data")
	}
	s.DirectoryRuneCounts = view[uint32](parts[8])
	s.DirectoryPairBitmaps = view[uint64](parts[9])
	s.DirectoryRows, s.RowDirectory, s.DirectoryStarts = view[uint32](parts[0]), view[uint32](parts[1]), view[uint32](parts[2])
	s.DirectoryOffsets, s.DirectoryBytes = view[uint32](parts[3]), parts[4]
	s.DirectoryKeyOffsets, s.DirectoryKeys, s.DirectoryBitmaps = view[uint32](parts[5]), parts[6], view[uint64](parts[7])
	if s.DirectoryOffsets[0] != 0 || s.DirectoryKeyOffsets[0] != 0 || uint64(s.DirectoryOffsets[n]) != pathBytes || uint64(s.DirectoryKeyOffsets[n]) != keyBytes {
		return fmt.Errorf("invalid directory strings")
	}
	for id, row := range s.DirectoryRows {
		if uint64(row) >= uint64(s.Len()) || s.Flags[row]&Directory == 0 || s.RowDirectory[row] != uint32(id) || s.DirectoryOffsets[id] > s.DirectoryOffsets[id+1] || s.DirectoryKeyOffsets[id] > s.DirectoryKeyOffsets[id+1] {
			return fmt.Errorf("invalid directory reference")
		}
		start, end := s.DirectoryStarts[id*2], s.DirectoryStarts[id*2+1]
		if start > end || uint64(end) > uint64(len(s.ChildRows)) {
			return fmt.Errorf("invalid directory children")
		}
		if start > 0 && s.Parents[s.ChildRows[start-1]] >= row || end < uint32(len(s.ChildRows)) && s.Parents[s.ChildRows[end]] <= row {
			return fmt.Errorf("incomplete directory children")
		}
		for _, child := range s.ChildRows[start:end] {
			if s.Parents[child] != row {
				return fmt.Errorf("incorrect directory child")
			}
		}
	}
	for row, id := range s.RowDirectory {
		if s.Flags[row]&Directory != 0 {
			if uint64(id) >= n || s.DirectoryRows[id] != uint32(row) {
				return fmt.Errorf("missing directory reference")
			}
		} else if id != NoRow {
			return fmt.Errorf("file has directory ordinal")
		}
	}
	return nil
}

// pathPairClass keeps letters and path separators in independent ordered classes.
func pathPairClass(c byte) int {
	if c >= 'a' && c <= 'z' {
		return int(c - 'a')
	}
	if c == '/' {
		return 26
	}
	return -1
}

// PathPairClasses supplies necessary ordered pairs, never adjacency constraints.
// Unicode patterns bypass this filter because normalization can expand letters.
func PathPairClasses(text string) []int {
	var pairs []int
	previous := -1
	var seen [27 * 27]bool
	for i := range len(text) {
		if text[i] >= 128 {
			return nil
		}
		class := pathPairClass(text[i])
		if class < 0 {
			continue
		}
		if previous >= 0 {
			pair := previous*27 + class
			if !seen[pair] {
				pairs = append(pairs, pair)
				seen[pair] = true
			}
		}
		previous = class
	}
	return pairs
}

// PathCharacterMask normalizes query literals exactly as the subsequence filter.
func PathCharacterMask(text string) uint64 {
	var mask uint64
	for c, present := range fuzzymatch.PossibleASCIICharacters(text, false) {
		if present {
			m, _ := CharacterMask(string(rune(c)))
			mask |= m
		}
	}
	return mask
}
