//go:build windows && cgo

package ui

/*
#cgo CFLAGS: -DUNICODE -D_UNICODE
#cgo LDFLAGS: -ld2d1 -ldwrite -ldwmapi -lwindowscodecs -lole32 -luuid -luser32 -lgdi32 -lshcore -ld3d11 -ldxgi -ldcomp

#include <stdlib.h>
#include <stdint.h>
#include <stdbool.h>

// DrawCommand mirrors the Go DrawCommand struct.
// All coordinates are in DIP (logical pixels).
typedef struct {
    int32_t cmd_type;
    float x, y, w, h;
    float r, g, b, a;
    float radius;
    float strokeWidth;
    const char* text;       // UTF-8, owned by Go (valid during Render call)
    int32_t textLen;
    float fontSize;
    const char* fontFamily;  // UTF-8, owned by Go
    int32_t fontFamilyLen;
    const uint8_t* imageData; // PNG bytes, owned by Go
    int32_t imageLen;
    const char* imageKey;     // UTF-8 cache key, owned by Go
    int32_t imageKeyLen;
    float imageWidth, imageHeight;
} CDrawCommand;

// WindowConfig controls native window creation.
typedef struct {
    int32_t width;
    int32_t height;
    float cornerRadius;
    bool frameless;
    bool transparent;
    bool darkMode;
} CWindowConfig;

// MeasureResult returns text dimensions.
typedef struct {
    float width;
    float height;
} CMeasureResult;

// Native functions implemented in ui_windows.c
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
extern void uiWindowRender(int32_t windowId, const CDrawCommand* commands, int32_t count);
extern CMeasureResult uiMeasureText(const char* text, int32_t textLen, float fontSize, const char* fontFamily, int32_t fontFamilyLen);

// Message loop — returns false when WM_QUIT is received.
extern bool uiPumpMessages(void);
extern void uiInvalidateWindow(int32_t windowId);

// Event callback — the C side calls this Go function for input events.
extern void uiEventCallback(int32_t windowId, int32_t eventType, int32_t key, int32_t mods,
    char* text, int32_t textLen,
    char* composeText, int32_t composeTextLen, int32_t composeCursor,
    float x, float y, float deltaY,
    int32_t width, int32_t height);
*/
import "C"
import (
	"sync"
	"unsafe"
)

// WindowsRenderer implements the Renderer interface using Direct2D/DirectWrite.
type WindowsRenderer struct {
	windowID int32

	nativeImageMu   sync.Mutex
	nativeImageKeys map[string]struct{}
}

// WindowsTextMeasurer implements TextMeasurer using DirectWrite.
type WindowsTextMeasurer struct{}

var (
	rendererMu       sync.Mutex
	rendererRegistry = make(map[int32]*WindowsRenderer)
	eventHandler     EventCallback
)

// NewWindowsRenderer creates a native window and returns a Renderer.
// The window starts hidden — call Show() to display it.
func NewWindowsRenderer(width, height int, theme Theme) (*WindowsRenderer, error) {
	// Treat the window as dark when the theme background luminance is low.
	// This drives DWMWA_USE_IMMERSIVE_DARK_MODE so the Mica/Acrylic backdrop
	// renders in the tone that matches the theme.
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
		windowID:        goID,
		nativeImageKeys: make(map[string]struct{}),
	}
	rendererMu.Lock()
	rendererRegistry[goID] = r
	rendererMu.Unlock()
	return r, nil
}

// SetEventHandler registers the global event callback for native input events.
// Must be called before Show().
func SetEventHandler(cb EventCallback) {
	eventHandler = cb
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

	if eventHandler != nil {
		eventHandler(ev)
	}
}

