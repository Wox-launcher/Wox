package textoverlay

/*
#cgo CFLAGS: -DUNICODE -D_UNICODE
#cgo LDFLAGS: -lgdi32 -luser32 -lmsimg32 -luxtheme
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
TextOverlayAttachment TextOverlayUpdateWindow(
    void* hwnd,
    char* message,
    int iconLen,
    bool loading,
    bool centerContent,
    float fontSize,
    float iconSize,
    int tooltipIconLen,
    float tooltipIconSize,
    bool showCopyButton,
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
	"sync"
	"unsafe"

	"wox/util/overlay"
)

// windowsTextRendererRegistration prevents stale release callbacks from destroying a reused HWND.
type windowsTextRendererRegistration struct {
	handle     uintptr
	generation uint64
}

var windowsTextRenderersMu sync.Mutex
var windowsTextRenderers = map[string]windowsTextRendererRegistration{}
var windowsTextRendererGeneration uint64

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

	windowsTextRenderersMu.Lock()
	defer windowsTextRenderersMu.Unlock()

	if existing, ok := windowsTextRenderers[opts.Window.ID]; ok && existing.handle != 0 {
		result := C.TextOverlayUpdateWindow(
			unsafe.Pointer(existing.handle),
			cMessage,
			cIconLen,
			C.bool(opts.Loading),
			C.bool(opts.CenterContent),
			C.float(opts.FontSize),
			C.float(opts.IconSize),
			cTooltipIconLen,
			C.float(opts.TooltipIconSize),
			C.bool(opts.ShowCopyButton),
			C.bool(opts.Closable),
			C.int(opts.AutoCloseSeconds),
			C.float(opts.Window.Width),
			C.float(opts.Window.MinWidth),
			C.float(opts.Window.MaxWidth),
			C.float(opts.Window.Height),
			C.float(opts.Window.MaxHeight),
		)
		if result.handle != nil {
			windowsTextRendererGeneration++
			windowsTextRenderers[opts.Window.ID] = windowsTextRendererRegistration{handle: uintptr(result.handle), generation: windowsTextRendererGeneration}
			return &textRenderer{
				id:         opts.Window.ID,
				generation: windowsTextRendererGeneration,
				handle:     uintptr(result.handle),
				width:      float64(result.width),
				height:     float64(result.height),
			}, true
		}
		delete(windowsTextRenderers, opts.Window.ID)
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
	windowsTextRendererGeneration++
	windowsTextRenderers[opts.Window.ID] = windowsTextRendererRegistration{handle: uintptr(result.handle), generation: windowsTextRendererGeneration}
	return &textRenderer{
		id:         opts.Window.ID,
		generation: windowsTextRendererGeneration,
		handle:     uintptr(result.handle),
		width:      float64(result.width),
		height:     float64(result.height),
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

	handle := renderer.handle
	shouldDestroy := true
	windowsTextRenderersMu.Lock()
	if current, ok := windowsTextRenderers[renderer.id]; ok {
		if current.handle == handle && current.generation == renderer.generation {
			delete(windowsTextRenderers, renderer.id)
		} else if current.handle == handle {
			shouldDestroy = false
		}
	}
	windowsTextRenderersMu.Unlock()

	if shouldDestroy {
		C.TextOverlayDestroyWindow(unsafe.Pointer(handle))
	}
	renderer.handle = 0
}
