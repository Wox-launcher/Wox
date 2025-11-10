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
	"wox/common"

	"github.com/disintegration/imaging"
)

func getFileTypeIconImpl(ctx context.Context, ext string) (common.WoxImage, error) {
	const size = 48
	cachePath := buildCachePath(ext, size)

	if _, err := os.Stat(cachePath); err == nil {
		return common.NewWoxImageAbsolutePath(cachePath), nil
	}

	cext := C.CString(ext)
	defer C.free(unsafe.Pointer(cext))

	var length C.size_t
	bytesPtr := C.GetFileTypeIconBytes(cext, &length)
	if bytesPtr == nil || length == 0 {
		return common.WoxImage{}, errors.New("no icon")
	}
	defer C.free(unsafe.Pointer(bytesPtr))

	data := C.GoBytes(unsafe.Pointer(bytesPtr), C.int(length))
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return common.WoxImage{}, err
	}

	if err := imaging.Save(img, cachePath); err != nil {
		// Fallback to embed base64
		wimg, _ := common.NewWoxImage(img)
		return wimg, nil
	}
	return common.NewWoxImageAbsolutePath(cachePath), nil
}
