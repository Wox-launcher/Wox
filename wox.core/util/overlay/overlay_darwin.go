package overlay

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices -framework CoreVideo -framework QuartzCore
#include <stdlib.h>
#include <stdbool.h>

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
    bool centerContent;
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

// Callback from C
bool overlayClickCallbackCGO(char* name);
void overlayCloseCallbackCGO(char* name);
void overlayDebugLogCallbackCGO(char* message);

*/
import "C"
import (
	"bytes"
	"context"
	"image"
	"image/png"
	"sync"
	"unsafe"

	"wox/util"
	"wox/util/mainthread"
)

var clickCallbacks = make(map[string]func() bool)
var clickCallbacksMu sync.RWMutex

var closeCallbacks = make(map[string]func())
var closeCallbacksMu sync.RWMutex

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
	name := C.GoString(cName)
	closeCallbacksMu.RLock()
	cb, ok := closeCallbacks[name]
	closeCallbacksMu.RUnlock()
	if ok {
		cb()
	}
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

func Show(opts OverlayOptions) {
	if opts.OnClick != nil {
		clickCallbacksMu.Lock()
		clickCallbacks[opts.Name] = opts.OnClick
		clickCallbacksMu.Unlock()
	} else {
		clickCallbacksMu.Lock()
		delete(clickCallbacks, opts.Name)
		clickCallbacksMu.Unlock()
	}

	if opts.OnClose != nil {
		closeCallbacksMu.Lock()
		closeCallbacks[opts.Name] = opts.OnClose
		closeCallbacksMu.Unlock()
	} else {
		closeCallbacksMu.Lock()
		delete(closeCallbacks, opts.Name)
		closeCallbacksMu.Unlock()
	}

	mainthread.Call(func() {
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

		offsetY := opts.OffsetY
		if !opts.AbsolutePosition {
			offsetY = -opts.OffsetY // Invert Y for MacOS to match Y-Down semantics requested
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
			centerContent:            C.bool(opts.CenterContent),
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
			offsetY:                  C.float(offsetY),
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
	clickCallbacksMu.Lock()
	delete(clickCallbacks, name)
	clickCallbacksMu.Unlock()
	// Remove the close callback so programmatic Close does not fire OnClose.
	// Only user-initiated close (close button / Escape) should trigger it.
	closeCallbacksMu.Lock()
	delete(closeCallbacks, name)
	closeCallbacksMu.Unlock()
	mainthread.Call(func() {
		cName := C.CString(name)
		defer C.free(unsafe.Pointer(cName))
		C.CloseOverlay(cName)
	})
}
