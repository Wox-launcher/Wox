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
