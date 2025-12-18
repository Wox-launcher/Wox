package notifier

/*
#cgo LDFLAGS: -luser32 -lgdi32 -ldwmapi -luxtheme -lmsimg32
#include <stdlib.h>

void showNotification(const char* message);
void showNotificationWithIcon(const char* message, const unsigned char* bgra, int width, int height);
*/
import "C"
import (
	"image"
	"unsafe"
)

func ShowNotification(icon image.Image, message string) {
	cMessage := C.CString(message)
	defer C.free(unsafe.Pointer(cMessage))

	if icon == nil {
		C.showNotification(cMessage)
		return
	}

	bgra, w, h := iconToBGRA(icon, notificationIconSize)
	if len(bgra) == 0 || w == 0 || h == 0 {
		C.showNotification(cMessage)
		return
	}

	C.showNotificationWithIcon(cMessage, (*C.uchar)(unsafe.Pointer(&bgra[0])), C.int(w), C.int(h))
}
