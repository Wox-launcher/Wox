package column

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"runtime"
	"unsafe"
)

const headerSize = 512
const magic = "WOXCOL02"

const (
	// legacySectionCount files predate the persisted child index; Open rebuilds
	// that index on the heap until the next publication rewrites the generation.
	legacySectionCount = 16
	sectionCount       = 17
	queryColumnSection = 15
	childRowsSection   = 16
)

// bytesOf exposes pointer-free scalar columns for sequential snapshot writes.
// Supported Wox targets are little-endian; Open rejects incompatible files.
func bytesOf[T ~uint64 | ~int64 | ~uint32 | ~uint8](values []T) []byte {
	if len(values) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(&values[0])), len(values)*int(unsafe.Sizeof(values[0])))
}

// Write creates an unpublished, exclusive snapshot. The owner publishes its manifest only after success.
func Write(path string, s *Snapshot) error {
	_, err := writeSnapshot(path, s)
	return err
}

// writeAndOpen keeps the small header produced from validated heap columns as
// the checksum authority. Publication can verify disk bytes without faulting a
// second complete copy into the mapping while the build heap awaits collection.
func writeAndOpen(path string, s *Snapshot) (*Snapshot, error) {
	header, err := writeSnapshot(path, s)
	if err != nil {
		return nil, err
	}
	return openSnapshot(path, header)
}

// writeSnapshot returns its exact header only after a successful durable write.
func writeSnapshot(path string, s *Snapshot) (header []byte, err error) {
	s.prepareChildRows()
	if err := s.Validate(); err != nil {
		return nil, err
	}
	s.PrepareSearchHints()
	columns := [][]byte{[]byte(s.Root), bytesOf(s.IDs), bytesOf(s.Parents), bytesOf(s.NameIDs), s.Flags,
		bytesOf(s.NameOffsets), s.NameBytes, bytesOf(s.NameMasks), s.NameFlags, bytesOf(s.NameStarts),
		bytesOf(s.NameRows), bytesOf(s.IDRows), bytesOf(s.Sizes), bytesOf(s.Modified), bytesOf(s.Versions)}
	var sections [sectionCount][][]byte
	for i, data := range columns {
		sections[i] = [][]byte{data}
	}
	sections[queryColumnSection] = s.queryColumnParts()
	sections[childRowsSection] = [][]byte{bytesOf(s.ChildRows)}
	header = make([]byte, headerSize)
	copy(header, magic)
	binary.LittleEndian.PutUint32(header[8:], 3)
	binary.LittleEndian.PutUint32(header[12:], sectionCount)
	offset := uint64(headerSize)
	for i, section := range sections {
		var length uint64
		checksum := crc32.NewIEEE()
		for _, part := range section {
			length += uint64(len(part))
			_, _ = checksum.Write(part)
		}
		descriptor := header[32+i*24:]
		binary.LittleEndian.PutUint64(descriptor, offset)
		binary.LittleEndian.PutUint64(descriptor[8:], length)
		binary.LittleEndian.PutUint32(descriptor[16:], checksum.Sum32())
		offset += (length + 7) &^ 7
	}
	binary.LittleEndian.PutUint64(header[16:], offset)
	binary.LittleEndian.PutUint32(header[24:], crc32.ChecksumIEEE(header[32:]))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	var padding [8]byte
	for _, section := range sections {
		var length uint64
		for _, part := range section {
			length += uint64(len(part))
			if len(part) != 0 {
				if _, err = file.Write(part); err != nil {
					return nil, err
				}
			}
		}
		if _, err = file.Write(padding[:(-length)&7]); err != nil {
			return nil, err
		}
	}
	return header, file.Sync()
}

// Open verifies the complete file before lending typed views into the read-only mapping.
func Open(path string) (*Snapshot, error) {
	return openSnapshot(path, nil)
}

