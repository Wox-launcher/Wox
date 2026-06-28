//go:build darwin && cgo

package ui

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework QuartzCore -framework CoreText -framework ApplicationServices -framework Carbon

#include <stdlib.h>
#include "ui_native.h"

// macOS-only extra declarations (not in the shared header because they
// only exist on this platform).
extern void uiDarwinOnDraw(int32_t windowId);
extern void uiWindowInvalidate(int32_t windowId);
*/
import "C"
import "log"
import "unsafe"

// MacRenderer implements NativeRenderer using CoreGraphics + CoreText on
// macOS. All shared logic (command conversion, image cache, event dispatch)
// lives in baseRenderer; this struct only adds thin platform-specific C calls.
type MacRenderer struct {
	baseRenderer
}

// MacTextMeasurer implements TextMeasurer using CoreText.
type MacTextMeasurer struct{}

// Compile-time interface assertion.
var _ NativeRenderer = (*MacRenderer)(nil)

// NewNativeRenderer creates a native macOS window and returns a NativeRenderer.
// The window starts hidden — call Show() to display it. Must be called on the
// main thread (Cocoa requires UI creation on the main thread).
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
	r := &MacRenderer{
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

//export uiDarwinOnDraw
func uiDarwinOnDraw(windowID C.int32_t) {
	r := findRenderer(int32(windowID))
	if r == nil {
		log.Printf("[uiDarwinOnDraw] no active renderer for windowId=%d", windowID)
		return
	}
	cb := r.renderCallback
	if cb == nil {
		log.Printf("[uiDarwinOnDraw] renderCallback is nil")
		return
	}
	commands := cb()
	if commands == nil {
		return
	}
	if len(commands.Commands) == 0 {
		return
	}
	// self is the full NativeRenderer; call Render through it so the
	// platform's Render method handles C conversion.
	if r.self != nil {
		_ = r.self.Render(commands)
	}
}

// Render executes the command list via CoreGraphics. On macOS the normal
// rendering path is driven by drawRect: (via uiDarwinOnDraw), so this method
// is only called directly in fallback scenarios.
func (r *MacRenderer) Render(commands *CommandList) error {
	if commands == nil || len(commands.Commands) == 0 {
		return nil
	}
	cCmds, textPtrs, imgPtrs := r.toCCommands(commands.Commands)
	C.uiWindowRender(C.int32_t(r.windowID), &cCmds[0], C.int32_t(len(cCmds)))
	r.freeCCommands(textPtrs, imgPtrs)
	r.trackUploadedImages(commands.Commands)
	return nil
}

func (r *MacRenderer) TextMeasurer() TextMeasurer {
	return MacTextMeasurer{}
}

func (MacTextMeasurer) MeasureText(text string, fontSize float32, fontFamily string) (width, height float32) {
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

func (r *MacRenderer) Show() error {
	C.uiWindowShow(C.int32_t(r.windowID))
	return nil
}

func (r *MacRenderer) Hide() error {
	C.uiWindowHide(C.int32_t(r.windowID))
	return nil
}

func (r *MacRenderer) SetPosition(x, y int) error {
	C.uiWindowSetPosition(C.int32_t(r.windowID), C.int32_t(x), C.int32_t(y))
	return nil
}

func (r *MacRenderer) SetSize(w, h int) error {
	C.uiWindowSetSize(C.int32_t(r.windowID), C.int32_t(w), C.int32_t(h))
	return nil
}

func (r *MacRenderer) Close() error {
	C.uiWindowDestroy(C.int32_t(r.windowID))
	activeRenderers.Delete(r.windowID)
	return nil
}

func (r *MacRenderer) IsVisible() bool {
	return bool(C.uiWindowIsVisible(C.int32_t(r.windowID)))
}

func (r *MacRenderer) GetSize() (int, int) {
	var w, h C.int32_t
	C.uiWindowGetSize(C.int32_t(r.windowID), &w, &h)
	return int(w), int(h)
}

func (r *MacRenderer) SetDarkMode(dark bool) {
	C.uiWindowSetDarkMode(C.int32_t(r.windowID), C.bool(dark))
}

// ReleaseMemory drops native caches that are only useful while the launcher
// is visible. Called when the window is hidden.
func (r *MacRenderer) ReleaseMemory() {
	C.uiWindowReleaseMemory(C.int32_t(r.windowID))
	r.clearImageCache()
}

// RequestRepaint triggers a native repaint via setNeedsDisplay:.
func (r *MacRenderer) RequestRepaint() {
	C.uiWindowInvalidate(C.int32_t(r.windowID))
}

// StartEventLoop on macOS is a no-op: the Cocoa event loop ([NSApp run]) is
// already running (started by mainthread_darwin.m os_main). We just store the
// onRender callback so drawRect: (via uiDarwinOnDraw) can retrieve commands
// from the Go layout engine on demand. Blocking here would freeze the main
// thread and prevent dispatch_async blocks (Show/Hide/Resize) from running.
func (r *MacRenderer) StartEventLoop(onRender func() *CommandList) {
	r.renderCallback = onRender
}