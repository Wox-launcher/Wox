package timeroverlay

/*
#cgo CFLAGS: -DUNICODE -D_UNICODE
#cgo LDFLAGS: -lgdi32 -luser32 -lmsimg32 -luxtheme
#include <stdlib.h>
#include <stdbool.h>

typedef struct {
    void* handle;
    float width;
    float height;
} TimerOverlayAttachment;

TimerOverlayAttachment TimerOverlayCreateWindow(
    char* name,
    char* countdown,
    char* note,
    bool closable,
    float countdownFontSize,
    float noteFontSize,
    float windowWidth,
    float minWindowWidth,
    float maxWindowWidth,
    float windowHeight,
    float maxWindowHeight
);
TimerOverlayAttachment TimerOverlayUpdateWindow(
    void* hwnd,
    char* countdown,
    char* note,
    bool closable,
    float countdownFontSize,
    float noteFontSize,
    float windowWidth,
    float minWindowWidth,
    float maxWindowWidth,
    float windowHeight,
    float maxWindowHeight
);
void TimerOverlayDestroyWindow(void* hwnd);
*/
import "C"
import (
	"sync"
	"unsafe"

	"wox/util/overlay"
)

type windowsTimerRendererRegistration struct {
	handle     uintptr
	generation uint64
}

var windowsTimerRenderersMu sync.Mutex
var windowsTimerRenderers = map[string]windowsTimerRendererRegistration{}
var windowsTimerRendererGeneration uint64

func newTimerRenderer(opts Options) (*timerRenderer, bool) {
	cName := C.CString(opts.Window.ID)
	defer C.free(unsafe.Pointer(cName))

	cCountdown := C.CString(opts.Countdown)
	defer C.free(unsafe.Pointer(cCountdown))

	cNote := C.CString(opts.Note)
	defer C.free(unsafe.Pointer(cNote))

	windowsTimerRenderersMu.Lock()
	defer windowsTimerRenderersMu.Unlock()

	if existing, ok := windowsTimerRenderers[opts.Window.ID]; ok && existing.handle != 0 {
		result := C.TimerOverlayUpdateWindow(
			unsafe.Pointer(existing.handle),
			cCountdown,
			cNote,
			C.bool(opts.Closable),
			C.float(opts.CountdownFontSize),
			C.float(opts.NoteFontSize),
			C.float(opts.Window.Width),
			C.float(opts.Window.MinWidth),
			C.float(opts.Window.MaxWidth),
			C.float(opts.Window.Height),
			C.float(opts.Window.MaxHeight),
		)
		if result.handle != nil {
			windowsTimerRendererGeneration++
			windowsTimerRenderers[opts.Window.ID] = windowsTimerRendererRegistration{
				handle:     uintptr(result.handle),
				generation: windowsTimerRendererGeneration,
			}
			return &timerRenderer{
				id:         opts.Window.ID,
				generation: windowsTimerRendererGeneration,
				handle:     uintptr(result.handle),
				width:      float64(result.width),
				height:     float64(result.height),
			}, true
		}
		delete(windowsTimerRenderers, opts.Window.ID)
	}

	result := C.TimerOverlayCreateWindow(
		cName,
		cCountdown,
		cNote,
		C.bool(opts.Closable),
		C.float(opts.CountdownFontSize),
		C.float(opts.NoteFontSize),
		C.float(opts.Window.Width),
		C.float(opts.Window.MinWidth),
		C.float(opts.Window.MaxWidth),
		C.float(opts.Window.Height),
		C.float(opts.Window.MaxHeight),
	)
	if result.handle == nil {
		return nil, false
	}
	windowsTimerRendererGeneration++
	windowsTimerRenderers[opts.Window.ID] = windowsTimerRendererRegistration{
		handle:     uintptr(result.handle),
		generation: windowsTimerRendererGeneration,
	}
	return &timerRenderer{
		id:         opts.Window.ID,
		generation: windowsTimerRendererGeneration,
		handle:     uintptr(result.handle),
		width:      float64(result.width),
		height:     float64(result.height),
	}, true
}

func (renderer *timerRenderer) nativeAttachment() overlay.NativeAttachment {
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

func (renderer *timerRenderer) destroy() {
	if renderer == nil || renderer.handle == 0 {
		return
	}

	handle := renderer.handle
	shouldDestroy := true
	windowsTimerRenderersMu.Lock()
	if current, ok := windowsTimerRenderers[renderer.id]; ok {
		if current.handle == handle && current.generation == renderer.generation {
			delete(windowsTimerRenderers, renderer.id)
		} else if current.handle == handle {
			shouldDestroy = false
		}
	}
	windowsTimerRenderersMu.Unlock()

	if shouldDestroy {
		C.TimerOverlayDestroyWindow(unsafe.Pointer(handle))
	}
	renderer.handle = 0
}
