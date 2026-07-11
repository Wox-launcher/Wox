package imageoverlay

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
#include <stdbool.h>

void* ImageOverlayCreateView(char* name, unsigned char* imageData, int imageLen, char* imageFilePath, float cornerRadius, bool closable);
void ImageOverlayDestroyView(void* view);
*/
import "C"
import (
	"unsafe"

	"wox/util/mainthread"
	"wox/util/overlay"
)

func newImageRenderer(id string, source overlayImage, width float64, height float64, cornerRadius float64, closable bool) (*imageRenderer, bool) {
	var handle unsafe.Pointer
	mainthread.Call(func() {
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

		handle = C.ImageOverlayCreateView(cName, cImageData, cImageLen, cImageFilePath, C.float(cornerRadius), C.bool(closable))
	})
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
		Kind:   overlay.NativeAttachmentKindView,
		Handle: renderer.handle,
		Width:  renderer.width,
		Height: renderer.height,
	}
}

func (renderer *imageRenderer) destroy() {
	if renderer == nil || renderer.handle == 0 {
		return
	}
	mainthread.Call(func() {
		if renderer.handle != 0 {
			C.ImageOverlayDestroyView(unsafe.Pointer(renderer.handle))
		}
	})
	renderer.handle = 0
}
