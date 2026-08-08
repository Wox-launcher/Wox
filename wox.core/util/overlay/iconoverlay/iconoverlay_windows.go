package iconoverlay

/*
#cgo CFLAGS: -DUNICODE -D_UNICODE
#cgo LDFLAGS: -lgdi32 -luser32 -lole32 -lwindowscodecs
#include <stdlib.h>
#include <stdbool.h>

typedef struct {
    void* handle;
    float width;
    float height;
} IconOverlayAttachment;

IconOverlayAttachment IconOverlayCreateWindow(
    char* name,
    unsigned char* iconData,
    int iconLen,
    float width,
    float height,
    float iconSize
);
void IconOverlayDestroyWindow(void* hwnd);
*/
import "C"
import (
	"math"
	"unsafe"

	"wox/util/overlay"
)

func newIconRenderer(opts Options) (*iconRenderer, bool) {
	if opts.Icon == nil {
		return nil, false
	}
	pngBytes, err := imageToPNG(opts.Icon)
	if err != nil || len(pngBytes) == 0 {
		return nil, false
	}

	cName := C.CString(opts.Window.ID)
	defer C.free(unsafe.Pointer(cName))
	cIconData := (*C.uchar)(unsafe.Pointer(&pngBytes[0]))
	iconSize := opts.IconSize
	if iconSize <= 0 {
		iconSize = math.Min(opts.Window.Width, opts.Window.Height)
	}
	result := C.IconOverlayCreateWindow(
		cName,
		cIconData,
		C.int(len(pngBytes)),
		C.float(opts.Window.Width),
		C.float(opts.Window.Height),
		C.float(iconSize),
	)
	if result.handle == nil {
		return nil, false
	}
	return &iconRenderer{
		id:     opts.Window.ID,
		handle: uintptr(result.handle),
		width:  float64(result.width),
		height: float64(result.height),
	}, true
}

func (renderer *iconRenderer) nativeAttachment() overlay.NativeAttachment {
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

func (renderer *iconRenderer) destroy() {
	if renderer == nil || renderer.handle == 0 {
		return
	}
	C.IconOverlayDestroyWindow(unsafe.Pointer(renderer.handle))
	renderer.handle = 0
}
