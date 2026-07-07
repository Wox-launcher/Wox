package dictationoverlay

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include <stdbool.h>
#include <stdlib.h>

void* DictationOverlayCreateView(char* name, bool closable);
void DictationOverlaySetActive(void* view, bool active);
void DictationOverlayDestroyView(void* view);
*/
import "C"
import (
	"unsafe"

	"wox/util/mainthread"
	"wox/util/overlay"
)

func newDictationOverlayRenderer(id string, closable bool) (*dictationOverlayRenderer, bool) {
	var handle unsafe.Pointer
	mainthread.Call(func() {
		cName := C.CString(id)
		defer C.free(unsafe.Pointer(cName))
		handle = C.DictationOverlayCreateView(cName, C.bool(closable))
	})
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
		Kind:   overlay.NativeAttachmentKindView,
		Handle: renderer.handle,
		Width:  dictationOverlayWidth,
		Height: dictationOverlayContentHeight,
	}
}

func (renderer *dictationOverlayRenderer) setActive(active bool) {
	if renderer == nil || renderer.handle == 0 {
		return
	}
	mainthread.Call(func() {
		if renderer.handle != 0 {
			C.DictationOverlaySetActive(unsafe.Pointer(renderer.handle), C.bool(active))
		}
	})
}

func (renderer *dictationOverlayRenderer) destroy() {
	if renderer == nil || renderer.handle == 0 {
		return
	}
	mainthread.Call(func() {
		if renderer.handle != 0 {
			C.DictationOverlayDestroyView(unsafe.Pointer(renderer.handle))
		}
	})
	renderer.handle = 0
}
