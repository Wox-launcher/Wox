package fileicon

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework Cocoa
#include <stdlib.h>

const unsigned char *GetFileTypeIconBytes(const char *ext, size_t *length);
*/
import "C"

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"os"
	"unsafe"

	"github.com/disintegration/imaging"
)

func getFileTypeIconImpl(ctx context.Context, ext string) (string, error) {
	const size = 48
	cachePath := buildCachePath(ext, size)

	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}

	cext := C.CString(ext)
	defer C.free(unsafe.Pointer(cext))

	var length C.size_t
	bytesPtr := C.GetFileTypeIconBytes(cext, &length)
	if bytesPtr == nil || length == 0 {
		return "", errors.New("no icon")
	}
	defer C.free(unsafe.Pointer(bytesPtr))

	data := C.GoBytes(unsafe.Pointer(bytesPtr), C.int(length))
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return "", err
	}

	if err := imaging.Save(img, cachePath); err != nil {
		return "", err
	}

	return cachePath, nil
}
