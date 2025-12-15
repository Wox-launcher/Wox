package notifier

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>

void showNotification(const char* message);
*/
import "C"
import (
	"image"
	"unsafe"

	"golang.design/x/hotkey/mainthread"
)

func ShowNotification(icon image.Image, message string) {
	_ = icon
	if message == "" {
		return
	}

	mainthread.Call(func() {
		cMessage := C.CString(message)
		defer C.free(unsafe.Pointer(cMessage))

		C.showNotification(cMessage)
	})
}
