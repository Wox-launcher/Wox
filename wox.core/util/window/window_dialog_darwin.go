//go:build darwin

package window

/*
#cgo LDFLAGS: -framework Foundation -framework Cocoa -framework ApplicationServices
#include <stdlib.h>
int isOpenSaveDialog();
int navigateActiveFileDialog(const char* path);
*/
import "C"
import (
	"unsafe"
)

func IsOpenSaveDialog() (bool, error) {
	return int(C.isOpenSaveDialog()) == 1, nil
}

func NavigateActiveFileDialog(targetPath string) bool {
	if targetPath == "" {
		return false
	}

	cPath := C.CString(targetPath)
	defer C.free(unsafe.Pointer(cPath))
	return int(C.navigateActiveFileDialog(cPath)) == 1
}