// Render executes the command list on the native Direct2D surface.
func (r *WindowsRenderer) Render(commands *CommandList) error {
	if len(commands.Commands) == 0 {
		return nil
	}

	// Convert Go DrawCommands to C DrawCommands.
	// Text and image data are copied to C-allocated memory to comply with
	// CGO's "no Go pointers in C" rule.
	cCmds := make([]C.CDrawCommand, len(commands.Commands))
	var cTextPtrs []*C.char // free after Render
	var cImagePtrs []unsafe.Pointer

	for i, cmd := range commands.Commands {
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
	for _, cmd := range commands.Commands {
		if cmd.ImageKey != "" && len(cmd.ImageData) > 0 {
			r.nativeImageKeys[cmd.ImageKey] = struct{}{}
		}
	}
	r.nativeImageMu.Unlock()

	// Free C-allocated strings and image data.
	for _, p := range cTextPtrs {
		C.free(unsafe.Pointer(p))
	}
	for _, p := range cImagePtrs {
		C.free(p)
	}
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

	// Copy to C memory to comply with CGO pointer rules.
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

// Show makes the window visible.
func (r *WindowsRenderer) Show() error {
	C.uiWindowShow(C.int32_t(r.windowID))
	return nil
}

// Hide hides the window.
func (r *WindowsRenderer) Hide() error {
	C.uiWindowHide(C.int32_t(r.windowID))
	return nil
}

// SetDarkMode toggles the DWM immersive dark mode attribute so the Mica
// backdrop renders in the tone matching the active theme.
func (r *WindowsRenderer) SetDarkMode(dark bool) {
	C.uiWindowSetDarkMode(C.int32_t(r.windowID), C.bool(dark))
}

// ReleaseMemory drops native caches that are only useful while the launcher is visible.
func (r *WindowsRenderer) ReleaseMemory() {
	C.uiWindowReleaseMemory(C.int32_t(r.windowID))
	r.nativeImageMu.Lock()
	r.nativeImageKeys = make(map[string]struct{})
	r.nativeImageMu.Unlock()
}

// SetPosition moves the window to absolute screen coordinates.
func (r *WindowsRenderer) SetPosition(x, y int) error {
	C.uiWindowSetPosition(C.int32_t(r.windowID), C.int32_t(x), C.int32_t(y))
	return nil
}

// SetSize resizes the window.
func (r *WindowsRenderer) SetSize(w, h int) error {
	C.uiWindowSetSize(C.int32_t(r.windowID), C.int32_t(w), C.int32_t(h))
	return nil
}

// Close destroys the window.
func (r *WindowsRenderer) Close() error {
	C.uiWindowDestroy(C.int32_t(r.windowID))
	rendererMu.Lock()
	delete(rendererRegistry, r.windowID)
	rendererMu.Unlock()
	return nil
}

// IsVisible returns whether the window is shown.
func (r *WindowsRenderer) IsVisible() bool {
	return bool(C.uiWindowIsVisible(C.int32_t(r.windowID)))
}

// GetSize returns the current logical (DIP) window dimensions.
func (r *WindowsRenderer) GetSize() (int, int) {
	var w, h C.int32_t
	C.uiWindowGetSize(C.int32_t(r.windowID), &w, &h)
	return int(w), int(h)
}

// RunMessageLoop runs the native Win32 message loop.
// Blocks until Close() is called or the window is destroyed.
// The onRender callback is invoked each frame to produce draw commands.
// onShouldRender returns false when the window is hidden, letting the loop
// block on GetMessage without busy-rendering an invisible surface.
func (r *WindowsRenderer) RunMessageLoop(onRender func() *CommandList) {
	for {
		// GetMessage blocks until a message arrives, so the loop does not
		// busy-poll when the window is hidden or idle.
		if !C.uiPumpMessages() {
			break // WM_QUIT received
		}

		// Render a frame after processing messages. The Go onRender callback
		// decides whether to produce commands (it checks g.visible internally).
		cmds := onRender()
		if cmds != nil {
			r.Render(cmds)
		}
	}
}

// RequestRepaint triggers a repaint of the window.
func (r *WindowsRenderer) RequestRepaint() {
	C.uiInvalidateWindow(C.int32_t(r.windowID))
}

// WindowError describes a native window operation failure.
type WindowError struct {
	Op  string
	Err string
}

func (e *WindowError) Error() string { return e.Op + ": " + e.Err }
