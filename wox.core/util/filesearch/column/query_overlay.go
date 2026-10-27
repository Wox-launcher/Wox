package column

import (
	"math/bits"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// QueryOverlay contains revision-owned edges and invalidations. Readers borrow
// it under the store lease rather than rebuilding it for each keystroke.
type QueryOverlay struct {
	Children         map[uint64][]uint64
	StaleDirectories []uint64
	// LiveDirectories is bounded: a large moved subtree keeps the uncached
	// traversal instead of retaining a second volume-sized directory dictionary.
	LiveDirectories []LiveDirectory
	PathsCached     bool
	Names           []LiveName
}

// LiveName keeps cheap rejection metadata beside one live change ID. Stable
// names borrow base filters; changed names retain no duplicate filename text.
type LiveName struct {
	ID       uint64
	Mask     uint64
	BaseName uint32
	NonASCII bool
}

// LiveDirectory owns a bounded overlay path while its record borrows the lease.
type LiveDirectory struct {
	Record    Record
	Row       uint32
	Path, Key string
	Runes     int
}

// PrepareQueryView also supports standalone views used by exhaustive tests.
func (v View) PrepareQueryView() View {
	if v.Query == nil {
		v.Query = buildQueryOverlay(v)
	}
	return v
}

// DirectoryUnchanged permits borrowing a persisted path only when every base
// ancestor survived. Renames, moves, deletions and type changes invalidate the
// complete old subtree, including files whose own records never changed.
func (v View) DirectoryUnchanged(row uint32) bool {
	if row == NoRow || len(v.Base.RowDirectory) == 0 {
		return false
	}
	id := v.Base.RowDirectory[row]
	if id == NoRow {
		return false
	}
	return len(v.Delta) == 0 || v.Query != nil && (len(v.Query.StaleDirectories) == 0 || v.Query.StaleDirectories[id/64]&(uint64(1)<<uint(id%64)) == 0)
}

// buildQueryOverlay scales with live changes and affected subtrees, not all
// base entries. File-only changes require no directory invalidation bitmap.
func buildQueryOverlay(v View) *QueryOverlay {
	q := &QueryOverlay{Children: make(map[uint64][]uint64), Names: make([]LiveName, 0, len(v.Delta))}
	var stack []uint32
	var added []Record
	for _, change := range v.Delta {
		row, inBase := v.Base.Row(change.ID)
		if !change.Deleted && change.ID != v.Base.IDs[0] {
			q.Children[change.ParentID] = append(q.Children[change.ParentID], change.ID)
			mask, nonASCII := CharacterMask(change.Name)
			name := LiveName{ID: change.ID, Mask: mask, NonASCII: nonASCII, BaseName: NoRow}
			if nonASCII {
				name.Mask = ^uint64(0)
			}
			if inBase && change.Name == v.Base.Name(v.Base.NameIDs[row]) {
				name.BaseName = v.Base.NameIDs[row]
				if !nonASCII || v.Base.NameFlags[name.BaseName]&NameMaskComplete != 0 {
					name.Mask = v.Base.NameMasks[name.BaseName]
				}
			}
			q.Names = append(q.Names, name)
		}
		if inBase && v.Base.Flags[row]&Directory != 0 {
			parent := v.Base.Parents[row]
			if change.Deleted || change.Flags&Directory == 0 || parent == NoRow || change.ParentID != v.Base.IDs[parent] || change.Name != v.Base.Name(v.Base.NameIDs[row]) {
				stack = append(stack, row)
			}
		}
		if !change.Deleted && change.Flags&Directory != 0 && (!inBase || v.Base.Flags[row]&Directory == 0) {
			added = append(added, change.Record)
		}
	}
	if len(stack) > 0 {
		q.StaleDirectories = make([]uint64, (len(v.Base.DirectoryRows)+63)/64)
	}
	for len(stack) > 0 {
		row := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		id := v.Base.RowDirectory[row]
		bit := uint64(1) << uint(id%64)
		if q.StaleDirectories[id/64]&bit != 0 {
			continue
		}
		q.StaleDirectories[id/64] |= bit
		for _, child := range v.Base.Children(row) {
			if v.Base.Flags[child]&Directory != 0 {
				stack = append(stack, child)
			}
		}
	}
	count := 0
	for _, word := range q.StaleDirectories {
		count += bits.OnesCount64(word)
	}
	if count+len(added) <= 4096 {
		q.PathsCached = true
		appendDirectory := func(record Record, row uint32) {
			path, ok := v.Path(record.ID)
			if !ok {
				return
			}
			key := strings.Trim(strings.ReplaceAll(strings.ToLower(filepath.ToSlash(strings.TrimSpace(path))), `\`, "/"), "/")
			q.LiveDirectories = append(q.LiveDirectories, LiveDirectory{Record: record, Row: row, Path: path, Key: key, Runes: utf8.RuneCountInString(key)})
		}
		for word, remaining := range q.StaleDirectories {
			for remaining != 0 {
				id := word*64 + bits.TrailingZeros64(remaining)
				remaining &= remaining - 1
				row := v.Base.DirectoryRows[id]
				if record, ok := v.Lookup(v.Base.IDs[row]); ok && record.Flags&Directory != 0 {
					appendDirectory(record, row)
				}
			}
		}
		for _, record := range added {
			appendDirectory(record, NoRow)
		}
	}
	return q
}
