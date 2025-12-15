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
	"image/color"
	"unsafe"

	"github.com/disintegration/imaging"
)

const notificationIconSize = 64

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

	resized := imaging.Resize(src, size, size, imaging.Lanczos)
	if resized == nil {
		return nil, 0, 0
	}

	b := resized.Bounds()
	if b.Dx() != size || b.Dy() != size {
		return nil, 0, 0
	}

	out := make([]byte, size*size*4)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			c := color.NRGBAModel.Convert(resized.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			a := uint32(c.A)
			r := uint32(c.R) * a / 255
			g := uint32(c.G) * a / 255
			bl := uint32(c.B) * a / 255

			i := (y*size + x) * 4
			out[i+0] = uint8(bl) // B (premultiplied)
			out[i+1] = uint8(g)  // G (premultiplied)
			out[i+2] = uint8(r)  // R (premultiplied)
			out[i+3] = c.A       // A
		}
	}

	return out, size, size
}
