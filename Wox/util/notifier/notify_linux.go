package notifier

/*
#cgo LDFLAGS: -lX11
#include <stdlib.h>

void showNotification(const char* message);
*/
import "C"
import "unsafe"

func ShowNotification(message string) {
	cMessage := C.CString(message)
	defer C.free(unsafe.Pointer(cMessage))

	C.showNotification(cMessage)
}
