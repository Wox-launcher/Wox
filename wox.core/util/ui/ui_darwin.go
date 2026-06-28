//go:build darwin && cgo

package ui

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework QuartzCore -framework CoreText -framework ApplicationServices -framework Carbon

#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>

// CDrawCommand mirrors the Go DrawCommand struct (same layout as Windows).
typedef struct {
    int32_t cmd_type;
    float x, y, w, h;
    float r, g, b, a;
    float radius;
    float strokeWidth;
    const char* text;
    int32_t textLen;
    float fontSize;
    const char* fontFamily;
    int32_t fontFamilyLen;
    const uint8_t* imageData;
    int32_t imageLen;
    const char* imageKey;
    int32_t imageKeyLen;
    float imageWidth, imageHeight;
} CDrawCommand;

typedef struct {
    int32_t width;
    int32_t height;
    float cornerRadius;
    bool frameless;
    bool transparent;
    bool darkMode;
} CWindowConfig;

typedef struct {
    float width;
    float height;
} CMeasureResult;

// Native functions implemented in ui_darwin.m
extern int32_t uiWindowCreate(CWindowConfig config);
extern void uiWindowDestroy(int32_t windowId);
extern void uiWindowShow(int32_t windowId);
extern void uiWindowHide(int32_t windowId);
extern void uiWindowSetDarkMode(int32_t windowId, bool darkMode);
extern void uiWindowSetPosition(int32_t windowId, int32_t x, int32_t y);
extern void uiWindowSetSize(int32_t windowId, int32_t w, int32_t h);
extern bool uiWindowIsVisible(int32_t windowId);
extern void uiWindowGetSize(int32_t windowId, int32_t* outW, int32_t* outH);
extern void uiWindowReleaseMemory(int32_t windowId);
extern void uiWindowInvalidate(int32_t windowId);
extern void uiWindowRender(int32_t windowId, const CDrawCommand* commands, int32_t count);
extern CMeasureResult uiMeasureText(const char* text, int32_t textLen, float fontSize, const char* fontFamily, int32_t fontFamilyLen);

// Event callback — C calls this Go function for input events.
extern void uiEventCallback(int32_t windowId, int32_t eventType, int32_t key, int32_t mods,
    char* text, int32_t textLen,
    char* composeText, int32_t composeTextLen, int32_t composeCursor,
    float x, float y, float deltaY,
    int32_t width, int32_t height);

// Triggered by drawRect: to pull a fresh frame from the Go layout engine.
// This is a re-entrant call (Obj-C → Go → Obj-C) — safe because Go callbacks
// run on the calling thread (the Cocoa main thread) without spawning goroutines.
extern void uiDarwinOnDraw(int32_t windowId);
*/
import "C"
import (
	"log"
	"sync"
	"unsafe"
)

// MacRenderer implements NativeRenderer using CoreGraphics + CoreText on macOS.
// Symmetric to WindowsRenderer: an integer handle references the native window
// state kept in the Objective-C side.
type MacRenderer struct {
	windowID int32

	nativeImageMu   sync.Mutex
	nativeImageKeys map[string]struct{}
}

// MacTextMeasurer implements TextMeasurer using CoreText.
type MacTextMeasurer struct{}

var (
	macEventHandler EventCallback

	renderCallbackMu sync.Mutex
	renderCallback   func() *CommandList

	// activeRenderers maps windowId → *MacRenderer so the drawRect: Go export
	// (uiDarwinOnDraw) can find the right renderer without passing Go pointers
	// through C.
	activeRenderers sync.Map
)

// Compile-time interface assertion.
var _ NativeRenderer = (*MacRenderer)(nil)

// NewNativeRenderer creates a native macOS window and returns a NativeRenderer.
// The window starts hidden — call Show() to display it. Must be called on the
// main thread (Cocoa requires UI creation on the main thread).
func NewNativeRenderer(width, height int, theme Theme) (*MacRenderer, error) {
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
		windowID:        goID,
		nativeImageKeys: make(map[string]struct{}),
	}
	activeRenderers.Store(goID, r)
	return r, nil
}

// SetEventHandler registers the global event callback for native input events.
// Must be called before Show().
func SetEventHandler(cb EventCallback) {
	macEventHandler = cb
}

//export uiEventCallback
func uiEventCallback(windowID C.int32_t, eventType C.int32_t, key C.int32_t, mods C.int32_t,
	text *C.char, textLen C.int32_t,
	composeText *C.char, composeTextLen C.int32_t, composeCursor C.int32_t,
	x C.float, y C.float, deltaY C.float,
	width C.int32_t, height C.int32_t) {

	ev := Event{
		Type:   EventType(eventType),
		Key:    Key(key),
		Mods:   Modifiers(mods),
		X:      float32(x),
		Y:      float32(y),
		DeltaY: float32(deltaY),
		Width:  int32(width),
		Height: int32(height),
	}

	if text != nil && textLen > 0 {
		ev.Text = C.GoStringN(text, textLen)
	}
	if composeText != nil && composeTextLen > 0 {
		ev.ComposeText = C.GoStringN(composeText, composeTextLen)
		ev.ComposeCursor = int(composeCursor)
	}

	if macEventHandler != nil {
		macEventHandler(ev)
	}
}

