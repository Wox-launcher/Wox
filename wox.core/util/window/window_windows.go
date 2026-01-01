package window

/*
#cgo LDFLAGS: -lpsapi -lgdi32 -luser32 -lshell32
#include <windows.h>
#include <psapi.h>
#include <shellapi.h>

char* getActiveWindowIcon(unsigned char **iconData, int *iconSize, int *width, int *height);
char* getActiveWindowName();
int getActiveWindowPid();
int activateWindowByPid(int pid);
int isOpenSaveDialog();
*/
import "C"
import (
	"fmt"
	"image"
	"image/color"
	"unsafe"
)

func GetActiveWindowIcon() (image.Image, error) {
	var iconData *C.uchar
	var iconSize C.int
	var width, height C.int

	errMsgC := C.getActiveWindowIcon(&iconData, &iconSize, &width, &height)
	if errMsgC != nil {
		errMsg := C.GoString(errMsgC)
		return nil, fmt.Errorf("failed to get active window icon: %s", errMsg)
	}
	defer C.free(unsafe.Pointer(iconData))

	data := C.GoBytes(unsafe.Pointer(iconData), iconSize)
	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))

	idx := 0
	for y := 0; y < int(height); y++ {
		for x := 0; x < int(width); x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: data[idx+2],
				G: data[idx+1],
				B: data[idx],
				A: data[idx+3],
			})
			idx += 4
		}
	}

	return img, nil
}

func GetActiveWindowName() string {
	cStr := C.getActiveWindowName()
	if cStr == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(cStr))
	length := C.int(C.strlen(cStr))
	bytes := C.GoBytes(unsafe.Pointer(cStr), length)
	return string(bytes)
}

func GetActiveWindowPid() int {
	pid := C.getActiveWindowPid()
	return int(pid)
}

func ActivateWindowByPid(pid int) bool {
	result := C.activateWindowByPid(C.int(pid))
	return int(result) == 1
}

func IsOpenSaveDialog() (bool, error) {
	result := C.isOpenSaveDialog()
	return int(result) == 1, nil
}
