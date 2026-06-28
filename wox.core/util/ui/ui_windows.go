//go:build windows && cgo

package ui

/*
#cgo CFLAGS: -DUNICODE -D_UNICODE
#cgo LDFLAGS: -ld2d1 -ldwrite -ldwmapi -lwindowscodecs -lole32 -luuid -luser32 -lgdi32 -lshcore -ld3d11 -ldxgi -ldcomp

#include <stdlib.h>
#include "ui_native.h"

// Windows-only extra declarations (not in the shared header because they
// only exist on this platform).
extern bool uiPumpMessages(void);
extern void uiInvalidateWindow(int32_t windowId);
*/
import "C"
import "unsafe"

// WindowsRenderer implements NativeRenderer using Direct2D/DirectWrite.
// All shared logic (command conversion, image cache, event dispatch) lives
// in baseRenderer; this struct only adds thin platform-specific C calls.
type WindowsRenderer struct {
	baseRenderer
}

// WindowsTextMeasurer implements TextMeasurer using DirectWrite.
type WindowsTextMeasurer struct{}

// Compile-time interface assertion.
var _ NativeRenderer = (*WindowsRenderer)(nil)

// NewNativeRenderer creates a native Windows window and returns a
// NativeRenderer. The window starts hidden — call Show() to display it.
func NewNativeRenderer(width, height int, theme Theme, eventCb EventCallback) (NativeRenderer, error) {
	bg := theme.WindowBg
	lum := 0.2126*bg.R + 0.7152*bg.G + 0.0722*bg.B
	darkMode := lum < 0.5

	cfg := C.CWindowConfig{
		width:        C.int32_t(width),
		height:       C.int32_t(height),
		cornerRadius: C.float(theme.WindowRadius),
		frameless:    C.bool(true),
		transparent:  C.bool(true),
		darkMode:     C.bool(darkMode),
	}

	id := C.uiWindowCreate(cfg)
	if id == 0 {
		return nil, &WindowError{Op: "create", Err: "native window creation failed"}
	}

	goID := int32(id)
	r := &WindowsRenderer{
		baseRenderer: baseRenderer{
			windowID:        goID,
			nativeImageKeys: make(map[string]struct{}),
			eventHandler:    eventCb,
		},
	}
	r.self = r
	activeRenderers.Store(goID, &r.baseRenderer)
	return r, nil
}

//export uiEventCallback
func uiEventCallback(windowID C.int32_t, eventType C.int32_t, key C.int32_t, mods C.int32_t,
	text *C.char, textLen C.int32_t,
	composeText *C.char, composeTextLen C.int32_t, composeCursor C.int32_t,
	x C.float, y C.float, deltaY C.float,
	width C.int32_t, height C.int32_t) {
	r := findRenderer(int32(windowID))
	if r == nil {
		return
	}
	r.dispatchEvent(parseCEvent(eventType, key, mods, text, textLen,
		composeText, composeTextLen, composeCursor, x, y, deltaY, width, height))
}

// Render executes the command list on the native Direct2D surface.
func (r *WindowsRenderer) Render(commands *CommandList) error {
	if len(commands.Commands) == 0 {
		return nil
	}
	cCmds, textPtrs, imgPtrs := r.toCCommands(commands.Commands)
	C.uiWindowRender(C.int32_t(r.windowID), &cCmds[0], C.int32_t(len(cCmds)))
	r.freeCCommands(textPtrs, imgPtrs)
	r.trackUploadedImages(commands.Commands)
	return nil
}

// TextMeasurer returns the DirectWrite-backed text measurer.
func (r *WindowsRenderer) TextMeasurer() TextMeasurer {
	return WindowsTextMeasurer{}
}

func (WindowsTextMeasurer) MeasureText(text string, fontSize float32, fontFamily string) (width, height float32) {
	if len(text) == 0 {
		return 0, fontSize * 1.2
	}

	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))

	var familyPtr *C.char
	var familyLen C.int32_t
	if fontFamily != "" {
		familyPtr = C.CString(fontFamily)
		defer C.free(unsafe.Pointer(familyPtr))
		familyLen = C.int32_t(len(fontFamily))
	}

	result := C.uiMeasureText(cText, C.int32_t(len(text)), C.float(fontSize), familyPtr, familyLen)
	return float32(result.width), float32(result.height)
}

func (r *WindowsRenderer) Show() error {
	C.uiWindowShow(C.int32_t(r.windowID))
	return nil
}

func (r *WindowsRenderer) Hide() error {
	C.uiWindowHide(C.int32_t(r.windowID))
	return nil
}

func (r *WindowsRenderer) SetPosition(x, y int) error {
	C.uiWindowSetPosition(C.int32_t(r.windowID), C.int32_t(x), C.int32_t(y))
	return nil
}

func (r *WindowsRenderer) SetSize(w, h int) error {
	C.uiWindowSetSize(C.int32_t(r.windowID), C.int32_t(w), C.int32_t(h))
	return nil
}

func (r *WindowsRenderer) Close() error {
	C.uiWindowDestroy(C.int32_t(r.windowID))
	activeRenderers.Delete(r.windowID)
	return nil
}

func (r *WindowsRenderer) IsVisible() bool {
	return bool(C.uiWindowIsVisible(C.int32_t(r.windowID)))
}

func (r *WindowsRenderer) GetSize() (int, int) {
	var w, h C.int32_t
	C.uiWindowGetSize(C.int32_t(r.windowID), &w, &h)
	return int(w), int(h)
}

// SetDarkMode toggles the DWM immersive dark mode attribute so the Mica
// backdrop renders in the tone matching the active theme.
func (r *WindowsRenderer) SetDarkMode(dark bool) {
	C.uiWindowSetDarkMode(C.int32_t(r.windowID), C.bool(dark))
}

// ReleaseMemory drops native caches that are only useful while the launcher
// is visible. Called when the window is hidden.
func (r *WindowsRenderer) ReleaseMemory() {
	C.uiWindowReleaseMemory(C.int32_t(r.windowID))
	r.clearImageCache()
}

// RequestRepaint triggers a repaint of the window.
func (r *WindowsRenderer) RequestRepaint() {
	C.uiInvalidateWindow(C.int32_t(r.windowID))
}

// StartEventLoop runs the native Win32 message loop. Blocks until Close()
// is called or the window is destroyed. The onRender callback is invoked
// each frame to produce draw commands; it returns nil when the window is
// hidden so the loop can block on GetMessage without busy-rendering.
func (r *WindowsRenderer) StartEventLoop(onRender func() *CommandList) {
	for {
		// GetMessage blocks until a message arrives, so the loop does not
		// busy-poll when the window is hidden or idle.
		if !C.uiPumpMessages() {
			break // WM_QUIT received
		}
		cmds := onRender()
		if cmds != nil {
			r.Render(cmds)
		}
	}
}