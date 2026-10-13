package column

import (
	"fmt"
	"math"
	"sort"
)

// Builder interns names during ingestion, without retaining full paths or per-file strings.
type Builder struct {
	base     Snapshot
	parents  []uint64
	rows     map[uint64]uint32
	names    map[string]uint32
	deleted  map[uint64]uint64
	metadata bool
	versions bool
}

func NewBuilder(root string, rootID uint64) *Builder {
	return &Builder{
		base:    Snapshot{Root: root, IDs: []uint64{rootID}, NameIDs: []uint32{0}, Flags: []byte{Directory | Structural}, NameOffsets: []uint32{0, 0}, NameMasks: []uint64{0}, NameFlags: []byte{0}},
		parents: []uint64{rootID}, rows: map[uint64]uint32{rootID: 0}, names: map[string]uint32{"": 0},
	}
}

func (b *Builder) Contains(id uint64) bool {
	_, found := b.rows[id]
	return found
}

// Add permits newer observations to replace a row without changing its stable build row.
func (b *Builder) Add(record Record) error {
	if b.rows == nil {
		return fmt.Errorf("builder already finished")
	}
	if record.ID == b.base.IDs[0] {
		record.ParentID = record.ID
		record.Flags |= Directory
	}
	if version, deleted := b.deleted[record.ID]; deleted {
		if record.Version <= version {
			return nil
		}
		delete(b.deleted, record.ID)
	}
	row, exists := b.rows[record.ID]
	if exists && len(b.base.Versions) != 0 && record.Version < b.base.Versions[row] {
		return nil
	}
	name, known := b.names[record.Name]
	if !known {
		if uint64(len(b.base.NameBytes))+uint64(len(record.Name)) > math.MaxUint32 || len(b.base.NameMasks) >= math.MaxUint32 {
			return fmt.Errorf("column dictionary exceeds 32-bit capacity")
		}
		name = uint32(len(b.base.NameMasks))
		b.names[record.Name] = name
		b.base.NameBytes = append(b.base.NameBytes, record.Name...)
		b.base.NameOffsets = append(b.base.NameOffsets, uint32(len(b.base.NameBytes)))
		mask, nonASCII := CharacterMask(record.Name)
		b.base.NameMasks = append(b.base.NameMasks, mask)
		flag := byte(0)
		if nonASCII {
			flag = NameNonASCII
		}
		b.base.NameFlags = append(b.base.NameFlags, flag)
	}
	if record.Flags&(SizeKnown|ModifiedKnown) != 0 && !b.metadata {
		b.metadata = true
		b.base.Sizes = make([]int64, b.base.Len())
		b.base.Modified = make([]int64, b.base.Len())
	}
	if record.Version != 0 && !b.versions {
		b.versions = true
		b.base.Versions = make([]uint64, b.base.Len())
	}
	if !exists {
		if b.base.Len() >= math.MaxUint32 {
			return fmt.Errorf("column shard exceeds 32-bit capacity")
		}
		row = uint32(b.base.Len())
		b.rows[record.ID] = row
		b.base.IDs = append(b.base.IDs, record.ID)
		b.parents = append(b.parents, record.ParentID)
		b.base.NameIDs = append(b.base.NameIDs, name)
		b.base.Flags = append(b.base.Flags, record.Flags)
		if b.metadata {
			b.base.Sizes = append(b.base.Sizes, record.Size)
			b.base.Modified = append(b.base.Modified, record.Modified)
		}
		if b.versions {
			b.base.Versions = append(b.base.Versions, record.Version)
		}
	} else {
		b.parents[row], b.base.NameIDs[row], b.base.Flags[row] = record.ParentID, name, record.Flags
		if b.metadata {
			b.base.Sizes[row], b.base.Modified[row] = record.Size, record.Modified
		}
		if b.versions {
			b.base.Versions[row] = record.Version
		}
	}
	return nil
}

// Remove replays deletes into an unpublished build without letting an older
// journal observation erase a newer record already seen by MFT enumeration.
func (b *Builder) Remove(id, version uint64) error {
	if b.rows == nil {
		return fmt.Errorf("builder already finished")
	}
	if id == b.base.IDs[0] {
		return fmt.Errorf("cannot delete the column root")
	}
	if row, exists := b.rows[id]; exists && len(b.base.Versions) != 0 && version < b.base.Versions[row] {
		return nil
	}
	if b.deleted == nil {
		b.deleted = make(map[uint64]uint64)
	}
	if previous, exists := b.deleted[id]; !exists || version > previous {
		b.deleted[id] = version
	}
	return nil
}

// pruneDeleted packs scalar columns in place before resolving parent rows.
// Descendants of a deleted parent stay unreachable until a later source repair.
func (b *Builder) pruneDeleted() {
	if len(b.deleted) == 0 {
		return
	}
	s := &b.base
	count := 0
	for row, id := range s.IDs {
		if _, deleted := b.deleted[id]; deleted {
			delete(b.rows, id)
			continue
		}
		s.IDs[count], s.NameIDs[count], s.Flags[count] = id, s.NameIDs[row], s.Flags[row]
		b.parents[count] = b.parents[row]
		b.rows[id] = uint32(count)
		if len(s.Sizes) != 0 {
			s.Sizes[count], s.Modified[count] = s.Sizes[row], s.Modified[row]
		}
		if len(s.Versions) != 0 {
			s.Versions[count] = s.Versions[row]
		}
		count++
	}
	s.IDs, s.NameIDs, s.Flags = s.IDs[:count], s.NameIDs[:count], s.Flags[:count]
	b.parents = b.parents[:count]
	if len(s.Sizes) != 0 {
		s.Sizes, s.Modified = s.Sizes[:count], s.Modified[:count]
	}
	if len(s.Versions) != 0 {
		s.Versions = s.Versions[:count]
	}
}

// Finish resolves late parents and builds compact identifier and name postings.
// Missing parents remain unreachable until reconciliation supplies them.
func (b *Builder) Finish() (*Snapshot, error) {
	if b.rows == nil {
		return nil, fmt.Errorf("builder already finished")
	}
	b.pruneDeleted()
	s := b.base
	s.Parents = make([]uint32, s.Len())
	s.Parents[0] = NoRow
	s.NameStarts = make([]uint32, s.Names()+1)
	s.IDRows = make([]uint32, s.Len())
	for row := range s.Len() {
		s.IDRows[row] = uint32(row)
		if row == 0 {
			continue
		}
		parent, ok := b.rows[b.parents[row]]
		if !ok {
			parent = NoRow
		}
		s.Parents[row] = parent
		s.NameStarts[s.NameIDs[row]+1]++
		if s.Flags[row]&Directory != 0 {
			s.NameFlags[s.NameIDs[row]] |= NameHasDirectory
		} else {
			s.NameFlags[s.NameIDs[row]] |= NameHasFile
		}
	}
	for id := 1; id < len(s.NameStarts); id++ {
		s.NameStarts[id] += s.NameStarts[id-1]
	}
	s.NameRows = make([]uint32, s.Len()-1)
	cursors := append([]uint32(nil), s.NameStarts...)
	for row := 1; row < s.Len(); row++ {
		id := s.NameIDs[row]
		s.NameRows[cursors[id]] = uint32(row)
		cursors[id]++
	}
	sort.Slice(s.IDRows, func(i, j int) bool { return s.IDs[s.IDRows[i]] < s.IDs[s.IDRows[j]] })
	*b = Builder{}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	s.PrepareSearchHints()
	return &s, nil
}
