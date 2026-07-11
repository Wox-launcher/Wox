package dictationoverlay

/*
#cgo CFLAGS: -DUNICODE -D_UNICODE
#cgo LDFLAGS: -lgdi32 -luser32 -luxtheme
#include <stdbool.h>
#include <stdlib.h>

void* DictationOverlayCreateWindow(char* name, bool closable);
void DictationOverlaySetActive(void* hwnd, bool active);
void DictationOverlayDestroyWindow(void* hwnd);
*/
import "C"
import (
	"unsafe"

	"wox/util/overlay"
)

func newDictationOverlayRenderer(id string, closable bool) (*dictationOverlayRenderer, bool) {
	cName := C.CString(id)
	defer C.free(unsafe.Pointer(cName))
	handle := C.DictationOverlayCreateWindow(cName, C.bool(closable))
	if handle == nil {
		return nil, false
	}
	return &dictationOverlayRenderer{handle: uintptr(handle)}, true
}

func (renderer *dictationOverlayRenderer) nativeAttachment() overlay.NativeAttachment {
	if renderer == nil || renderer.handle == 0 {
		return overlay.NativeAttachment{}
	}
	return overlay.NativeAttachment{
		Kind:   overlay.NativeAttachmentKindWindow,
		Handle: renderer.handle,
		Width:  dictationOverlayWidth,
		Height: dictationOverlayContentHeight,
	}
}

func (renderer *dictationOverlayRenderer) setActive(active bool) {
	if renderer == nil || renderer.handle == 0 {
		return
	}
	C.DictationOverlaySetActive(unsafe.Pointer(renderer.handle), C.bool(active))
}

func (renderer *dictationOverlayRenderer) destroy() {
	if renderer == nil || renderer.handle == 0 {
		return
	}
	C.DictationOverlayDestroyWindow(unsafe.Pointer(renderer.handle))
	renderer.handle = 0
}
