package imageoverlay

/*
#cgo CFLAGS: -DUNICODE -D_UNICODE
#cgo LDFLAGS: -lgdi32 -luser32 -lole32 -lwindowscodecs -lmsimg32
#include <stdlib.h>
#include <stdbool.h>

void* ImageOverlayCreateWindow(char* name, unsigned char* imageData, int imageLen, char* imageFilePath, float width, float height, float cornerRadius, bool closable);
void ImageOverlayDestroyWindow(void* hwnd);
*/
import "C"
import (
	"unsafe"

	"wox/util/overlay"
)

func newImageRenderer(id string, source overlayImage, width float64, height float64, cornerRadius float64, closable bool) (*imageRenderer, bool) {
	cName := C.CString(id)
	defer C.free(unsafe.Pointer(cName))

	var cImageData *C.uchar
	var cImageLen C.int
	var cImageFilePath *C.char
	var pngBytes []byte

	switch source.kind {
	case overlayImageKindFile:
		if source.filePath != "" {
			cImageFilePath = C.CString(source.filePath)
			defer C.free(unsafe.Pointer(cImageFilePath))
		}
	case overlayImageKindImage:
		pngBytes, _ = imageToPNG(source.image)
		if len(pngBytes) > 0 {
			cImageData = (*C.uchar)(unsafe.Pointer(&pngBytes[0]))
			cImageLen = C.int(len(pngBytes))
		}
	}

	handle := C.ImageOverlayCreateWindow(cName, cImageData, cImageLen, cImageFilePath, C.float(width), C.float(height), C.float(cornerRadius), C.bool(closable))
	if handle == nil {
		return nil, false
	}
	return &imageRenderer{handle: uintptr(handle), width: width, height: height}, true
}

func (renderer *imageRenderer) nativeAttachment() overlay.NativeAttachment {
	if renderer == nil || renderer.handle == 0 {
		return overlay.NativeAttachment{}
	}
	return overlay.NativeAttachment{
		Kind:   overlay.NativeAttachmentKindWindow,
		Handle: renderer.handle,
		Width:  renderer.width,
		Height: renderer.height,
	}
}

func (renderer *imageRenderer) destroy() {
	if renderer == nil || renderer.handle == 0 {
		return
	}
	C.ImageOverlayDestroyWindow(unsafe.Pointer(renderer.handle))
	renderer.handle = 0
}
