package notifier

/*
#cgo LDFLAGS: -luser32 -lgdi32
#include <stdlib.h>

void showNotification(const char* title, const char* message);
*/
import "C"
import "unsafe"

func ShowNotification(title, message string) {
	cTitle := C.CString(title)
	cMessage := C.CString(message)
	defer C.free(unsafe.Pointer(cTitle))
	defer C.free(unsafe.Pointer(cMessage))

	C.showNotification(cTitle, cMessage)
}
