package clipboard

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework Cocoa
#include <stdlib.h>

const char* GetClipboardText();
char* GetAllClipboardFilePaths();
unsigned char *GetClipboardImage(size_t *length);
void WriteClipboardText(const char *text);
_Bool hasClipboardChanged();
*/
import "C"
import (
	"bytes"
	"fmt"
	"github.com/samber/lo"
	"image"
	"strings"
	"unsafe"
)

func readText() (string, error) {
	text := C.GetClipboardText()
	if text != nil {
		return C.GoString(text), nil
	}

	return "", noDataErr
}

func readFilePaths() ([]string, error) {
	cstr := C.GetAllClipboardFilePaths()
	if cstr != nil {
		defer C.free(unsafe.Pointer(cstr))
		filePaths := strings.Split(C.GoString(cstr), "\n")
		filePaths = lo.Filter(filePaths, func(s string, _ int) bool {
			return s != ""
		})
		return filePaths, nil
	}

	return nil, noDataErr
}

func readImage() (image.Image, error) {
	var length C.size_t
	imageData := C.GetClipboardImage(&length)
	if imageData != nil {
		defer C.free(unsafe.Pointer(imageData))
		pngBytes := C.GoBytes(unsafe.Pointer(imageData), C.int(length))
		imgReader := bytes.NewReader(pngBytes)
		img, _, err := image.Decode(imgReader)
		if err != nil {
			return nil, fmt.Errorf("failed to decode image: %v", err)
		}
		return img, nil
	}

	return nil, noDataErr
}

func writeTextData(text string) error {
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))
	C.WriteClipboardText(cText)

	return nil
}

func isClipboardChanged() bool {
	return bool(C.hasClipboardChanged())
}
