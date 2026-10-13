// Package column owns immutable filename columns independently of scanning and query policy.
package column

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"unsafe"
	"wox/util/fuzzymatch"
)

const NoRow uint32 = math.MaxUint32

const (
	NameNonASCII byte = 1 << iota
	NameHasDirectory
	NameHasFile
	// NameMaskComplete includes source-supplied normalization/pinyin characters.
	NameMaskComplete
)

const (
	Directory byte = 1 << iota
	SizeKnown
	ModifiedKnown
	// Structural rows resolve ancestry but are not source-observed search entries.
	Structural
)

// Record describes one directory entry; ID is local to a shard, not a global inode.
// Adapters keep native object identity separate when multiple entries share an object.
type Record struct {
	ID, ParentID   uint64
	Name           string
	Flags          byte
	Size, Modified int64
	Version        uint64
}

// Snapshot stores only scalar columns and interned names. Callers retain a read
// lease until all names borrowed from a mapped snapshot are no longer in use.
type Snapshot struct {
	Root                 string
	IDs                  []uint64
	Parents              []uint32
	NameIDs              []uint32
	Flags                []byte
	NameOffsets          []uint32
	NameBytes            []byte
	NameMasks            []uint64
	NameScoreHints       []uint64
	NameSecondScoreHints []uint64
	FrequentNameIDs      []uint32
	NameRowsPathSorted   bool
	queryColumnsReady    bool
	NameBitmaps          []uint64
	NameRepeatBitmaps    []uint64
	NonASCIINameBitmap   []uint64
	NameFlags            []byte
	NameStarts           []uint32
	NameRows             []uint32
	IDRows               []uint32
	Sizes                []int64
	Modified             []int64
	Versions             []uint64
	mapping              *fileMapping
}

func (s *Snapshot) Len() int   { return len(s.IDs) }
func (s *Snapshot) Names() int { return len(s.NameMasks) }

// Name borrows the immutable dictionary bytes without allocating a string.
func (s *Snapshot) Name(id uint32) string {
	start, end := s.NameOffsets[id], s.NameOffsets[id+1]
	if start == end {
		return ""
	}
	return unsafe.String(&s.NameBytes[start], int(end-start))
}

// Row resolves an adapter's stable identifier through a compact sorted indirection.
func (s *Snapshot) Row(id uint64) (uint32, bool) {
	i := sort.Search(len(s.IDRows), func(i int) bool { return s.IDs[s.IDRows[i]] >= id })
	if i == len(s.IDRows) || s.IDs[s.IDRows[i]] != id {
		return NoRow, false
	}
	return s.IDRows[i], true
}

// Path materializes one surviving result, never a path cache for the whole shard.
func (s *Snapshot) Path(row uint32) (string, bool) {
	var local [32]string
	parts := local[:0]
	for steps := 0; row != 0; steps++ {
		if row == NoRow || uint64(row) >= uint64(s.Len()) || steps >= s.Len() {
			return "", false
		}
		parts = append(parts, s.Name(s.NameIDs[row]))
		row = s.Parents[row]
	}
	for left, right := 0, len(parts)-1; left < right; left, right = left+1, right-1 {
		parts[left], parts[right] = parts[right], parts[left]
	}
	return filepath.Join(s.Root, strings.Join(parts, string(filepath.Separator))), true
}

// Record returns a borrowed view of one row, including optional metadata.
func (s *Snapshot) Record(row uint32) Record {
	result := Record{ID: s.IDs[row], Name: s.Name(s.NameIDs[row]), Flags: s.Flags[row]}
	if parent := s.Parents[row]; parent != NoRow {
		result.ParentID = s.IDs[parent]
	}
	if len(s.Sizes) != 0 {
		result.Size = s.Sizes[row]
	}
	if len(s.Modified) != 0 {
		result.Modified = s.Modified[row]
	}
	if len(s.Versions) != 0 {
		result.Version = s.Versions[row]
	}
	return result
}

func (s *Snapshot) Close() error {
	if s.mapping == nil {
		return nil
	}
	err := s.mapping.close()
	*s = Snapshot{}
	return err
}

