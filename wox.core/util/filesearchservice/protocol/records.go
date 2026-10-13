package protocol

import (
	"encoding/binary"
	"fmt"
	"unicode/utf8"
)

const (
	Directory uint32 = 1 << iota
	SizeKnown
	ModifiedKnown
	Deleted
)

const entrySize = 56

// Entry contains filesystem facts only. Volume identity belongs to Begin/Checkpoint,
// keeping it out of every row; metadata validity is explicit because USN omits size.
type Entry struct {
	ID       uint64
	ParentID uint64
	USN      int64
	Reason   uint32
	Flags    uint32
	Size     int64
	Modified int64
	Name     string
}

// AppendEntry writes one fixed record plus its length-prefixed filename to a reusable batch.
// Offsets are local to the batch, so no decoder pointer can refer outside its frame.
func AppendEntry(batch []byte, entry Entry) ([]byte, error) {
	if !utf8.ValidString(entry.Name) || entry.Flags & ^uint32(Directory|SizeKnown|ModifiedKnown|Deleted) != 0 || len(entry.Name) > MaxPayload-entrySize || len(batch) > MaxPayload-entrySize-len(entry.Name) {
		return batch, fmt.Errorf("invalid file fact record size or encoding")
	}
	start := len(batch)
	batch = append(batch, make([]byte, entrySize+len(entry.Name))...)
	r := batch[start:]
	binary.LittleEndian.PutUint64(r, entry.ID)
	binary.LittleEndian.PutUint64(r[8:], entry.ParentID)
	binary.LittleEndian.PutUint64(r[16:], uint64(entry.USN))
	binary.LittleEndian.PutUint32(r[24:], entry.Reason)
	binary.LittleEndian.PutUint32(r[28:], entry.Flags)
	binary.LittleEndian.PutUint64(r[32:], uint64(entry.Size))
	binary.LittleEndian.PutUint64(r[40:], uint64(entry.Modified))
	binary.LittleEndian.PutUint32(r[48:], uint32(len(entry.Name)))
	copy(r[entrySize:], entry.Name)
	return batch, nil
}

// VisitEntries decodes a batch without building a second slice of all its records.
func VisitEntries(batch []byte, visit func(Entry) error) error {
	if len(batch) > MaxPayload {
		return fmt.Errorf("file fact batch too large")
	}
	for len(batch) != 0 {
		if len(batch) < entrySize {
			return fmt.Errorf("truncated file fact record")
		}
		length := int(binary.LittleEndian.Uint32(batch[48:]))
		if length > len(batch)-entrySize || binary.LittleEndian.Uint32(batch[52:]) != 0 {
			return fmt.Errorf("invalid file fact name length")
		}
		name := batch[entrySize : entrySize+length]
		flags := binary.LittleEndian.Uint32(batch[28:])
		if !utf8.Valid(name) || flags & ^uint32(Directory|SizeKnown|ModifiedKnown|Deleted) != 0 {
			return fmt.Errorf("invalid file fact name or flags")
		}
		entry := Entry{ID: binary.LittleEndian.Uint64(batch), ParentID: binary.LittleEndian.Uint64(batch[8:]), USN: int64(binary.LittleEndian.Uint64(batch[16:])), Reason: binary.LittleEndian.Uint32(batch[24:]), Flags: flags, Size: int64(binary.LittleEndian.Uint64(batch[32:])), Modified: int64(binary.LittleEndian.Uint64(batch[40:])), Name: string(name)}
		if err := visit(entry); err != nil {
			return err
		}
		batch = batch[entrySize+length:]
	}
	return nil
}

// Boundary identifies one volume generation and an exclusive replay position.
// A mismatched journal or wrapped history must return ResetRequired, never an empty success.
type Boundary struct {
	Volume    string
	RootID    uint64
	JournalID uint64
	NextUSN   int64
}

// EncodeBoundary describes the replay position without repeating it in every entry.
func EncodeBoundary(boundary Boundary) ([]byte, error) {
	if len(boundary.Volume) > 4096 || !utf8.ValidString(boundary.Volume) {
		return nil, fmt.Errorf("invalid volume identity")
	}
	data := make([]byte, 28+len(boundary.Volume))
	binary.LittleEndian.PutUint64(data, boundary.RootID)
	binary.LittleEndian.PutUint64(data[8:], boundary.JournalID)
	binary.LittleEndian.PutUint64(data[16:], uint64(boundary.NextUSN))
	binary.LittleEndian.PutUint32(data[24:], uint32(len(boundary.Volume)))
	copy(data[28:], boundary.Volume)
	return data, nil
}

// DecodeBoundary rejects trailing or oversized fields before accepting a cursor.
func DecodeBoundary(data []byte) (Boundary, error) {
	if len(data) < 28 || len(data) > 28+4096 || int(binary.LittleEndian.Uint32(data[24:])) != len(data)-28 || !utf8.Valid(data[28:]) {
		return Boundary{}, fmt.Errorf("invalid volume boundary")
	}
	return Boundary{RootID: binary.LittleEndian.Uint64(data), JournalID: binary.LittleEndian.Uint64(data[8:]), NextUSN: int64(binary.LittleEndian.Uint64(data[16:])), Volume: string(data[28:])}, nil
}
