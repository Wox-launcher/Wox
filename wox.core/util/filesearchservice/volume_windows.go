//go:build windows

package filesearchservice

import (
	"fmt"
	"golang.org/x/sys/windows"
	"strings"
)

const (
	fsctlQueryUSNJournal   = 0x000900f4
	fsctlReadUSNJournal    = 0x000900bb
	usnReadBufferSize      = 1 << 20
	usnReasonFileCreate    = 0x00000100
	usnReasonFileDelete    = 0x00000200
	usnReasonRenameOldName = 0x00001000
	usnReasonRenameNewName = 0x00002000
)

type usnJournalData struct {
	JournalID       uint64
	FirstUSN        int64
	NextUSN         int64
	LowestValidUSN  int64
	MaxUSN          int64
	MaximumSize     uint64
	AllocationDelta uint64
}

type readUSNJournalData struct {
	StartUSN          int64
	ReasonMask        uint32
	ReturnOnlyOnClose uint32
	Timeout           uint64
	BytesToWaitFor    uint64
	JournalID         uint64
	MinMajorVersion   uint16
	MaxMajorVersion   uint16
}

// fixedNTFSVolumeRoots returns the local fixed NTFS volumes owned by service mode.
func fixedNTFSVolumeRoots() ([]string, error) {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil, err
	}
	roots := make([]string, 0, 4)
	for drive := 0; drive < 26; drive++ {
		if mask&(uint32(1)<<drive) == 0 {
			continue
		}
		root := fmt.Sprintf("%c:\\", 'A'+drive)
		rootPtr, err := windows.UTF16PtrFromString(root)
		if err != nil || windows.GetDriveType(rootPtr) != windows.DRIVE_FIXED {
			continue
		}
		fileSystem := make([]uint16, 32)
		if err := windows.GetVolumeInformation(rootPtr, nil, 0, nil, nil, nil, &fileSystem[0], uint32(len(fileSystem))); err != nil {
			continue
		}
		if strings.EqualFold(windows.UTF16ToString(fileSystem), "NTFS") {
			roots = append(roots, root)
		}
	}
	return roots, nil
}
