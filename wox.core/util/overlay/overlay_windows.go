package overlay

/*
#cgo CFLAGS: -DUNICODE -D_UNICODE
#cgo LDFLAGS: -ldwmapi -luxtheme -lmsimg32 -lgdi32 -luser32 -lole32 -luuid -lwindowscodecs -lcomctl32
#include <stdlib.h>
#include <stdbool.h>

typedef struct {
    char* name;
    bool transparent;
    bool hitTestIconOnly;
    bool closeOnEscape;
    bool nativeAttachment;
    int nativeAttachmentKind;
    void* nativeAttachmentHandle;
    float nativeAttachmentWidth;
    float nativeAttachmentHeight;
    bool topmost;
    bool absolutePosition;
    bool preservePosition;
    int stickyWindowPid;
    int anchor;
    bool movable;
    bool resizable;
    float cornerRadius;
    float aspectRatio;
    float offsetX;
    float offsetY;
    float width;
    float minWidth;
    float maxWidth;
    float height;
    float maxHeight;
} OverlayOptions;

void ShowOverlay(OverlayOptions opts);
void CloseOverlay(char* name);
bool overlayClickCallbackCGO(char* name);
void overlayCloseCallbackCGO(char* name);
void overlayRequestCloseCallbackCGO(char* name);
*/
import "C"
import (
	"unsafe"
)

func showWindow(opts WindowOptions) {
	cName := C.CString(opts.ID)
	defer C.free(unsafe.Pointer(cName))

	cOpts := C.OverlayOptions{
		name:                   cName,
		transparent:            C.bool(opts.Transparent),
		hitTestIconOnly:        C.bool(opts.HitTestIconOnly),
		closeOnEscape:          C.bool(opts.CloseOnEscape),
		nativeAttachment:       C.bool(opts.NativeAttachment.active()),
		nativeAttachmentKind:   C.int(opts.NativeAttachment.Kind),
		nativeAttachmentHandle: unsafe.Pointer(opts.NativeAttachment.Handle),
		nativeAttachmentWidth:  C.float(opts.NativeAttachment.Width),
		nativeAttachmentHeight: C.float(opts.NativeAttachment.Height),
		topmost:                C.bool(opts.Topmost),
		absolutePosition:       C.bool(opts.AbsolutePosition),
		preservePosition:       C.bool(opts.PreservePosition),
		stickyWindowPid:        C.int(opts.StickyWindowPid),
		anchor:                 C.int(opts.Anchor),
		movable:                C.bool(opts.Movable),
		resizable:              C.bool(opts.Resizable),
		cornerRadius:           C.float(opts.CornerRadius),
		aspectRatio:            C.float(opts.AspectRatio),
		offsetX:                C.float(opts.OffsetX),
		offsetY:                C.float(opts.OffsetY),
		width:                  C.float(opts.Width),
		minWidth:               C.float(opts.MinWidth),
		maxWidth:               C.float(opts.MaxWidth),
		height:                 C.float(opts.Height),
		maxHeight:              C.float(opts.MaxHeight),
	}

	C.ShowOverlay(cOpts)
}

func Close(id string) {
	clickCallbacksMu.Lock()
	delete(clickCallbacks, id)
	clickCallbacksMu.Unlock()
	closeCallbacksMu.Lock()
	delete(closeCallbacks, id)
	closeCallbacksMu.Unlock()
	cName := C.CString(id)
	defer C.free(unsafe.Pointer(cName))
	C.CloseOverlay(cName)
	ReleaseNativeAttachment(id)
}

//export overlayClickCallbackCGO
func overlayClickCallbackCGO(cName *C.char) C.bool {
	name := C.GoString(cName)
	clickCallbacksMu.RLock()
	cb, ok := clickCallbacks[name]
	clickCallbacksMu.RUnlock()
	if ok {
		return C.bool(cb())
	}
	return C.bool(false)
}

//export overlayCloseCallbackCGO
func overlayCloseCallbackCGO(cName *C.char) {
	invokeCloseCallback(C.GoString(cName))
}

//export overlayRequestCloseCallbackCGO
func overlayRequestCloseCallbackCGO(cName *C.char) {
	RequestClose(C.GoString(cName))
}
