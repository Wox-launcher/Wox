//go:build windows

package column

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"unsafe"
)

// releasePages evicts only this clean file mapping, never the process heap.
// VirtualUnlock documents ERROR_NOT_LOCKED as the expected result for this use.
func (m *fileMapping) releasePages() error {
	if len(m.data) == 0 {
		return nil
	}
	err := windows.VirtualUnlock(uintptr(unsafe.Pointer(&m.data[0])), uintptr(len(m.data)))
	if errors.Is(err, windows.ERROR_NOT_LOCKED) {
		return nil
	}
	return err
}

type fileMapping struct{ data []byte }

// mapFile keeps clean snapshot columns outside the Go heap; the section owns its file reference.
func mapFile(file *os.File, size int) (*fileMapping, error) {
	section, err := windows.CreateFileMapping(windows.Handle(file.Fd()), nil, windows.PAGE_READONLY, 0, 0, nil)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(section)
	address, err := windows.MapViewOfFile(section, windows.FILE_MAP_READ, 0, 0, uintptr(size))
	if err != nil {
		return nil, err
	}
	return &fileMapping{data: unsafe.Slice((*byte)(unsafe.Pointer(address)), size)}, nil
}

func (m *fileMapping) close() error {
	if len(m.data) == 0 {
		return nil
	}
	err := windows.UnmapViewOfFile(uintptr(unsafe.Pointer(&m.data[0])))
	m.data = nil
	return err
}
