package notifier

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>

void showNotification(const char* message);
*/
import "C"
import (
	"unsafe"
)

func ShowNotification(message string) {
	if message == "" {
		return
	}

	cMessage := C.CString(message)
	defer C.free(unsafe.Pointer(cMessage))

	C.showNotification(cMessage)
}
