package clipboard

/*
#include "clipboard_windows.c"
*/
import "C"
import (
	"image"
	"unsafe"
)

func readText() (string, error) {
	text := C.GetClipboardText()
	if text != nil {
		defer C.free(unsafe.Pointer(text))
		return C.GoString(text), nil
	}

	return "", noDataErr
}

func readFilePaths() ([]string, error) {
	return nil, notImplement
}

func readImage() (image.Image, error) {
	return nil, notImplement
}

func writeTextData(text string) error {
	return notImplement
}
