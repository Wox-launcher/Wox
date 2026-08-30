//go:build windows

package filesearchservice

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// mappedColumnFile owns the file-backed address range used by one volume.
type mappedColumnFile struct {
	address unsafe.Pointer
	file    *os.File
	path    string
}

// secureIndexDirectory pins the validated non-reparse cache directory so all
// privileged writes can use relative handles without resolving user paths again.
type secureIndexDirectory struct {
	path   string
	handle windows.Handle
}

type mappedSnapshotDirectory struct {
	handle windows.Handle
}

// writeMappedVolumeColumn writes the immutable base columns once and returns
// the same column view backed by a read-only Windows file mapping.
func writeMappedVolumeColumn(path string, source *volumeColumn) (*volumeColumn, error) {
	file, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	mapped, err := writeMappedVolumeColumnFile(file, source, path)
	if err != nil {
		file.Close()
		_ = os.Remove(path)
		return nil, err
	}
	return mapped, nil
}

func writeMappedVolumeColumnFile(file *os.File, source *volumeColumn, path string) (*volumeColumn, error) {
	if err := writeVolumeColumns(file, source); err != nil {
		return nil, err
	}
	storage, err := mapColumnFile(file, path)
	if err != nil {
		return nil, err
	}
	offset := uintptr(0)
	mapped := &volumeColumn{
		root:          source.root,
		references:    mappedSlice[uint64](storage.address, &offset, len(source.references)),
		nameMasks:     mappedSlice[uint64](storage.address, &offset, len(source.nameMasks)),
		parentRows:    mappedSlice[uint32](storage.address, &offset, len(source.parentRows)),
		nameIDs:       mappedSlice[uint32](storage.address, &offset, len(source.nameIDs)),
		nameOffsets:   mappedSlice[uint32](storage.address, &offset, len(source.nameOffsets)),
		nameRowStarts: mappedSlice[uint32](storage.address, &offset, len(source.nameRowStarts)),
		nameRows:      mappedSlice[uint32](storage.address, &offset, len(source.nameRows)),
		referenceRows: mappedSlice[uint32](storage.address, &offset, len(source.referenceRows)),
		rowFlags:      mappedSlice[byte](storage.address, &offset, len(source.rowFlags)),
		nameBytes:     mappedSlice[byte](storage.address, &offset, len(source.nameBytes)),
		nameFlags:     mappedSlice[byte](storage.address, &offset, len(source.nameFlags)),
		journalID:     source.journalID,
		nextUSN:       source.nextUSN,
		delta:         make(map[uint64]deltaNode),
		fileCount:     source.fileCount,
		storage:       storage,
	}
	return mapped, nil
}

func writeVolumeColumns(file *os.File, source *volumeColumn) error {
	columns := [][]byte{
		sliceBytes(source.references),
		sliceBytes(source.nameMasks),
		sliceBytes(source.parentRows),
		sliceBytes(source.nameIDs),
		sliceBytes(source.nameOffsets),
		sliceBytes(source.nameRowStarts),
		sliceBytes(source.nameRows),
		sliceBytes(source.referenceRows),
		source.rowFlags,
		source.nameBytes,
		source.nameFlags,
	}
	for _, column := range columns {
		for len(column) > 0 {
			written, err := file.Write(column)
			if err != nil {
				return err
			}
			if written == 0 {
				return fmt.Errorf("write file index snapshot: no progress")
			}
			column = column[written:]
		}
	}
	return nil
}

func mapColumnFile(file *os.File, path string) (*mappedColumnFile, error) {
	mapping, err := windows.CreateFileMapping(windows.Handle(file.Fd()), nil, windows.PAGE_READONLY, 0, 0, nil)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(mapping)
	address, err := windows.MapViewOfFile(mapping, windows.FILE_MAP_READ, 0, 0, 0)
	runtime.KeepAlive(file)
	if err != nil {
		return nil, err
	}
	return &mappedColumnFile{address: unsafe.Pointer(address), file: file, path: path}, nil
}

func (m *mappedColumnFile) close() error {
	if m == nil || m.address == nil {
		return nil
	}
	err := windows.UnmapViewOfFile(uintptr(m.address))
	m.address = nil
	if m.file != nil {
		if closeErr := m.file.Close(); err == nil {
			err = closeErr
		}
		m.file = nil
	}
	if m.path != "" {
		removeErr := os.Remove(m.path)
		if err == nil && !os.IsNotExist(removeErr) {
			err = removeErr
		}
	}
	return err
}

