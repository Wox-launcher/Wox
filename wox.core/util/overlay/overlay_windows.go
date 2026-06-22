package overlay

/*
#cgo CFLAGS: -DUNICODE -D_UNICODE
#cgo LDFLAGS: -ldwmapi -luxtheme -lmsimg32 -lgdi32 -luser32 -lole32 -luuid -lwindowscodecs -lcomctl32
#include <stdlib.h>
#include <stdbool.h>
#include <stdint.h>

typedef struct {
    char* name;
    char* title;
    char* message;
    unsigned char* iconData;
    int iconLen;
    char* iconFilePath;
    bool transparent;
    bool hitTestIconOnly;
    float iconX;
    float iconY;
    float iconWidth;
    float iconHeight;
    bool closable;
    bool closeOnEscape;
    bool loading;
    bool topmost;
    bool absolutePosition;
    bool preservePosition;
    int stickyWindowPid;
    int anchor;
    int autoCloseSeconds;
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
    bool followScroll;
    float fontSize;
    float iconSize;
    char* tooltip;
    unsigned char* tooltipIconData;
    int tooltipIconLen;
    float tooltipIconSize;
    bool showCopyButton;
    char* copyButtonTooltip;
    char* copyButtonSuccessTooltip;
} OverlayOptions;

void ShowOverlay(OverlayOptions opts);
void CloseOverlay(char* name);
bool overlayClickCallbackCGO(char* name);
uintptr_t GetOverlayWindowHandle(char* name);
*/
import "C"
import (
	"bytes"
	"image"
	"image/png"
	"sync"
	"unsafe"
)

var clickCallbacks = make(map[string]func() bool)
var clickCallbacksMu sync.RWMutex

func Show(opts OverlayOptions) {
	if opts.OnClick != nil {
		// Reused overlays can refresh their callbacks while native click events are
		// delivered from another thread. Guard replacement so the overlay API stays
		// safe for high-frequency updates and ordinary notification windows.
		clickCallbacksMu.Lock()
		clickCallbacks[opts.Name] = opts.OnClick
		clickCallbacksMu.Unlock()
	}

	cName := C.CString(opts.Name)
	defer C.free(unsafe.Pointer(cName))

	cTitle := C.CString(opts.Title)
	defer C.free(unsafe.Pointer(cTitle))

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
	var cIconFilePath *C.char
	iconKind := opts.Icon.activeKind()
	var pngBytes []byte
	switch iconKind {
	case OverlayImageKindFile:
		if opts.Icon.FilePath != "" {
			cIconFilePath = C.CString(opts.Icon.FilePath)
			defer C.free(unsafe.Pointer(cIconFilePath))
		}
	case OverlayImageKindImage:
		pngBytes, _ = imageToPNG(opts.Icon.Image)
		if len(pngBytes) > 0 {
			cIconData = (*C.uchar)(unsafe.Pointer(&pngBytes[0]))
			cIconLen = C.int(len(pngBytes))
		}
	}

	var cTooltipIconData *C.uchar
	var cTooltipIconLen C.int
	tooltipPngBytes, _ := imageToPNG(opts.TooltipIcon)
	if len(tooltipPngBytes) > 0 {
		cTooltipIconData = (*C.uchar)(unsafe.Pointer(&tooltipPngBytes[0]))
		cTooltipIconLen = C.int(len(tooltipPngBytes))
	}

	cOpts := C.OverlayOptions{
		name:                     cName,
		title:                    cTitle,
		message:                  cMessage,
		iconData:                 cIconData,
		iconLen:                  cIconLen,
		iconFilePath:             cIconFilePath,
		transparent:              C.bool(opts.Transparent),
		hitTestIconOnly:          C.bool(opts.HitTestIconOnly),
		iconX:                    C.float(opts.IconX),
		iconY:                    C.float(opts.IconY),
		iconWidth:                C.float(opts.IconWidth),
		iconHeight:               C.float(opts.IconHeight),
		closable:                 C.bool(opts.Closable),
		closeOnEscape:            C.bool(opts.CloseOnEscape),
		loading:                  C.bool(opts.Loading),
		topmost:                  C.bool(opts.Topmost),
		absolutePosition:         C.bool(opts.AbsolutePosition),
		preservePosition:         C.bool(opts.PreservePosition),
		stickyWindowPid:          C.int(opts.StickyWindowPid),
		anchor:                   C.int(opts.Anchor),
		autoCloseSeconds:         C.int(opts.AutoCloseSeconds),
		movable:                  C.bool(opts.Movable),
		resizable:                C.bool(opts.Resizable),
		cornerRadius:             C.float(opts.CornerRadius),
		aspectRatio:              C.float(opts.AspectRatio),
		offsetX:                  C.float(opts.OffsetX),
		offsetY:                  C.float(opts.OffsetY),
		width:                    C.float(opts.Width),
		minWidth:                 C.float(opts.MinWidth),
		maxWidth:                 C.float(opts.MaxWidth),
		height:                   C.float(opts.Height),
		maxHeight:                C.float(opts.MaxHeight),
		followScroll:             C.bool(opts.FollowScroll),
		fontSize:                 C.float(opts.FontSize),
		iconSize:                 C.float(opts.IconSize),
		tooltip:                  cTooltip,
		tooltipIconData:          cTooltipIconData,
		tooltipIconLen:           cTooltipIconLen,
		tooltipIconSize:          C.float(opts.TooltipIconSize),
		showCopyButton:           C.bool(opts.ShowCopyButton),
		copyButtonTooltip:        cCopyButtonTooltip,
		copyButtonSuccessTooltip: cCopyButtonSuccessTooltip,
	}

	C.ShowOverlay(cOpts)
}

func Close(name string) {
	clickCallbacksMu.Lock()
	delete(clickCallbacks, name)
	clickCallbacksMu.Unlock()
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	C.CloseOverlay(cName)
}

func GetWindowHandle(name string) uintptr {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	return uintptr(C.GetOverlayWindowHandle(cName))
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
