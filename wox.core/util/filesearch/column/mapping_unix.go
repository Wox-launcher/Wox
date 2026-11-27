//go:build !windows

package column

import (
	"golang.org/x/sys/unix"
	"os"
)

type fileMapping struct{ data []byte }

// releasePages keeps the read-only mapping valid while discarding resident pages.
func (m *fileMapping) releasePages() error {
	if len(m.data) == 0 {
		return nil
	}
	return unix.Madvise(m.data, unix.MADV_DONTNEED)
}

func (m *fileMapping) releaseRange(start, end int) error {
	if m == nil || len(m.data) == 0 {
		return nil
	}
	if start < 0 {
		start = 0
	}
	if end > len(m.data) {
		end = len(m.data)
	}
	page := os.Getpagesize()
	if page <= 0 {
		page = 4096
	}
	alignedStart := (start + page - 1) &^ (page - 1)
	alignedEnd := end &^ (page - 1)
	if alignedStart >= alignedEnd {
		return nil
	}
	return unix.Madvise(m.data[alignedStart:alignedEnd], unix.MADV_DONTNEED)
}

func mapFile(file *os.File, size int) (*fileMapping, error) {
	data, err := unix.Mmap(int(file.Fd()), 0, size, unix.PROT_READ, unix.MAP_PRIVATE)
	if err != nil {
		return nil, err
	}
	return &fileMapping{data: data}, nil
}

func (m *fileMapping) close() error {
	if len(m.data) == 0 {
		return nil
	}
	err := unix.Munmap(m.data)
	m.data = nil
	return err
}
