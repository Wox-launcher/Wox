package overlay

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices
#include <stdlib.h>
#include <stdbool.h>

typedef struct {
    char* name;
    char* title;
    char* message;
    unsigned char* iconData;
    int iconLen;
    bool closable;
    int stickyWindowPid;
    int anchor;
    int autoCloseSeconds;
    bool movable;
    float offsetX;
    float offsetY;
    float width;
    float height;
} OverlayOptions;

void ShowOverlay(OverlayOptions opts);
void CloseOverlay(char* name);

// Callback from C
void overlayClickCallbackCGO(char* name);

*/
import "C"
import (
	"bytes"
	"image"
	"image/png"
	"unsafe"

	"golang.design/x/hotkey/mainthread"
)

var clickCallbacks = make(map[string]func())

//export overlayClickCallbackCGO
func overlayClickCallbackCGO(cName *C.char) {
	name := C.GoString(cName)
	if cb, ok := clickCallbacks[name]; ok {
		cb()
	}
}

func Show(opts OverlayOptions) {
	if opts.OnClick != nil {
		clickCallbacks[opts.Name] = opts.OnClick
	}

	mainthread.Call(func() {
		cName := C.CString(opts.Name)
		defer C.free(unsafe.Pointer(cName))

		cTitle := C.CString(opts.Title)
		defer C.free(unsafe.Pointer(cTitle))

		cMessage := C.CString(opts.Message)
		defer C.free(unsafe.Pointer(cMessage))

		var cIconData *C.uchar
		var cIconLen C.int

		pngBytes, _ := imageToPNG(opts.Icon)
		if len(pngBytes) > 0 {
			cIconData = (*C.uchar)(unsafe.Pointer(&pngBytes[0]))
			cIconLen = C.int(len(pngBytes))
		}

		cOpts := C.OverlayOptions{
			name:             cName,
			title:            cTitle,
			message:          cMessage,
			iconData:         cIconData,
			iconLen:          cIconLen,
			closable:         C.bool(opts.Closable),
			stickyWindowPid:  C.int(opts.StickyWindowPid),
			anchor:           C.int(opts.Anchor),
			autoCloseSeconds: C.int(opts.AutoCloseSeconds),
			movable:          C.bool(opts.Movable),
			offsetX:          C.float(opts.OffsetX),
			offsetY:          C.float(-opts.OffsetY), // Invert Y for MacOS to match Y-Down semantics requested
			width:            C.float(opts.Width),
			height:           C.float(opts.Height),
		}

		C.ShowOverlay(cOpts)
	})
}

func imageToPNG(img image.Image) ([]byte, error) {
	if img == nil {
		return nil, nil
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func Close(name string) {
	delete(clickCallbacks, name)
	mainthread.Call(func() {
		cName := C.CString(name)
		defer C.free(unsafe.Pointer(cName))
		C.CloseOverlay(cName)
	})
}
