//go:build cgo

package ui

/*
#include <stdlib.h>
#include "ui_native.h"
*/
import "C"
import (
	"sync"
	"unsafe"
)

// baseRenderer holds shared state and Go→C conversion logic used by all
// platform renderers. Platform renderers embed this struct and only
// implement the thin platform-specific C calls (uiWindowShow, uiWindowHide,
// etc.). All shared plumbing — command conversion, image cache tracking,
// event dispatch — lives here.
type baseRenderer struct {
	windowID int32

	// self is set by the platform constructor to the concrete NativeRenderer
	// (e.g. *MacRenderer). C→Go callbacks (uiEventCallback, uiDarwinOnDraw)
	// find the baseRenderer via the activeRenderers registry, then use self
	// to call back into the full NativeRenderer interface without knowing
	// the concrete platform type.
	self NativeRenderer

	// Image cache: tracks which ImageKeys have already been uploaded to
	// native GPU memory so we skip re-uploading identical data.
	nativeImageMu   sync.Mutex
	nativeImageKeys map[string]struct{}

	// Event callback — per-renderer, not package-global. C→Go callbacks
	// find the renderer via activeRenderers, then call dispatchEvent.
	eventHandler EventCallback

	// Render callback — used by macOS drawRect: re-entrant rendering to
	// pull a fresh CommandList from the Go layout engine. Per-renderer,
	// not package-global. Windows does not use this field.
	renderCallback func() *CommandList
}

// toCCommands converts a slice of Go DrawCommands to C DrawCommands.
// Allocates C memory for text and image data (must be freed via freeCCommands
// after the native render call completes). Returns the C array plus slices
// of pointers to free.
func (b *baseRenderer) toCCommands(cmds []DrawCommand) (
	cCmds []C.CDrawCommand, textPtrs []*C.char, imagePtrs []unsafe.Pointer,
) {
	cCmds = make([]C.CDrawCommand, len(cmds))

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
			textPtrs = append(textPtrs, cstr)
			cCmds[i].text = cstr
			cCmds[i].textLen = C.int32_t(len(cmd.Text))
		}
		if cmd.FontFamily != "" {
			cstr := C.CString(cmd.FontFamily)
			textPtrs = append(textPtrs, cstr)
			cCmds[i].fontFamily = cstr
			cCmds[i].fontFamilyLen = C.int32_t(len(cmd.FontFamily))
		}

		uploadImage := len(cmd.ImageData) > 0
		if cmd.ImageKey != "" {
			cstr := C.CString(cmd.ImageKey)
			textPtrs = append(textPtrs, cstr)
			cCmds[i].imageKey = cstr
			cCmds[i].imageKeyLen = C.int32_t(len(cmd.ImageKey))

			b.nativeImageMu.Lock()
			_, uploaded := b.nativeImageKeys[cmd.ImageKey]
			b.nativeImageMu.Unlock()
			if uploaded {
				uploadImage = false
			}
		}
		if uploadImage {
			cdata := C.CBytes(cmd.ImageData)
			imagePtrs = append(imagePtrs, cdata)
			cCmds[i].imageData = (*C.uint8_t)(cdata)
			cCmds[i].imageLen = C.int32_t(len(cmd.ImageData))
		}
	}

	return cCmds, textPtrs, imagePtrs
}

// freeCCommands releases C-allocated memory after a native render call.
func (b *baseRenderer) freeCCommands(textPtrs []*C.char, imagePtrs []unsafe.Pointer) {
	for _, p := range textPtrs {
		C.free(unsafe.Pointer(p))
	}
	for _, p := range imagePtrs {
		C.free(p)
	}
}

// trackUploadedImages records image keys that were included in the last
// command list so future frames skip re-uploading the same bitmap data.
func (b *baseRenderer) trackUploadedImages(cmds []DrawCommand) {
	b.nativeImageMu.Lock()
	for _, cmd := range cmds {
		if cmd.ImageKey != "" && len(cmd.ImageData) > 0 {
			b.nativeImageKeys[cmd.ImageKey] = struct{}{}
		}
	}
	b.nativeImageMu.Unlock()
}

// clearImageCache resets the native image cache tracking. Called by
// platform ReleaseMemory implementations.
func (b *baseRenderer) clearImageCache() {
	b.nativeImageMu.Lock()
	b.nativeImageKeys = make(map[string]struct{})
	b.nativeImageMu.Unlock()
}

// dispatchEvent invokes the registered event handler. Called by the
// platform's C→Go uiEventCallback export after finding the renderer
// via the activeRenderers registry.
func (b *baseRenderer) dispatchEvent(ev Event) {
	if b.eventHandler != nil {
		b.eventHandler(ev)
	}
}

// parseCEvent builds a Go Event from the raw C callback parameters.
// Shared between Windows and macOS uiEventCallback exports.
func parseCEvent(eventType, key, mods C.int32_t,
	text *C.char, textLen C.int32_t,
	composeText *C.char, composeTextLen C.int32_t, composeCursor C.int32_t,
	x, y, deltaY C.float, width, height C.int32_t,
) Event {
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

	return ev
}