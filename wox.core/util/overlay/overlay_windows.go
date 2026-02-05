package overlay

/*
#cgo CFLAGS: -DUNICODE -D_UNICODE
#cgo LDFLAGS: -ldwmapi -luxtheme -lmsimg32 -lgdi32 -luser32 -lole32 -luuid -lwindowscodecs -lcomctl32
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
    float fontSize;
    float iconSize;
    char* tooltip;
    unsigned char* tooltipIconData;
    int tooltipIconLen;
    float tooltipIconSize;
} OverlayOptions;

void ShowOverlay(OverlayOptions opts);
void CloseOverlay(char* name);
void overlayClickCallbackCGO(char* name);
*/
import "C"
import (
	"bytes"
	"image"
	"image/png"
	"unsafe"
)

var clickCallbacks = make(map[string]func())

func Show(opts OverlayOptions) {
	if opts.OnClick != nil {
		clickCallbacks[opts.Name] = opts.OnClick
	}

	cName := C.CString(opts.Name)
	defer C.free(unsafe.Pointer(cName))

	cTitle := C.CString(opts.Title)
	defer C.free(unsafe.Pointer(cTitle))

	cMessage := C.CString(opts.Message)
	defer C.free(unsafe.Pointer(cMessage))

	cTooltip := C.CString(opts.Tooltip)
	defer C.free(unsafe.Pointer(cTooltip))

	var cIconData *C.uchar
	var cIconLen C.int
	pngBytes, _ := imageToPNG(opts.Icon)
	if len(pngBytes) > 0 {
		cIconData = (*C.uchar)(unsafe.Pointer(&pngBytes[0]))
		cIconLen = C.int(len(pngBytes))
	}

	var cTooltipIconData *C.uchar
	var cTooltipIconLen C.int
	tooltipPngBytes, _ := imageToPNG(opts.TooltipIcon)
	if len(tooltipPngBytes) > 0 {
		cTooltipIconData = (*C.uchar)(unsafe.Pointer(&tooltipPngBytes[0]))
		cTooltipIconLen = C.int(len(tooltipPngBytes))
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
		offsetY:          C.float(opts.OffsetY),
		width:            C.float(opts.Width),
		height:           C.float(opts.Height),
		fontSize:         C.float(opts.FontSize),
		iconSize:         C.float(opts.IconSize),
		tooltip:          cTooltip,
		tooltipIconData:  cTooltipIconData,
		tooltipIconLen:   cTooltipIconLen,
		tooltipIconSize:  C.float(opts.TooltipIconSize),
	}

	C.ShowOverlay(cOpts)
}

func Close(name string) {
	delete(clickCallbacks, name)
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	C.CloseOverlay(cName)
}

//export overlayClickCallbackCGO
func overlayClickCallbackCGO(cName *C.char) {
	name := C.GoString(cName)
	if cb, ok := clickCallbacks[name]; ok {
		cb()
	}
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
