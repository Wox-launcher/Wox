//go:build darwin

package woxui

/*
#cgo CFLAGS: -fblocks -Wno-deprecated-declarations
#cgo LDFLAGS: -framework Cocoa -framework Metal -framework QuartzCore -framework CoreText -framework CoreGraphics
#include <stdlib.h>
#include "native_darwin.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"runtime"
	"runtime/cgo"
	"sync"
	"unsafe"
)

type darwinRunState struct {
	start     func() error
	err       error
	mu        sync.Mutex
	accepting bool
	windows   []*platformWindow
}

var darwinRuntime struct {
	sync.Mutex
	current *darwinRunState
}

type platformWindow struct {
	mu        sync.Mutex
	native    *C.WoxDarwinWindow
	options   WindowOptions
	handle    cgo.Handle
	closing   bool
	closed    bool
	renderErr error
}

// AppKit requires the package's main goroutine to remain on the process main thread.
func init() {
	runtime.LockOSThread()
}

func platformRun(start func() error) error {
	state := &darwinRunState{start: start, accepting: true}
	darwinRuntime.Lock()
	if darwinRuntime.current != nil {
		darwinRuntime.Unlock()
		return errors.New("woxui: Run is already active on macOS")
	}
	darwinRuntime.current = state
	darwinRuntime.Unlock()
	defer func() {
		darwinRuntime.Lock()
		darwinRuntime.current = nil
		darwinRuntime.Unlock()
	}()

	handle := cgo.NewHandle(state)
	result := C.wox_darwin_run(C.uintptr_t(handle))
	handle.Delete()

	if state.err != nil {
		return state.err
	}
	if result == -2 {
		return errors.New("woxui: Run must be called from the process main goroutine on macOS")
	}
	if result != 0 {
		return fmt.Errorf("woxui: AppKit event loop failed with status %d", int32(result))
	}
	return nil
}

func openPlatformWindow(options WindowOptions) (*platformWindow, error) {
	darwinRuntime.Lock()
	run := darwinRuntime.current
	darwinRuntime.Unlock()
	if run != nil {
		run.mu.Lock()
		accepting := run.accepting
		run.mu.Unlock()
		if !accepting {
			run = nil
		}
	}
	if run == nil {
		return nil, errors.New("woxui: Open must be called from Run's start callback or a UI callback on macOS")
	}

	window := &platformWindow{options: options}
	window.handle = cgo.NewHandle(window)
	title := C.CString(options.Title)
	defer C.free(unsafe.Pointer(title))

	hideOnBlur := C.int32_t(0)
	if options.HideOnBlur {
		hideOnBlur = 1
	}
	window.native = C.wox_darwin_window_create(
		title,
		C.float(options.Size.Width),
		C.float(options.Size.Height),
		hideOnBlur,
		C.uintptr_t(window.handle),
	)
	if window.native == nil {
		window.handle.Delete()
		return nil, errors.New("woxui: failed to create AppKit window or Metal renderer")
	}
	run.mu.Lock()
	run.windows = append(run.windows, window)
	run.mu.Unlock()
	return window, nil
}

func (w *platformWindow) show() (FocusEpoch, error) {
	native, err := w.openNative()
	if err != nil {
		return 0, err
	}
	epoch := C.wox_darwin_window_show(native)
	if epoch == 0 {
		return 0, errors.New("woxui: failed to show macOS window")
	}
	return FocusEpoch(epoch), nil
}

func (w *platformWindow) hide() error {
	native, err := w.openNative()
	if err != nil {
		return err
	}
	if C.wox_darwin_window_hide(native) != 0 {
		return errors.New("woxui: failed to hide macOS window")
	}
	return nil
}

func (w *platformWindow) invalidate() error {
	w.mu.Lock()
	if w.renderErr != nil {
		err := w.renderErr
		w.mu.Unlock()
		return err
	}
	w.mu.Unlock()

	native, err := w.openNative()
	if err != nil {
		return err
	}
	if C.wox_darwin_window_invalidate(native) != 0 {
		return errors.New("woxui: failed to invalidate macOS window")
	}
	return nil
}

func (w *platformWindow) close() error {
	w.mu.Lock()
	if w.closed || w.closing {
		w.mu.Unlock()
		return nil
	}
	w.closing = true
	native := w.native
	w.mu.Unlock()

	if native == nil || C.wox_darwin_window_close(native) != 0 {
		w.mu.Lock()
		w.closing = false
		w.mu.Unlock()
		return errors.New("woxui: failed to close macOS window")
	}

	// Native close drains onto the AppKit thread and clears its callback context before the handle is deleted.
	w.mu.Lock()
	w.native = nil
	w.closing = false
	w.closed = true
	handle := w.handle
	w.handle = 0
	w.mu.Unlock()
	if handle != 0 {
		handle.Delete()
	}
	return nil
}

func (w *platformWindow) openNative() (*C.WoxDarwinWindow, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed || w.closing || w.native == nil {
		return nil, errors.New("woxui: window is closed")
	}
	return w.native, nil
}

func (w *platformWindow) recordRenderError(operation string, result C.int32_t) {
	w.mu.Lock()
	if w.renderErr == nil {
		w.renderErr = fmt.Errorf("woxui: %s failed with status %d", operation, int32(result))
	}
	w.mu.Unlock()
}

func (w *platformWindow) drawFrame(frame FrameInfo) {
	displayList := &DisplayList{}
	if w.options.OnFrame != nil {
		w.options.OnFrame(displayList, frame)
	}

	native, err := w.openNative()
	if err != nil {
		return
	}
	result := C.wox_darwin_window_begin_frame(
		native,
		C.float(frame.Size.Width),
		C.float(frame.Size.Height),
		C.float(frame.Scale),
		C.uint8_t(displayList.clearColor.R),
		C.uint8_t(displayList.clearColor.G),
		C.uint8_t(displayList.clearColor.B),
		C.uint8_t(displayList.clearColor.A),
	)
	if result > 0 {
		return
	}
	if result < 0 {
		w.recordRenderError("begin Metal frame", result)
		return
	}

	for _, command := range displayList.commands {
		switch command.kind {
		case displayCommandFillRoundedRect:
			result = C.wox_darwin_window_fill_rounded_rect(
				native,
				C.float(command.rect.X),
				C.float(command.rect.Y),
				C.float(command.rect.Width),
				C.float(command.rect.Height),
				C.float(command.radius),
				C.uint8_t(command.color.R),
				C.uint8_t(command.color.G),
				C.uint8_t(command.color.B),
				C.uint8_t(command.color.A),
			)
		case displayCommandDrawText:
			text := C.CString(command.text)
			result = C.wox_darwin_window_draw_text(
				native,
				text,
				C.float(command.rect.X),
				C.float(command.rect.Y),
				C.float(command.rect.Width),
				C.float(command.rect.Height),
				C.float(command.style.Size),
				C.uint8_t(command.style.Weight),
				C.uint8_t(command.color.R),
				C.uint8_t(command.color.G),
				C.uint8_t(command.color.B),
				C.uint8_t(command.color.A),
			)
			C.free(unsafe.Pointer(text))
		}
		if result != 0 {
			_ = C.wox_darwin_window_end_frame(native)
			w.recordRenderError("encode Metal frame", result)
			return
		}
	}

	if result = C.wox_darwin_window_end_frame(native); result != 0 {
		w.recordRenderError("present Metal frame", result)
	}
}

//export woxGoDarwinStart
func woxGoDarwinStart(context C.uintptr_t) C.int32_t {
	state := cgo.Handle(context).Value().(*darwinRunState)
	state.err = state.start()
	if state.err != nil {
		state.mu.Lock()
		state.accepting = false
		windows := append([]*platformWindow(nil), state.windows...)
		state.mu.Unlock()
		for _, window := range windows {
			_ = window.close()
		}
		return -1
	}
	return 0
}

//export woxGoDarwinFrame
func woxGoDarwinFrame(context C.uintptr_t, width C.float, height C.float, pixelWidth C.int32_t, pixelHeight C.int32_t, scale C.float) {
	window := cgo.Handle(context).Value().(*platformWindow)
	window.drawFrame(FrameInfo{
		Size:      Size{Width: float32(width), Height: float32(height)},
		PixelSize: PixelSize{Width: int(pixelWidth), Height: int(pixelHeight)},
		Scale:     float32(scale),
	})
}

//export woxGoDarwinFocus
func woxGoDarwinFocus(context C.uintptr_t, epoch C.uint64_t, active C.int32_t) {
	window := cgo.Handle(context).Value().(*platformWindow)
	if window.options.OnFocus != nil {
		window.options.OnFocus(FocusEvent{Epoch: FocusEpoch(epoch), Active: active != 0})
	}
}
