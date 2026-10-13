package column

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"runtime"
	"unsafe"
)

const headerSize = 512
const sectionCount = 16
const magic = "WOXCOL02"

// bytesOf exposes pointer-free scalar columns for sequential snapshot writes.
// Supported Wox targets are little-endian; Open rejects incompatible files.
func bytesOf[T ~uint64 | ~int64 | ~uint32 | ~uint8](values []T) []byte {
	if len(values) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(&values[0])), len(values)*int(unsafe.Sizeof(values[0])))
}

// Write creates an unpublished, exclusive snapshot. The owner publishes its manifest only after success.
func Write(path string, s *Snapshot) (err error) {
	if err := s.Validate(); err != nil {
		return err
	}
	s.PrepareSearchHints()
	sections := [][]byte{[]byte(s.Root), bytesOf(s.IDs), bytesOf(s.Parents), bytesOf(s.NameIDs), s.Flags,
		bytesOf(s.NameOffsets), s.NameBytes, bytesOf(s.NameMasks), s.NameFlags, bytesOf(s.NameStarts),
		bytesOf(s.NameRows), bytesOf(s.IDRows), bytesOf(s.Sizes), bytesOf(s.Modified), bytesOf(s.Versions), s.encodeQueryColumns()}
	header := make([]byte, headerSize)
	copy(header, magic)
	binary.LittleEndian.PutUint32(header[8:], 3)
	binary.LittleEndian.PutUint32(header[12:], sectionCount)
	offset := uint64(headerSize)
	for i, section := range sections {
		descriptor := header[32+i*24:]
		binary.LittleEndian.PutUint64(descriptor, offset)
		binary.LittleEndian.PutUint64(descriptor[8:], uint64(len(section)))
		binary.LittleEndian.PutUint32(descriptor[16:], crc32.ChecksumIEEE(section))
		offset += (uint64(len(section)) + 7) &^ 7
	}
	binary.LittleEndian.PutUint64(header[16:], offset)
	binary.LittleEndian.PutUint32(header[24:], crc32.ChecksumIEEE(header[32:]))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(path)
		}
	}()
	if _, err = file.Write(header); err != nil {
		return err
	}
	var padding [8]byte
	for _, section := range sections {
		if _, err = file.Write(section); err != nil {
			return err
		}
		if _, err = file.Write(padding[:(-len(section))&7]); err != nil {
			return err
		}
	}
	return file.Sync()
}

// Open verifies the complete file before lending typed views into the read-only mapping.
func Open(path string) (*Snapshot, error) {
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" && runtime.GOARCH != "386" {
		return nil, fmt.Errorf("unsupported column architecture")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if stat.Size() < headerSize || uint64(stat.Size()) > uint64(int(^uint(0)>>1)) {
		return nil, fmt.Errorf("invalid column file size")
	}
	header := make([]byte, headerSize)
	if _, err := io.ReadFull(file, header); err != nil {
		return nil, err
	}
	version := binary.LittleEndian.Uint32(header[8:])
	if string(header[:8]) != magic || version < 1 || version > 3 || binary.LittleEndian.Uint32(header[12:]) != sectionCount || binary.LittleEndian.Uint64(header[16:]) != uint64(stat.Size()) || binary.LittleEndian.Uint32(header[24:]) != crc32.ChecksumIEEE(header[32:]) {
		return nil, fmt.Errorf("invalid column header")
	}
	mapping, err := mapFile(file, int(stat.Size()))
	if err != nil {
		return nil, err
	}
	valid := false
	defer func() {
		if !valid {
			_ = mapping.close()
		}
	}()
	sections := make([][]byte, sectionCount)
	offset := uint64(headerSize)
	for i := range sections {
		descriptor := header[32+i*24:]
		start, length := binary.LittleEndian.Uint64(descriptor), binary.LittleEndian.Uint64(descriptor[8:])
		if start != offset || start > uint64(len(mapping.data)) || length > uint64(len(mapping.data))-start {
			return nil, fmt.Errorf("invalid column section bounds")
		}
		sections[i] = mapping.data[start : start+length]
		if crc32.ChecksumIEEE(sections[i]) != binary.LittleEndian.Uint32(descriptor[16:]) {
			return nil, fmt.Errorf("column checksum mismatch")
		}
		offset = start + ((length + 7) &^ 7)
	}
	if offset != uint64(stat.Size()) {
		return nil, fmt.Errorf("invalid column trailing data")
	}
	widths := []int{1, 8, 4, 4, 1, 4, 1, 8, 1, 4, 4, 4, 8, 8, 8, 8}
	for i, data := range sections {
		if len(data)%widths[i] != 0 {
			return nil, fmt.Errorf("misaligned column length")
		}
	}
	s := &Snapshot{Root: string(sections[0]), IDs: view[uint64](sections[1]), Parents: view[uint32](sections[2]), NameIDs: view[uint32](sections[3]), Flags: sections[4], NameOffsets: view[uint32](sections[5]), NameBytes: sections[6], NameMasks: view[uint64](sections[7]), NameFlags: sections[8], NameStarts: view[uint32](sections[9]), NameRows: view[uint32](sections[10]), IDRows: view[uint32](sections[11]), Sizes: view[int64](sections[12]), Modified: view[int64](sections[13]), Versions: view[uint64](sections[14]), NameRowsPathSorted: binary.LittleEndian.Uint32(header[8:]) >= 2, mapping: mapping}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	if version >= 3 {
		if err := s.mapQueryColumns(sections[15]); err != nil {
			return nil, err
		}
	} else {
		// Compatibility reads remain write-free. The ordered migration upgrades
		// old generations before normal engine startup.
		s.PrepareSearchHints()
	}
	valid = true
	return s, nil
}

func view[T ~uint64 | ~int64 | ~uint32](data []byte) []T {
	if len(data) == 0 {
		return nil
	}
	var scalar T
	return unsafe.Slice((*T)(unsafe.Pointer(&data[0])), len(data)/int(unsafe.Sizeof(scalar)))
}
