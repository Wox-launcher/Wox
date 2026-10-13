// Package directoryread owns filesystem enumeration for the file-search feature.
package directoryread

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const (
	bulkName       = 0x00000001
	bulkObjectType = 0x00000008
	bulkModified   = 0x00000400
	bulkAccess     = 0x00020000
	bulkError      = 0x20000000
	bulkReturned   = 0x80000000
	bulkDataLength = 0x00000200
	// Packed common attributes. ATTR_FILE_DATALENGTH is not included: Darwin
	// omits file attributes for directories instead of reserving that slot.
	bulkCommonBytes = 60
	bulkFileBytes   = 8
)

type bulkEntry struct {
	name, parent  string
	mode          os.FileMode
	size          int64
	modified      time.Time
	metadataKnown bool
	err           error
}

func (e *bulkEntry) Name() string      { return e.name }
func (e *bulkEntry) IsDir() bool       { return e.mode.IsDir() }
func (e *bulkEntry) Type() os.FileMode { return e.mode.Type() }

// Info only stats entries for which the filesystem omitted requested metadata.
// Lstat preserves the scanner's link-entry semantics and avoids link cycles.
func (e *bulkEntry) Info() (os.FileInfo, error) {
	if e.err != nil {
		return nil, e.err
	}
	if !e.metadataKnown {
		return os.Lstat(filepath.Join(e.parent, e.name))
	}
	return bulkInfo{e}, nil
}

type bulkInfo struct{ *bulkEntry }

func (i bulkInfo) Size() int64        { return i.size }
func (i bulkInfo) Mode() os.FileMode  { return i.mode }
func (i bulkInfo) ModTime() time.Time { return i.modified }
func (i bulkInfo) Sys() any           { return nil }

// decodeBulkEntries reads Darwin's packed attribute fields without casting
// potentially unaligned native structs. It validates each record and relative
// name range before retaining owned names across batches.
func decodeBulkEntries(parent string, buffer []byte, count int) ([]os.DirEntry, error) {
	if count < 0 || count > len(buffer)/bulkCommonBytes {
		return nil, fmt.Errorf("invalid bulk entry count %d", count)
	}
	entries := make([]os.DirEntry, 0, count)
	for index := 0; index < count; index++ {
		if len(buffer) < bulkCommonBytes {
			return nil, fmt.Errorf("truncated bulk entry")
		}
		length := int(binary.LittleEndian.Uint32(buffer[:4]))
		if length < bulkCommonBytes || length > len(buffer) {
			return nil, fmt.Errorf("invalid bulk entry length %d", length)
		}
		record := buffer[:length]
		buffer = buffer[length:]
		common := binary.LittleEndian.Uint32(record[4:8])
		file := binary.LittleEndian.Uint32(record[16:20])
		if common&(bulkName|bulkReturned) != bulkName|bulkReturned {
			return nil, fmt.Errorf("bulk entry omitted its name")
		}
		nameStart := int64(28) + int64(int32(binary.LittleEndian.Uint32(record[28:32])))
		nameLength := int64(binary.LittleEndian.Uint32(record[32:36]))
		// FSOPT_PACK_INVAL_ATTRS keeps common attributes at fixed offsets, but
		// directories do not get the file-length slot. Their names therefore
		// start at byte 60. Treating 68 as the minimum rejected those entries.
		nameMin := int64(bulkCommonBytes)
		hasFileLength := file&bulkDataLength != 0
		if hasFileLength {
			nameMin += bulkFileBytes
		}
		if nameStart < nameMin || nameLength < 2 || nameStart+nameLength > int64(length) {
			return nil, fmt.Errorf("invalid bulk name range")
		}
		nameBytes := record[nameStart : nameStart+nameLength]
		if nameBytes[len(nameBytes)-1] != 0 || bytes.IndexByte(nameBytes[:len(nameBytes)-1], 0) >= 0 || bytes.IndexByte(nameBytes, '/') >= 0 {
			return nil, fmt.Errorf("invalid bulk name")
		}
		name := string(nameBytes[:len(nameBytes)-1])
		if name == "." || name == ".." {
			continue
		}
		entry := &bulkEntry{name: name, parent: parent}
		if common&bulkError != 0 {
			if errno := binary.LittleEndian.Uint32(record[24:28]); errno != 0 {
				entry.err = syscall.Errno(errno)
			}
		}
		kind := binary.LittleEndian.Uint32(record[36:40])
		if common&bulkObjectType != 0 {
			switch kind {
			case 1:
			case 2:
				entry.mode = os.ModeDir
			case 3:
				entry.mode = os.ModeDevice
			case 4:
				entry.mode = os.ModeDevice | os.ModeCharDevice
			case 5:
				entry.mode = os.ModeSymlink
			case 6:
				entry.mode = os.ModeSocket
			case 7:
				entry.mode = os.ModeNamedPipe
			default:
				common &^= bulkObjectType
			}
		}
		access := binary.LittleEndian.Uint32(record[56:60])
		entry.mode |= os.FileMode(access & 0777)
		if access&04000 != 0 {
			entry.mode |= os.ModeSetuid
		}
		if access&02000 != 0 {
			entry.mode |= os.ModeSetgid
		}
		if access&01000 != 0 {
			entry.mode |= os.ModeSticky
		}
		seconds := int64(binary.LittleEndian.Uint64(record[40:48]))
		nanos := int64(binary.LittleEndian.Uint64(record[48:56]))
		entry.modified = time.Unix(seconds, nanos)
		if hasFileLength {
			entry.size = int64(binary.LittleEndian.Uint64(record[bulkCommonBytes : bulkCommonBytes+bulkFileBytes]))
		}
		required := uint32(bulkObjectType | bulkModified | bulkAccess)
		entry.metadataKnown = common&required == required && hasFileLength && nanos >= 0 && nanos < 1e9 && entry.size >= 0
		entries = append(entries, entry)
	}
	return entries, nil
}
