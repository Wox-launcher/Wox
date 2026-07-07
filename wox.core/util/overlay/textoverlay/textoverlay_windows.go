package textoverlay

/*
#cgo CFLAGS: -DUNICODE -D_UNICODE
#cgo LDFLAGS: -lgdi32 -luser32
#include <stdlib.h>
#include <stdbool.h>

typedef struct {
    void* handle;
    float width;
    float height;
} TextOverlayAttachment;

TextOverlayAttachment TextOverlayCreateWindow(
    char* name,
    char* message,
    unsigned char* iconData,
    int iconLen,
    bool loading,
    bool centerContent,
    float fontSize,
    float iconSize,
    char* tooltip,
    unsigned char* tooltipIconData,
    int tooltipIconLen,
    float tooltipIconSize,
    bool showCopyButton,
    char* copyButtonTooltip,
    char* copyButtonSuccessTooltip,
    bool closable,
    int autoCloseSeconds,
    float windowWidth,
    float minWindowWidth,
    float maxWindowWidth,
    float windowHeight,
    float maxWindowHeight
);
void TextOverlayDestroyWindow(void* hwnd);
*/
import "C"
import (
	"unsafe"

	"wox/util/overlay"
)

func newTextRenderer(opts Options) (*textRenderer, bool) {
	cName := C.CString(opts.Window.ID)
	defer C.free(unsafe.Pointer(cName))

	cMessage := C.CString(opts.Message)
	defer C.free(unsafe.Pointer(cMessage))

	cTooltip := C.CString(opts.Tooltip)
	defer C.free(unsafe.Pointer(cTooltip))

	cCopyButtonTooltip := C.CString(opts.CopyButtonTooltip)
	defer C.free(unsafe.Pointer(cCopyButtonTooltip))

	cCopyButtonSuccessTooltip := C.CString(opts.CopyButtonSuccessTooltip)
	defer C.free(unsafe.Pointer(cCopyButtonSuccessTooltip))

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

	result := C.TextOverlayCreateWindow(
		cName,
		cMessage,
		cIconData,
		cIconLen,
		C.bool(opts.Loading),
		C.bool(opts.CenterContent),
		C.float(opts.FontSize),
		C.float(opts.IconSize),
		cTooltip,
		cTooltipIconData,
		cTooltipIconLen,
		C.float(opts.TooltipIconSize),
		C.bool(opts.ShowCopyButton),
		cCopyButtonTooltip,
		cCopyButtonSuccessTooltip,
		C.bool(opts.Closable),
		C.int(opts.AutoCloseSeconds),
		C.float(opts.Window.Width),
		C.float(opts.Window.MinWidth),
		C.float(opts.Window.MaxWidth),
		C.float(opts.Window.Height),
		C.float(opts.Window.MaxHeight),
	)
	if result.handle == nil {
		return nil, false
	}
	return &textRenderer{
		handle: uintptr(result.handle),
		width:  float64(result.width),
		height: float64(result.height),
	}, true
}

func (renderer *textRenderer) nativeAttachment() overlay.NativeAttachment {
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

func (renderer *textRenderer) destroy() {
	if renderer == nil || renderer.handle == 0 {
		return
	}
	C.TextOverlayDestroyWindow(unsafe.Pointer(renderer.handle))
	renderer.handle = 0
}