// openSnapshot fully validates existing files. A freshly written file may reuse
// the writer's structural validation only when its header and every checksummed
// section match the writer's output; the file itself cannot assert this trust.
func openSnapshot(path string, writtenHeader []byte) (*Snapshot, error) {
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
	if writtenHeader != nil && !bytes.Equal(header, writtenHeader) {
		return nil, fmt.Errorf("written column header changed")
	}
	version := binary.LittleEndian.Uint32(header[8:])
	fileSections := int(binary.LittleEndian.Uint32(header[12:]))
	if string(header[:8]) != magic || version < 1 || version > 4 || fileSections != sectionCount && fileSections != legacySectionCount || binary.LittleEndian.Uint64(header[16:]) != uint64(stat.Size()) || binary.LittleEndian.Uint32(header[24:]) != crc32.ChecksumIEEE(header[32:]) {
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
	var verifyBuffer []byte
	if writtenHeader != nil {
		verifyBuffer = make([]byte, 64*1024)
	}
	sections := make([][]byte, fileSections)
	offset := uint64(headerSize)
	for i := range sections {
		descriptor := header[32+i*24:]
		start, length := binary.LittleEndian.Uint64(descriptor), binary.LittleEndian.Uint64(descriptor[8:])
		if start != offset || start > uint64(len(mapping.data)) || length > uint64(len(mapping.data))-start {
			return nil, fmt.Errorf("invalid column section bounds")
		}
		sections[i] = mapping.data[start : start+length]
		var checksum uint32
		// Keep the name dictionary, posting offsets and query filters warm for
		// the first filename query. Per-entry columns stay demand-paged: their
		// structural validation already ran on the writer's heap snapshot.
		nameColumns := i >= 5 && i <= 9 || i == queryColumnSection
		if writtenHeader != nil && !nameColumns {
			hash := crc32.NewIEEE()
			n, err := io.CopyBuffer(hash, io.NewSectionReader(file, int64(start), int64(length)), verifyBuffer)
			if err != nil {
				return nil, err
			}
			if uint64(n) != length {
				return nil, fmt.Errorf("truncated column section")
			}
			checksum = hash.Sum32()
		} else {
			checksum = crc32.ChecksumIEEE(sections[i])
		}
		if checksum != binary.LittleEndian.Uint32(descriptor[16:]) {
			return nil, fmt.Errorf("column checksum mismatch")
		}
		offset = start + ((length + 7) &^ 7)
	}
	if offset != uint64(stat.Size()) {
		return nil, fmt.Errorf("invalid column trailing data")
	}
	widths := []int{1, 8, 4, 4, 1, 4, 1, 8, 1, 4, 4, 4, 8, 8, 8, 8, 4}
	for i, data := range sections {
		if len(data)%widths[i] != 0 {
			return nil, fmt.Errorf("misaligned column length")
		}
	}
	s := &Snapshot{Root: string(sections[0]), IDs: view[uint64](sections[1]), Parents: view[uint32](sections[2]), NameIDs: view[uint32](sections[3]), Flags: sections[4], NameOffsets: view[uint32](sections[5]), NameBytes: sections[6], NameMasks: view[uint64](sections[7]), NameFlags: sections[8], NameStarts: view[uint32](sections[9]), NameRows: view[uint32](sections[10]), IDRows: view[uint32](sections[11]), Sizes: view[int64](sections[12]), Modified: view[int64](sections[13]), Versions: view[uint64](sections[14]), NameRowsPathSorted: binary.LittleEndian.Uint32(header[8:]) >= 2, mapping: mapping}
	if fileSections > childRowsSection {
		s.ChildRows = view[uint32](sections[childRowsSection])
	} else {
		s.prepareChildRows()
	}
	if writtenHeader == nil {
		if err := s.Validate(); err != nil {
			return nil, err
		}
	}
	if version >= 3 {
		if err := s.mapQueryColumns(sections[queryColumnSection], version); err != nil {
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
