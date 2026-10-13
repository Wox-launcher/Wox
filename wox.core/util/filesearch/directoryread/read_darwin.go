//go:build darwin && cgo

package directoryread

/*
#include <sys/attr.h>
#include <unistd.h>

// PACK_INVAL keeps common attributes at fixed offsets. File attributes are
// still omitted for directories; returned bitmaps decide which metadata is valid.
static int wox_read_directory_attributes(int fd, void *buffer, size_t length) {
    struct attrlist attributes = {0};
    attributes.bitmapcount = ATTR_BIT_MAP_COUNT;
    attributes.commonattr = ATTR_CMN_RETURNED_ATTRS | ATTR_CMN_ERROR |
        ATTR_CMN_NAME | ATTR_CMN_OBJTYPE | ATTR_CMN_MODTIME | ATTR_CMN_ACCESSMASK;
    attributes.fileattr = ATTR_FILE_DATALENGTH;
    return getattrlistbulk(fd, &attributes, buffer, length, FSOPT_PACK_INVAL_ATTRS);
}
*/
import "C"

import (
	"context"
	"errors"
	"os"
	"sort"
	"unsafe"

	"golang.org/x/sys/unix"
)

// ReadDir gathers names and metadata in bounded native batches. Filesystems
// without bulk attributes use a fresh ordinary directory read; IO/permission
// errors are returned, never converted into a successfully empty directory.
func ReadDir(ctx context.Context, path string) ([]os.DirEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	defer unix.Close(fd)
	buffer := make([]byte, 256<<10)
	var entries []os.DirEntry
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		count, err := C.wox_read_directory_attributes(C.int(fd), unsafe.Pointer(&buffer[0]), C.size_t(len(buffer)))
		if count < 0 {
			if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EINVAL) {
				return os.ReadDir(path)
			}
			return nil, &os.PathError{Op: "getattrlistbulk", Path: path, Err: err}
		}
		if count == 0 {
			break
		}
		batch, err := decodeBulkEntries(path, buffer, int(count))
		if err != nil {
			return nil, &os.PathError{Op: "getattrlistbulk", Path: path, Err: err}
		}
		entries = append(entries, batch...)
	}
	sort.Slice(entries, func(a, b int) bool { return entries[a].Name() < entries[b].Name() })
	return entries, nil
}
