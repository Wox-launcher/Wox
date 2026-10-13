package column

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
)

const maxJournalBatch = 1 << 20
const recordHeaderSize = 46

type Change struct {
	Record
	Deleted bool
}

// encodeBatch keeps a source cursor in the same checksummed record as its mutations.
func encodeBatch(changes []Change, cursor string) ([]byte, error) {
	size := 8 + len(cursor)
	for _, change := range changes {
		if len(change.Name) > maxJournalBatch-recordHeaderSize || size > maxJournalBatch-recordHeaderSize-len(change.Name) {
			return nil, fmt.Errorf("column journal batch too large")
		}
		size += recordHeaderSize + len(change.Name)
	}
	if size > maxJournalBatch {
		return nil, fmt.Errorf("column journal cursor too large")
	}
	data := make([]byte, 8+size)
	binary.LittleEndian.PutUint32(data, uint32(size))
	body := data[8:]
	binary.LittleEndian.PutUint32(body, uint32(len(cursor)))
	binary.LittleEndian.PutUint32(body[4:], uint32(len(changes)))
	copy(body[8:], cursor)
	offset := 8 + len(cursor)
	for _, change := range changes {
		record := body[offset:]
		binary.LittleEndian.PutUint64(record, change.ID)
		binary.LittleEndian.PutUint64(record[8:], change.ParentID)
		binary.LittleEndian.PutUint64(record[16:], change.Version)
		binary.LittleEndian.PutUint64(record[24:], uint64(change.Size))
		binary.LittleEndian.PutUint64(record[32:], uint64(change.Modified))
		record[40] = change.Flags
		if change.Deleted {
			record[41] = 1
		}
		binary.LittleEndian.PutUint32(record[42:], uint32(len(change.Name)))
		copy(record[recordHeaderSize:], change.Name)
		offset += recordHeaderSize + len(change.Name)
	}
	binary.LittleEndian.PutUint32(data[4:], crc32.ChecksumIEEE(body))
	return data, nil
}

// readBatch bounds every allocation and returns UnexpectedEOF for an uncommitted tail.
func readBatch(reader io.Reader) ([]Change, string, int64, error) {
	var header [8]byte
	n, err := io.ReadFull(reader, header[:])
	if err != nil {
		return nil, "", int64(n), err
	}
	size := binary.LittleEndian.Uint32(header[:])
	if size < 8 || size > maxJournalBatch {
		return nil, "", 8, fmt.Errorf("invalid column journal frame size")
	}
	data := make([]byte, int(size))
	n, err = io.ReadFull(reader, data)
	if err != nil {
		return nil, "", int64(8 + n), err
	}
	if crc32.ChecksumIEEE(data) != binary.LittleEndian.Uint32(header[4:]) {
		return nil, "", int64(8 + n), fmt.Errorf("column journal checksum mismatch")
	}
	cursorSize, count := int(binary.LittleEndian.Uint32(data)), int(binary.LittleEndian.Uint32(data[4:]))
	if cursorSize > len(data)-8 || count > (len(data)-8-cursorSize)/recordHeaderSize {
		return nil, "", int64(8 + n), fmt.Errorf("invalid column journal counts")
	}
	cursor := string(data[8 : 8+cursorSize])
	offset := 8 + cursorSize
	changes := make([]Change, 0, count)
	for range count {
		if len(data)-offset < recordHeaderSize {
			return nil, "", int64(8 + n), fmt.Errorf("truncated column journal record")
		}
		r := data[offset:]
		length := int(binary.LittleEndian.Uint32(r[42:]))
		if length > len(r)-recordHeaderSize || r[41] > 1 {
			return nil, "", int64(8 + n), fmt.Errorf("invalid column journal record")
		}
		changes = append(changes, Change{Record: Record{ID: binary.LittleEndian.Uint64(r), ParentID: binary.LittleEndian.Uint64(r[8:]), Version: binary.LittleEndian.Uint64(r[16:]), Size: int64(binary.LittleEndian.Uint64(r[24:])), Modified: int64(binary.LittleEndian.Uint64(r[32:])), Flags: r[40], Name: string(r[recordHeaderSize : recordHeaderSize+length])}, Deleted: r[41] != 0})
		offset += recordHeaderSize + length
	}
	if offset != len(data) {
		return nil, "", int64(8 + n), fmt.Errorf("trailing column journal data")
	}
	return changes, cursor, int64(8 + n), nil
}
