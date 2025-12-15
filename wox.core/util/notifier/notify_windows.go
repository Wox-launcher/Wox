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

const notificationIconSize = 32

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

func iconToBGRA(src image.Image, size int) ([]byte, int, int) {
	if src == nil || size <= 0 {
		return nil, 0, 0
	}

	b := src.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return nil, 0, 0
	}

	out := make([]byte, size*size*4)
	srcW := b.Dx()
	srcH := b.Dy()

	for y := 0; y < size; y++ {
		sy := b.Min.Y + (y*srcH)/size
		for x := 0; x < size; x++ {
			sx := b.Min.X + (x*srcW)/size
			r, g, bl, a := src.At(sx, sy).RGBA()
			i := (y*size + x) * 4
			out[i+0] = uint8(bl >> 8) // B
			out[i+1] = uint8(g >> 8)  // G
			out[i+2] = uint8(r >> 8)  // R
			out[i+3] = uint8(a >> 8)  // A
		}
	}

	return out, size, size
}