//export uiDarwinOnDraw
func uiDarwinOnDraw(windowID C.int32_t) {
	val, ok := activeRenderers.Load(int32(windowID))
	if !ok {
		log.Printf("[uiDarwinOnDraw] no active renderer for windowId=%d", windowID)
		return
	}
	r, ok := val.(*MacRenderer)
	if !ok {
		return
	}
	renderCallbackMu.Lock()
	cb := renderCallback
	renderCallbackMu.Unlock()
	if cb == nil {
		log.Printf("[uiDarwinOnDraw] renderCallback is nil")
		return
	}
	commands := cb()
	if commands == nil {
		log.Printf("[uiDarwinOnDraw] commands is nil")
		return
	}
	if len(commands.Commands) == 0 {
		log.Printf("[uiDarwinOnDraw] commands is empty")
		return
	}
	log.Printf("[uiDarwinOnDraw] executing %d commands", len(commands.Commands))
	r.flattenAndExecute(commands.Commands)
}

// Render is a no-op on macOS: rendering is driven by drawRect: (via
// uiDarwinOnDraw). Kept to satisfy the Renderer interface so callers can
// invoke it without caring about the platform.
func (r *MacRenderer) Render(commands *CommandList) error {
	if commands == nil || len(commands.Commands) == 0 {
		return nil
	}
	r.flattenAndExecute(commands.Commands)
	return nil
}

// flattenAndExecute converts Go DrawCommands to C and calls the Objective-C
// executor. Mirrors WindowsRenderer.Render's conversion logic.
func (r *MacRenderer) flattenAndExecute(cmds []DrawCommand) {
	cCmds := make([]C.CDrawCommand, len(cmds))
	var cTextPtrs []*C.char
	var cImagePtrs []unsafe.Pointer

	for i, cmd := range cmds {
		cCmds[i].cmd_type = C.int32_t(cmd.Type)
		cCmds[i].x = C.float(cmd.X)
		cCmds[i].y = C.float(cmd.Y)
		cCmds[i].w = C.float(cmd.W)
		cCmds[i].h = C.float(cmd.H)
		cCmds[i].r = C.float(cmd.R)
		cCmds[i].g = C.float(cmd.G)
		cCmds[i].b = C.float(cmd.B)
		cCmds[i].a = C.float(cmd.A)
		cCmds[i].radius = C.float(cmd.Radius)
		cCmds[i].strokeWidth = C.float(cmd.StrokeWidth)
		cCmds[i].fontSize = C.float(cmd.FontSize)

		if cmd.Text != "" {
			cstr := C.CString(cmd.Text)
			cTextPtrs = append(cTextPtrs, cstr)
			cCmds[i].text = cstr
			cCmds[i].textLen = C.int32_t(len(cmd.Text))
		}
		if cmd.FontFamily != "" {
			cstr := C.CString(cmd.FontFamily)
			cTextPtrs = append(cTextPtrs, cstr)
			cCmds[i].fontFamily = cstr
			cCmds[i].fontFamilyLen = C.int32_t(len(cmd.FontFamily))
		}
		uploadImage := len(cmd.ImageData) > 0
		if cmd.ImageKey != "" {
			cstr := C.CString(cmd.ImageKey)
			cTextPtrs = append(cTextPtrs, cstr)
			cCmds[i].imageKey = cstr
			cCmds[i].imageKeyLen = C.int32_t(len(cmd.ImageKey))

			r.nativeImageMu.Lock()
			_, uploaded := r.nativeImageKeys[cmd.ImageKey]
			r.nativeImageMu.Unlock()
			if uploaded {
				uploadImage = false
			}
		}
		if uploadImage {
			cdata := C.CBytes(cmd.ImageData)
			cImagePtrs = append(cImagePtrs, cdata)
			cCmds[i].imageData = (*C.uint8_t)(cdata)
			cCmds[i].imageLen = C.int32_t(len(cmd.ImageData))
		}
	}

	C.uiWindowRender(C.int32_t(r.windowID), &cCmds[0], C.int32_t(len(cCmds)))

	r.nativeImageMu.Lock()
	for _, cmd := range cmds {
		if cmd.ImageKey != "" && len(cmd.ImageData) > 0 {
			r.nativeImageKeys[cmd.ImageKey] = struct{}{}
		}
	}
	r.nativeImageMu.Unlock()

	for _, p := range cTextPtrs {
		C.free(unsafe.Pointer(p))
	}
	for _, p := range cImagePtrs {
		C.free(p)
	}
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

	result := C.uiMeasureText(
		cText,
		C.int32_t(len(text)),
		C.float(fontSize),
		familyPtr, familyLen,
	)
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

func (r *MacRenderer) SetDarkMode(dark bool) {
	C.uiWindowSetDarkMode(C.int32_t(r.windowID), C.bool(dark))
}

func (r *MacRenderer) ReleaseMemory() {
	C.uiWindowReleaseMemory(C.int32_t(r.windowID))
	r.nativeImageMu.Lock()
	r.nativeImageKeys = make(map[string]struct{})
	r.nativeImageMu.Unlock()
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

// RunMessageLoop on macOS is a no-op: the Cocoa event loop ([NSApp run]) is
// already running (started by mainthread_darwin.m os_main). We just store the
// onRender callback so drawRect: (via uiDarwinOnDraw) can retrieve commands
// from the Go layout engine on demand. Blocking here would freeze the main
// thread and prevent dispatch_async blocks (Show/Hide/Resize) from running.
// The process stays alive because run() falls through to StartWebsocketAndWait
// after gpuUI.Run returns.
func (r *MacRenderer) RunMessageLoop(onRender func() *CommandList) {
	renderCallbackMu.Lock()
	renderCallback = onRender
	renderCallbackMu.Unlock()
}

// RequestRepaint triggers a native repaint via setNeedsDisplay:.
func (r *MacRenderer) RequestRepaint() {
	C.uiWindowInvalidate(C.int32_t(r.windowID))
}

// WindowError describes a native window operation failure.
type WindowError struct {
	Op  string
	Err string
}

func (e *WindowError) Error() string { return e.Op + ": " + e.Err }