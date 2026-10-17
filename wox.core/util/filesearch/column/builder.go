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
	rows     []uint32 // Open-addressed row+1 slots; keys remain in base.IDs.
	names    map[string]uint32
	deleted  map[uint64]uint64
	metadata bool
	versions bool
	files    int64
	entries  int64
}

func NewBuilder(root string, rootID uint64) *Builder {
	b := &Builder{
		base:    Snapshot{Root: root, IDs: []uint64{rootID}, NameIDs: []uint32{0}, Flags: []byte{Directory | Structural}, NameOffsets: []uint32{0, 0}, NameMasks: []uint64{0}, NameFlags: []byte{0}},
		parents: []uint64{rootID}, names: map[string]uint32{"": 0},
	}
	b.addRow(0)
	return b
}

func (b *Builder) Contains(id uint64) bool {
	_, found := b.lookupRow(id)
	return found
}

// Counts reports retained non-structural rows before final reachability checks.
// It uses scalar counters so progress never rescans a growing build.
func (b *Builder) Counts() (files, entries int64) { return b.files, b.entries }

// adjustCounts excludes structural parents from user-visible build progress.
func (b *Builder) adjustCounts(flags byte, delta int64) {
	if flags&Structural == 0 {
		b.entries += delta
		if flags&Directory == 0 {
			b.files += delta
		}
	}
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
	version, wasDeleted := b.deleted[record.ID]
	if wasDeleted {
		if record.Version <= version {
			return nil
		}
		delete(b.deleted, record.ID)
	}
	row, exists := b.lookupRow(record.ID)
	if exists && len(b.base.Versions) != 0 && record.Version < b.base.Versions[row] {
		return nil
	}
	var previousFlags byte
	if exists {
		previousFlags = b.base.Flags[row]
	}
	name, known := b.names[record.Name]
	if !known {
		nameEnd := uint64(b.base.NameOffsets[len(b.base.NameOffsets)-1]) + uint64(len(record.Name))
		if nameEnd > math.MaxUint32 || len(b.base.NameMasks) >= math.MaxUint32 {
			return fmt.Errorf("column dictionary exceeds 32-bit capacity")
		}
		name = uint32(len(b.base.NameMasks))
		b.names[record.Name] = name
		// The intern map already owns each distinct name. Materialize packed bytes
		// once at Finish instead of retaining and repeatedly growing a second copy.
		b.base.NameOffsets = append(b.base.NameOffsets, uint32(nameEnd))
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
		b.base.IDs = append(b.base.IDs, record.ID)
		b.addRow(row)
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
	if exists && !wasDeleted {
		b.adjustCounts(previousFlags, -1)
	}
	b.adjustCounts(record.Flags, 1)
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
	if row, exists := b.lookupRow(id); exists && len(b.base.Versions) != 0 && version < b.base.Versions[row] {
		return nil
	}
	if b.deleted == nil {
		b.deleted = make(map[uint64]uint64)
	}
	if _, deleted := b.deleted[id]; !deleted {
		if row, exists := b.lookupRow(id); exists {
			b.adjustCounts(b.base.Flags[row], -1)
		}
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
			continue
		}
		s.IDs[count], s.NameIDs[count], s.Flags[count] = id, s.NameIDs[row], s.Flags[row]
		b.parents[count] = b.parents[row]
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
	// Packing changes the rows used as hash keys. Rebuild only after all IDs
	// reach their final positions, so overwritten rows cannot corrupt lookups.
	clear(b.rows)
	for row := range s.IDs {
		b.placeRow(uint32(row))
	}
}

// Finish resolves late parents and builds compact identifier and name postings.
// Missing parents remain unreachable until reconciliation supplies them.
func (b *Builder) Finish() (*Snapshot, error) {
	if b.rows == nil {
		return nil, fmt.Errorf("builder already finished")
	}
	b.pruneDeleted()
	b.base.NameBytes = make([]byte, int(b.base.NameOffsets[len(b.base.NameOffsets)-1]))
	for name, id := range b.names {
		copy(b.base.NameBytes[b.base.NameOffsets[id]:b.base.NameOffsets[id+1]], name)
	}
	// Ingestion is complete. Release interned string keys and tombstones before
	// allocating final columns; neither is needed to resolve parent rows.
	b.names, b.deleted = nil, nil
	s := b.base
	s.Parents = make([]uint32, s.Len())
	s.Parents[0] = NoRow
	for row := 1; row < s.Len(); row++ {
		parent, ok := b.lookupRow(b.parents[row])
		if !ok {
			parent = NoRow
		}
		s.Parents[row] = parent
	}
	// Parent IDs and the ID-to-row table have served their last purpose. Keeping
	// them until after postings/sort construction overlaps whole-volume buffers.
	*b = Builder{}
	s.NameStarts = make([]uint32, s.Names()+1)
	s.IDRows = make([]uint32, s.Len())
	for row := range s.Len() {
		s.IDRows[row] = uint32(row)
		if row == 0 {
			continue
		}
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
	// Scatter advances starts to group ends. Shifting those ends restores the
	// original offsets without a second dictionary-sized cursor array.
	for row := 1; row < s.Len(); row++ {
		id := s.NameIDs[row]
		s.NameRows[s.NameStarts[id]] = uint32(row)
		s.NameStarts[id]++
	}
	copy(s.NameStarts[1:], s.NameStarts[:len(s.NameStarts)-1])
	s.NameStarts[0] = 0
	sort.Slice(s.IDRows, func(i, j int) bool { return s.IDs[s.IDRows[i]] < s.IDs[s.IDRows[j]] })
	s.prepareChildRows()
	if err := s.Validate(); err != nil {
		return nil, err
	}
	s.PrepareSearchHints()
	return &s, nil
}