func openSecureIndexDirectory(ownerSID string, indexPath string) (*secureIndexDirectory, error) {
	if err := validateIndexDirectoryPath(ownerSID, indexPath); err != nil {
		return nil, err
	}
	pathPtr, err := windows.UTF16PtrFromString(filepath.Clean(indexPath))
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(pathPtr, windows.FILE_GENERIC_READ|windows.FILE_GENERIC_WRITE|windows.DELETE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, fmt.Errorf("open file index directory: %w", err)
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		windows.CloseHandle(handle)
		return nil, err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		windows.CloseHandle(handle)
		return nil, fmt.Errorf("file index directory must not be a reparse point")
	}
	profilePath, err := profilePathForSID(ownerSID)
	if err != nil {
		windows.CloseHandle(handle)
		return nil, err
	}
	profileHandle, err := openDirectoryHandle(profilePath)
	if err != nil {
		windows.CloseHandle(handle)
		return nil, err
	}
	profileFinal, profileErr := finalPathByHandle(profileHandle)
	windows.CloseHandle(profileHandle)
	indexFinal, indexErr := finalPathByHandle(handle)
	if profileErr != nil || indexErr != nil || !strings.EqualFold(filepath.Clean(indexFinal), filepath.Join(filepath.Clean(profileFinal), ".wox", "filesearch", IndexDirectory)) {
		windows.CloseHandle(handle)
		return nil, fmt.Errorf("file index directory resolves outside the owner profile")
	}
	if err := setOwnerSIDACL(indexPath, ownerSID, true); err != nil {
		windows.CloseHandle(handle)
		return nil, err
	}
	return &secureIndexDirectory{path: filepath.Clean(indexPath), handle: handle}, nil
}

func openDirectoryHandle(path string) (windows.Handle, error) {
	pathPtr, err := windows.UTF16PtrFromString(filepath.Clean(path))
	if err != nil {
		return 0, err
	}
	return windows.CreateFile(pathPtr, windows.FILE_GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
}

func finalPathByHandle(handle windows.Handle) (string, error) {
	buffer := make([]uint16, windows.MAX_LONG_PATH)
	length, err := windows.GetFinalPathNameByHandle(handle, &buffer[0], uint32(len(buffer)), 0)
	if err != nil {
		return "", err
	}
	if length >= uint32(len(buffer)) {
		return "", fmt.Errorf("resolved file index path is too long")
	}
	return strings.TrimPrefix(windows.UTF16ToString(buffer[:length]), `\\?\`), nil
}

func (d *secureIndexDirectory) createSnapshotDirectory() (*mappedSnapshotDirectory, error) {
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return nil, err
	}
	handle, err := createRelativeFile(d.handle, "snapshot-"+hex.EncodeToString(random), true)
	if err != nil {
		return nil, err
	}
	return &mappedSnapshotDirectory{handle: handle}, nil
}

func (d *mappedSnapshotDirectory) createFile(name string) (*os.File, error) {
	handle, err := createRelativeFile(d.handle, name, false)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), name), nil
}

func createRelativeFile(parent windows.Handle, name string, directory bool) (windows.Handle, error) {
	objectName, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return 0, err
	}
	attributes := &windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: parent,
		ObjectName:    objectName,
		Attributes:    windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE,
	}
	options := uint32(windows.FILE_NON_DIRECTORY_FILE | windows.FILE_SYNCHRONOUS_IO_NONALERT | windows.FILE_DELETE_ON_CLOSE)
	if directory {
		options = windows.FILE_DIRECTORY_FILE | windows.FILE_SYNCHRONOUS_IO_NONALERT | windows.FILE_DELETE_ON_CLOSE
	}
	var handle windows.Handle
	var status windows.IO_STATUS_BLOCK
	var size int64
	err = windows.NtCreateFile(&handle, windows.FILE_GENERIC_READ|windows.FILE_GENERIC_WRITE|windows.DELETE, attributes, &status, &size, windows.FILE_ATTRIBUTE_NORMAL, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, windows.FILE_CREATE, options, 0, 0)
	if err != nil {
		return 0, err
	}
	return handle, nil
}

func (d *mappedSnapshotDirectory) close() error {
	if d == nil || d.handle == 0 {
		return nil
	}
	err := windows.CloseHandle(d.handle)
	d.handle = 0
	return err
}

func (d *secureIndexDirectory) close() error {
	if d == nil || d.handle == 0 {
		return nil
	}
	err := windows.CloseHandle(d.handle)
	d.handle = 0
	return err
}

func sliceBytes[T any](values []T) []byte {
	if len(values) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(&values[0])), len(values)*int(unsafe.Sizeof(values[0])))
}

func mappedSlice[T any](address unsafe.Pointer, offset *uintptr, length int) []T {
	if length == 0 {
		return nil
	}
	itemSize := unsafe.Sizeof(*new(T))
	result := unsafe.Slice((*T)(unsafe.Add(address, *offset)), length)
	*offset += uintptr(length) * itemSize
	return result
}
