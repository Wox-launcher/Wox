package notifier

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>

void showNotification(const char* message);
void showNotificationWithIcon(const char* message, const unsigned char* bgra, int width, int height);
*/
import "C"
import (
	"image"
	"unsafe"

	"golang.design/x/hotkey/mainthread"
)

func ShowNotification(icon image.Image, message string) {
	if message == "" {
		return
	}

	mainthread.Call(func() {
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
	})
}
