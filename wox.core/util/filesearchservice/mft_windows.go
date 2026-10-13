//go:build windows

package filesearchservice

import (
	"encoding/binary"
	"fmt"
	"golang.org/x/sys/windows"
	"unicode/utf16"
)

const (
	fsctlEnumUSNData  = 0x000900b3
	mftReadBufferSize = 1 << 20
	ntfsRootRecord    = 5
)

type mftEnumData struct {
	StartFileReferenceNumber uint64
	LowUSN                   int64
	HighUSN                  int64
}

type mftNode struct {
	Reference       uint64
	ParentReference uint64
	Name            string
	IsDir           bool
	USN             int64
	Reason          uint32
}

// parseUSNRecords validates native offsets before converting one bounded read.
func parseUSNRecords(buffer []byte) (uint64, []mftNode, error) {
	if len(buffer) < 8 {
		return 0, nil, fmt.Errorf("invalid MFT response length %d", len(buffer))
	}
	next := binary.LittleEndian.Uint64(buffer[:8])
	var nodes []mftNode
	if len(buffer) > 8 {
		nodes = make([]mftNode, 0, 1024)
	}
	for offset := 8; offset < len(buffer); {
		if len(buffer)-offset < 60 {
			return 0, nil, fmt.Errorf("truncated USN record at offset %d", offset)
		}
		record := buffer[offset:]
		recordLength := int(binary.LittleEndian.Uint32(record[:4]))
		if recordLength < 60 || recordLength > len(record) {
			return 0, nil, fmt.Errorf("invalid USN record length %d", recordLength)
		}
		if binary.LittleEndian.Uint16(record[4:6]) != 2 {
			return 0, nil, fmt.Errorf("unsupported USN record version %d", binary.LittleEndian.Uint16(record[4:6]))
		}
		nameLength := int(binary.LittleEndian.Uint16(record[56:58]))
		nameOffset := int(binary.LittleEndian.Uint16(record[58:60]))
		if nameLength%2 != 0 || nameOffset < 60 || nameOffset+nameLength > recordLength {
			return 0, nil, fmt.Errorf("invalid USN file name range %d:%d", nameOffset, nameLength)
		}
		nameWords := make([]uint16, nameLength/2)
		for index := range nameWords {
			nameWords[index] = binary.LittleEndian.Uint16(record[nameOffset+index*2:])
		}
		nodes = append(nodes, mftNode{
			Reference:       binary.LittleEndian.Uint64(record[8:16]),
			ParentReference: binary.LittleEndian.Uint64(record[16:24]),
			Name:            string(utf16.Decode(nameWords)),
			IsDir:           binary.LittleEndian.Uint32(record[52:56])&windows.FILE_ATTRIBUTE_DIRECTORY != 0,
			USN:             int64(binary.LittleEndian.Uint64(record[24:32])),
			Reason:          binary.LittleEndian.Uint32(record[40:44]),
		})
		offset += recordLength
	}
	return next, nodes, nil
}