// CharacterMask is a necessary, never sufficient condition for ASCII fuzzy matching.
// Non-ASCII names bypass it because normalization and pinyin can introduce letters.
func CharacterMask(text string) (mask uint64, nonASCII bool) {
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c >= 128 {
			nonASCII = true
			continue
		}
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		// Reserve separate bits for letters and digits: modulo-64 put 'r' and
		// '2' on the same bit, defeating rejection for numbered filenames.
		var bit byte
		switch {
		case c >= 'a' && c <= 'z':
			bit = c - 'a'
		case c >= '0' && c <= '9':
			bit = 26 + c - '0'
		default:
			bit = 36 + c%28
		}
		mask |= uint64(1) << bit
	}
	return
}

// Validate checks references before exposing mapped memory to traversal or search.
func (s *Snapshot) Validate() error {
	n, names := s.Len(), s.Names()
	if len(s.NameScoreHints) != 0 && len(s.NameScoreHints) != names {
		return fmt.Errorf("invalid name score hints")
	}
	if n == 0 || names == 0 || s.Root == "" || !filepath.IsAbs(s.Root) {
		return fmt.Errorf("invalid column root")
	}
	if len(s.Parents) != n || len(s.NameIDs) != n || len(s.Flags) != n || len(s.IDRows) != n || len(s.NameRows) != n-1 {
		return fmt.Errorf("invalid row column lengths")
	}
	if len(s.NameOffsets) != names+1 || len(s.NameFlags) != names || len(s.NameStarts) != names+1 {
		return fmt.Errorf("invalid name column lengths")
	}
	if s.NameOffsets[0] != 0 || uint64(s.NameOffsets[names]) != uint64(len(s.NameBytes)) || s.NameStarts[0] != 0 || uint64(s.NameStarts[names]) != uint64(n-1) {
		return fmt.Errorf("invalid dictionary bounds")
	}
	for _, length := range []int{len(s.Sizes), len(s.Modified), len(s.Versions)} {
		if length != 0 && length != n {
			return fmt.Errorf("invalid optional column length")
		}
	}
	if s.Parents[0] != NoRow || s.Flags[0]&Directory == 0 {
		return fmt.Errorf("invalid root row")
	}
	seen := make([]uint64, (n+63)/64)
	for i := 0; i < names; i++ {
		if s.NameOffsets[i] > s.NameOffsets[i+1] || uint64(s.NameOffsets[i+1]) > uint64(len(s.NameBytes)) || s.NameStarts[i] > s.NameStarts[i+1] || uint64(s.NameStarts[i+1]) > uint64(len(s.NameRows)) {
			return fmt.Errorf("unordered dictionary bounds")
		}
		for _, row := range s.NameRows[s.NameStarts[i]:s.NameStarts[i+1]] {
			if row == 0 || uint64(row) >= uint64(n) || s.NameIDs[row] != uint32(i) {
				return fmt.Errorf("invalid name posting")
			}
			bit := uint64(1) << (row % 64)
			if seen[row/64]&bit != 0 {
				return fmt.Errorf("duplicate name posting")
			}
			seen[row/64] |= bit
		}
	}
	for row := range n {
		if uint64(s.NameIDs[row]) >= uint64(names) || uint64(s.IDRows[row]) >= uint64(n) {
			return fmt.Errorf("invalid row reference")
		}
		if row > 0 && s.IDs[s.IDRows[row-1]] >= s.IDs[s.IDRows[row]] {
			return fmt.Errorf("unordered entry identifiers")
		}
		if row > 0 {
			parent := s.Parents[row]
			if parent != NoRow && (uint64(parent) >= uint64(n) || s.Flags[parent]&Directory == 0) {
				return fmt.Errorf("invalid parent")
			}
			name := s.Name(s.NameIDs[row])
			if name == "" || name == "." || name == ".." || strings.ContainsRune(name, 0) || strings.ContainsRune(name, '/') || strings.ContainsRune(name, filepath.Separator) || filepath.VolumeName(name) != "" {
				return fmt.Errorf("invalid entry name")
			}
			if s.Flags[row]&SizeKnown != 0 && len(s.Sizes) == 0 || s.Flags[row]&ModifiedKnown != 0 && len(s.Modified) == 0 {
				return fmt.Errorf("missing metadata column")
			}
		}
	}
	// Three states detect cycles in linear time even when MFT children arrive before parents.
	state := make([]byte, n)
	state[0] = 2
	for start := 1; start < n; start++ {
		row := uint32(start)
		for row != NoRow && state[row] == 0 {
			state[row] = 1
			row = s.Parents[row]
		}
		if row != NoRow && state[row] == 1 {
			return fmt.Errorf("parent cycle")
		}
		row = uint32(start)
		for row != NoRow && state[row] == 1 {
			state[row] = 2
			row = s.Parents[row]
		}
	}
	return nil
}

