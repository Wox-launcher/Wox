package airdrop

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework AppKit
#include <stdlib.h>

void sendFilesViaAirDrop(const char **filePaths, int count);
*/
import "C"
import (
	"golang.design/x/hotkey/mainthread"
	"unsafe"
)

func Airdrop(filePaths []string) {
	mainthread.Call(func() {
		cFilePaths := make([]*C.char, len(filePaths))
		for i, path := range filePaths {
			cFilePaths[i] = C.CString(path)
		}

		C.sendFilesViaAirDrop((**C.char)(unsafe.Pointer(&cFilePaths[0])), C.int(len(filePaths)))

		for _, cStr := range cFilePaths {
			C.free(unsafe.Pointer(cStr))
		}
	})
}
