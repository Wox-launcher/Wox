package overlay

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices -framework CoreVideo -framework QuartzCore
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

// Callback from C
bool overlayClickCallbackCGO(char* name);
void overlayCloseCallbackCGO(char* name);
void overlayRequestCloseCallbackCGO(char* name);
void overlayDebugLogCallbackCGO(char* message);

*/
import "C"
import (
	"context"
	"unsafe"

	"wox/util"
	"wox/util/mainthread"
)

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

//export overlayDebugLogCallbackCGO
func overlayDebugLogCallbackCGO(cMessage *C.char) {
	if cMessage == nil {
		return
	}
	// Diagnostics are emitted at INFO because the default Wox log level filters
	// DEBUG. Native-side sampling keeps the output useful without hiding the drag
	// timing evidence needed to diagnose sticky overlay lag.
	util.GetLogger().Info(context.Background(), "[Overlay] "+C.GoString(cMessage))
}

func showWindow(opts WindowOptions) {
	mainthread.Call(func() {
		cName := C.CString(opts.ID)
		defer C.free(unsafe.Pointer(cName))

		offsetY := opts.OffsetY
		if !opts.AbsolutePosition {
			offsetY = -opts.OffsetY // Invert Y for MacOS to match Y-Down semantics requested
		}

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
			offsetY:                C.float(offsetY),
			width:                  C.float(opts.Width),
			minWidth:               C.float(opts.MinWidth),
			maxWidth:               C.float(opts.MaxWidth),
			height:                 C.float(opts.Height),
			maxHeight:              C.float(opts.MaxHeight),
		}

		C.ShowOverlay(cOpts)
	})
}

func Close(id string) {
	clickCallbacksMu.Lock()
	delete(clickCallbacks, id)
	clickCallbacksMu.Unlock()
	// Remove the close callback so programmatic Close does not fire OnClose.
	// Only user-initiated close (close button / Escape) should trigger it.
	closeCallbacksMu.Lock()
	delete(closeCallbacks, id)
	closeCallbacksMu.Unlock()
	mainthread.Call(func() {
		cName := C.CString(id)
		defer C.free(unsafe.Pointer(cName))
		C.CloseOverlay(cName)
	})
	ReleaseNativeAttachment(id)
}