// PrepareSearchHints builds query columns on unpublished or legacy snapshots.
// Current mappings already contain these columns and never copy them to the heap.
func (s *Snapshot) PrepareSearchHints() {
	if s.queryColumnsReady {
		return
	}
	s.prepareNameBitmaps()
	if !s.NameRowsPathSorted {
		// Legacy mappings are read-only. Only the compact posting column is copied;
		// future snapshots persist this ordering as part of normal publication.
		if s.mapping != nil {
			s.NameRows = append([]uint32(nil), s.NameRows...)
		}
		for id := 0; id < s.Names(); id++ {
			rows := s.NameRows[s.NameStarts[id]:s.NameStarts[id+1]]
			sort.Slice(rows, func(i, j int) bool { return s.comparePaths(rows[i], rows[j]) < 0 })
		}
		s.NameRowsPathSorted = true
	}

	s.FrequentNameIDs = nil
	for id := 0; id < s.Names(); id++ {
		if s.NameStarts[id+1]-s.NameStarts[id] >= 16 {
			s.FrequentNameIDs = append(s.FrequentNameIDs, uint32(id))
		}
	}

	if len(s.NameScoreHints) == s.Names() && len(s.NameSecondScoreHints) == s.Names() {
		s.queryColumnsReady = true
		return
	}
	s.NameScoreHints = make([]uint64, s.Names())
	s.NameSecondScoreHints = make([]uint64, s.Names())
	for id := range s.NameScoreHints {
		if s.NameFlags[id]&NameNonASCII == 0 {
			s.NameScoreHints[id], s.NameSecondScoreHints[id] = fuzzymatch.ASCIINameScoreHints(s.Name(uint32(id)))
		}
	}
	s.queryColumnsReady = true
}

// comparePaths orders postings by their joined byte paths without allocating
// strings. Unreachable entries sort last and remain excluded by query readers.
func (s *Snapshot) comparePaths(left, right uint32) int {
	var a, b [64]uint32
	x, y := a[:0], b[:0]
	for left != 0 && left != NoRow {
		x = append(x, left)
		left = s.Parents[left]
	}
	for right != 0 && right != NoRow {
		y = append(y, right)
		right = s.Parents[right]
	}
	if left == NoRow {
		if right == NoRow {
			return 0
		}
		return 1
	}
	if right == NoRow {
		return -1
	}
	for i, j := len(x)-1, len(y)-1; i >= 0 && j >= 0; i, j = i-1, j-1 {
		u, v := s.Name(s.NameIDs[x[i]]), s.Name(s.NameIDs[y[j]])
		n := min(len(u), len(v))
		for k := 0; k < n; k++ {
			if u[k] < v[k] {
				return -1
			}
			if u[k] > v[k] {
				return 1
			}
		}
		if len(u) != len(v) {
			c, d := -1, -1
			if len(u) > n {
				c = int(u[n])
			} else if i > 0 {
				c = int(filepath.Separator)
			}
			if len(v) > n {
				d = int(v[n])
			} else if j > 0 {
				d = int(filepath.Separator)
			}
			if c < d {
				return -1
			}
			return 1
		}
	}
	if len(x) < len(y) {
		return -1
	}
	if len(x) > len(y) {
		return 1
	}
	return 0
}
