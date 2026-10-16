package column

// rowHash mixes both halves of native IDs, including NTFS sequence bits. Slots
// store row+1, with zero reserved for empty, and borrow keys from the ID column.
func rowHash(id uint64) uint64 {
	id ^= id >> 30
	id *= 0xbf58476d1ce4e5b9
	id ^= id >> 27
	id *= 0x94d049bb133111eb
	return id ^ (id >> 31)
}

// lookupRow probes a bounded-load table without duplicating 64-bit entry IDs.
func (b *Builder) lookupRow(id uint64) (uint32, bool) {
	if len(b.rows) == 0 {
		return 0, false
	}
	mask := uint64(len(b.rows) - 1)
	for slot := rowHash(id) & mask; ; slot = (slot + 1) & mask {
		value := b.rows[slot]
		if value == 0 {
			return 0, false
		}
		row := value - 1
		if b.base.IDs[row] == id {
			return row, true
		}
	}
}

// addRow runs after appending a unique ID. Growing rehashes all existing rows,
// including that new ID, and caps load at 75% to bound expected probe lengths.
func (b *Builder) addRow(row uint32) {
	if len(b.base.IDs) > len(b.rows)/4*3 {
		b.rows = make([]uint32, max(16, len(b.rows)*2))
		for row := range b.base.IDs {
			b.placeRow(uint32(row))
		}
		return
	}
	b.placeRow(row)
}

// placeRow inserts into a table with spare capacity and no duplicate keys.
func (b *Builder) placeRow(row uint32) {
	mask := uint64(len(b.rows) - 1)
	for slot := rowHash(b.base.IDs[row]) & mask; ; slot = (slot + 1) & mask {
		if b.rows[slot] == 0 {
			b.rows[slot] = row + 1
			return
		}
	}
}
