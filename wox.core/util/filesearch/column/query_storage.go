package column

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

// encodeQueryColumns uses fixed-width, aligned arrays inside the reserved section.
// These derived columns live in the same immutable generation as their names.
func (s *Snapshot) encodeQueryColumns() []byte {
	parts := [][]byte{bytesOf(s.NameScoreHints), bytesOf(s.NameSecondScoreHints), bytesOf(s.NameBitmaps), bytesOf(s.NameRepeatBitmaps), bytesOf(s.NonASCIINameBitmap), bytesOf(s.FrequentNameIDs)}
	size := 8
	for _, part := range parts {
		size += (len(part) + 7) &^ 7
	}
	data := make([]byte, size)
	binary.LittleEndian.PutUint64(data, uint64(len(s.FrequentNameIDs)))
	offset := 8
	for _, part := range parts {
		copy(data[offset:], part)
		offset += (len(part) + 7) &^ 7
	}
	return data
}

// mapQueryColumns validates all lengths before exposing zero-copy scalar views.
func (s *Snapshot) mapQueryColumns(data []byte) error {
	if len(data) < 8 {
		return fmt.Errorf("missing query columns")
	}
	frequent := binary.LittleEndian.Uint64(data)
	names := uint64(s.Names())
	words := (names + 63) / 64
	if frequent > names {
		return fmt.Errorf("invalid frequent-name count")
	}
	lengths := []uint64{names * 8, names * 8, words * 64 * 8, words * 26 * 8, words * 8, frequent * 4}
	parts := make([][]byte, len(lengths))
	offset := uint64(8)
	for i, length := range lengths {
		aligned := (length + 7) &^ 7
		if offset > uint64(len(data)) || aligned > uint64(len(data))-offset {
			return fmt.Errorf("invalid query column bounds")
		}
		parts[i] = data[offset : offset+length]
		offset += aligned
	}
	if offset != uint64(len(data)) {
		return fmt.Errorf("invalid query column trailing data")
	}
	s.NameScoreHints = view[uint64](parts[0])
	s.NameSecondScoreHints = view[uint64](parts[1])
	s.NameBitmaps = view[uint64](parts[2])
	s.NameRepeatBitmaps = view[uint64](parts[3])
	s.NonASCIINameBitmap = view[uint64](parts[4])
	s.FrequentNameIDs = view[uint32](parts[5])
	for _, id := range s.FrequentNameIDs {
		if uint64(id) >= names {
			return fmt.Errorf("invalid frequent-name ID")
		}
	}
	s.queryColumnsReady = true
	return nil
}

// UpgradeQueryColumns is invoked by the migration runner before engines open.
// Replacing only the immutable snapshot preserves the manifest and live journal.
func UpgradeQueryColumns(path string) error {
	s, err := Open(path)
	if err != nil {
		return err
	}
	defer s.Close()
	if binary.LittleEndian.Uint32(s.mapping.data[8:]) >= 3 {
		return nil
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".query-columns-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Remove(temporary); err != nil {
		return err
	}
	if err := Write(temporary, s); err != nil {
		return err
	}
	if err := s.Close(); err != nil {
		return err
	}
	_, err = replaceManifest(temporary, path)
	return err
}
