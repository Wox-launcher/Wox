//go:build darwin

package woxui

/*
#cgo CFLAGS: -fblocks -Wno-deprecated-declarations
#cgo LDFLAGS: -framework Cocoa -framework Metal -framework QuartzCore -framework CoreText -framework CoreGraphics -framework WebKit
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
	mu         sync.Mutex
	native     *C.WoxDarwinWindow
	options    WindowOptions
	handle     cgo.Handle
	closing    bool
	closed     bool
	renderErr  error
	fontFamily string
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

func platformCall(fn func()) error {
	handle := cgo.NewHandle(fn)
	defer handle.Delete()
	if C.wox_darwin_call(C.uintptr_t(handle)) != 0 {
		return errors.New("woxui: AppKit runtime is not running")
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
	applicationWindow := C.int32_t(0)
	if options.Role == WindowRoleApplication {
		applicationWindow = 1
	}
	window.native = C.wox_darwin_window_create(
		title,
		C.float(options.Size.Width),
		C.float(options.Size.Height),
		hideOnBlur,
		applicationWindow,
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

func (w *platformWindow) setBounds(bounds Rect) error {
	native, err := w.openNative()
	if err != nil {
		return err
	}
	if C.wox_darwin_window_set_bounds(native, C.float(bounds.X), C.float(bounds.Y), C.float(bounds.Width), C.float(bounds.Height)) != 0 {
		return errors.New("woxui: failed to set macOS window bounds")
	}
	return nil
}

func (w *platformWindow) bounds() (Rect, error) {
	native, err := w.openNative()
	if err != nil {
		return Rect{}, err
	}
	var x, y, width, height C.float
	if C.wox_darwin_window_get_bounds(native, &x, &y, &width, &height) != 0 {
		return Rect{}, errors.New("woxui: failed to read macOS window bounds")
	}
	return Rect{X: float32(x), Y: float32(y), Width: float32(width), Height: float32(height)}, nil
}

func (w *platformWindow) capturePNG(path string) error {
	native, err := w.openNative()
	if err != nil {
		return err
	}
	nativePath := C.CString(path)
	defer C.free(unsafe.Pointer(nativePath))
	if C.wox_darwin_window_capture_png(native, nativePath) != 0 {
		return errors.New("woxui: failed to capture macOS window")
	}
	return nil
}

func (w *platformWindow) center(size Size) error {
	native, err := w.openNative()
	if err != nil {
		return err
	}
	if C.wox_darwin_window_center(native, C.float(size.Width), C.float(size.Height)) != 0 {
		return errors.New("woxui: failed to center macOS window")
	}
	return nil
}

func (w *platformWindow) startDragging() error {
	native, err := w.openNative()
	if err != nil {
		return err
	}
	if C.wox_darwin_window_start_dragging(native) != 0 {
		return errors.New("woxui: failed to start macOS window drag")
	}
	return nil
}

func (w *platformWindow) setHideOnBlur(enabled bool) error {
	native, err := w.openNative()
	if err != nil {
		return err
	}
	nativeEnabled := C.int32_t(0)
	if enabled {
		nativeEnabled = 1
	}
	if C.wox_darwin_window_set_hide_on_blur(native, nativeEnabled) != 0 {
		return errors.New("woxui: failed to update macOS hide-on-blur behavior")
	}
	return nil
}

func (w *platformWindow) setFontFamily(family string) error {
	w.mu.Lock()
	w.fontFamily = family
	w.mu.Unlock()
	return w.invalidate()
}

func (w *platformWindow) pickFile(options FileDialogOptions) (string, error) {
	native, err := w.openNative()
	if err != nil {
		return "", err
	}
	directory := C.int32_t(0)
	if options.Directory {
		directory = 1
	}
	var path *C.char
	result := C.wox_darwin_window_pick_file(native, directory, &path)
	if result == 1 {
		return "", nil
	}
	if result != 0 {
		return "", errors.New("woxui: failed to open macOS file dialog")
	}
	if path == nil {
		return "", errors.New("woxui: macOS file dialog returned no path")
	}
	defer C.free(unsafe.Pointer(path))
	return C.GoString(path), nil
}

func (w *platformWindow) openExternalURL(rawURL string) error {
	native, err := w.openNative()
	if err != nil {
		return err
	}
	nativeURL := C.CString(rawURL)
	defer C.free(unsafe.Pointer(nativeURL))
	if C.wox_darwin_window_open_external_url(native, nativeURL) != 0 {
		return errors.New("woxui: failed to open external URL on macOS")
	}
	return nil
}

func (w *platformWindow) showWebView(content WebViewContent, bounds Rect) error {
	native, err := w.openNative()
	if err != nil {
		return err
	}
	url := C.CString(content.URL)
	html := C.CString(content.HTML)
	css := C.CString(content.InjectCSS)
	cacheKey := C.CString(content.CacheKey)
	defer C.free(unsafe.Pointer(url))
	defer C.free(unsafe.Pointer(html))
	defer C.free(unsafe.Pointer(css))
	defer C.free(unsafe.Pointer(cacheKey))
	cacheDisabled := C.int32_t(0)
	if content.CacheDisabled {
		cacheDisabled = 1
	}
	if C.wox_darwin_window_show_webview(native, url, html, css, cacheDisabled, cacheKey, C.float(bounds.X), C.float(bounds.Y), C.float(bounds.Width), C.float(bounds.Height)) != 0 {
		return errors.New("woxui: failed to show macOS WebView")
	}
	return nil
}

func (w *platformWindow) hideWebView() error {
	native, err := w.openNative()
	if err != nil {
		return err
	}
	if C.wox_darwin_window_hide_webview(native) != 0 {
		return errors.New("woxui: failed to hide macOS WebView")
	}
	return nil
}

func (w *platformWindow) writeClipboardText(text string) error {
	native, err := w.openNative()
	if err != nil {
		return err
	}
	nativeText := C.CString(text)
	defer C.free(unsafe.Pointer(nativeText))
	if C.wox_darwin_window_write_clipboard_text(native, nativeText) != 0 {
		return errors.New("woxui: failed to write macOS clipboard text")
	}
	return nil
}

func (w *platformWindow) writeClipboardImage(image *clipboardImage) error {
	native, err := w.openNative()
	if err != nil {
		return err
	}
	if image == nil || len(image.pixels) == 0 {
		return errors.New("woxui: clipboard image is empty")
	}
	if C.wox_darwin_window_write_clipboard_image(
		native,
		(*C.uint8_t)(unsafe.Pointer(&image.pixels[0])),
		C.int32_t(image.width),
		C.int32_t(image.height),
		C.int32_t(image.stride),
	) != 0 {
		return errors.New("woxui: failed to write macOS clipboard image")
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

// setTextInputState updates NSTextInputClient activation and candidate geometry on the AppKit thread.
func (w *platformWindow) setTextInputState(state TextInputState) error {
	native, err := w.openNative()
	if err != nil {
		return err
	}
	enabled := C.int32_t(0)
	if state.Enabled {
		enabled = 1
	}
	if C.wox_darwin_window_set_text_input_state(
		native,
		enabled,
		C.float(state.CursorRect.X),
		C.float(state.CursorRect.Y),
		C.float(state.CursorRect.Width),
		C.float(state.CursorRect.Height),
	) != 0 {
		return errors.New("woxui: failed to update macOS text input state")
	}
	return nil
}

// measureText uses CoreText on the AppKit thread so it matches the native renderer.
func (w *platformWindow) measureText(text string, style TextStyle) (TextMetrics, error) {
	native, err := w.openNative()
	if err != nil {
		return TextMetrics{}, err
	}
	nativeText := C.CString(text)
	defer C.free(unsafe.Pointer(nativeText))
	w.mu.Lock()
	fontFamily := w.fontFamily
	w.mu.Unlock()
	nativeFontFamily := C.CString(fontFamily)
	defer C.free(unsafe.Pointer(nativeFontFamily))
	var width C.float
	var height C.float
	var baseline C.float
	result := C.wox_darwin_window_measure_text(native, nativeText, nativeFontFamily, C.float(style.Size), C.uint8_t(style.Weight), &width, &height, &baseline)
	if result != 0 {
		return TextMetrics{}, errors.New("woxui: failed to measure macOS text")
	}
	return TextMetrics{Size: Size{Width: float32(width), Height: float32(height)}, Baseline: float32(baseline)}, nil
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
	w.markClosed()
	return nil
}

func (w *platformWindow) markClosed() {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.native = nil
	w.closing = false
	w.closed = true
	handle := w.handle
	w.handle = 0
	onClosed := w.options.OnClosed
	w.mu.Unlock()
	if handle != 0 {
		handle.Delete()
	}
	if onClosed != nil {
		onClosed()
	}
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
	w.mu.Lock()
	fontFamily := w.fontFamily
	w.mu.Unlock()
	nativeFontFamily := C.CString(fontFamily)
	defer C.free(unsafe.Pointer(nativeFontFamily))
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
		case displayCommandStrokeRoundedRect:
			result = C.wox_darwin_window_stroke_rounded_rect(
				native,
				C.float(command.rect.X),
				C.float(command.rect.Y),
				C.float(command.rect.Width),
				C.float(command.rect.Height),
				C.float(command.radius),
				C.float(command.stroke),
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
				nativeFontFamily,
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
		case displayCommandDrawImage:
			result = C.wox_darwin_window_draw_image(
				native,
				(*C.uint8_t)(unsafe.Pointer(&command.image.pixels[0])),
				C.int32_t(command.image.Width),
				C.int32_t(command.image.Height),
				C.int32_t(command.image.Width*4),
				C.float(command.rect.X),
				C.float(command.rect.Y),
				C.float(command.rect.Width),
				C.float(command.rect.Height),
			)
		case displayCommandSetClipRect:
			result = C.wox_darwin_window_set_clip_rect(native, C.float(command.rect.X), C.float(command.rect.Y), C.float(command.rect.Width), C.float(command.rect.Height))
		case displayCommandClearClip:
			result = C.wox_darwin_window_clear_clip(native)
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

//export woxGoDarwinCall
func woxGoDarwinCall(context C.uintptr_t) {
	cgo.Handle(context).Value().(func())()
}

//export woxGoDarwinCloseRequested
func woxGoDarwinCloseRequested(context C.uintptr_t) {
	window := cgo.Handle(context).Value().(*platformWindow)
	if window.options.OnCloseRequested != nil {
		window.options.OnCloseRequested()
		return
	}
	go func() {
		if err := window.close(); err != nil {
			window.recordRenderError("close requested window", -1)
		}
	}()
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

// woxGoDarwinKey forwards a normalized AppKit key event into the window callback.
//
//export woxGoDarwinKey
func woxGoDarwinKey(context C.uintptr_t, key *C.char, modifiers C.uint8_t, down C.int32_t, repeat C.int32_t, composing C.int32_t) C.int32_t {
	window := cgo.Handle(context).Value().(*platformWindow)
	if window.options.OnKey == nil {
		return 0
	}
	handled := window.options.OnKey(KeyEvent{
		Key:       Key(C.GoString(key)),
		Modifiers: KeyModifiers(modifiers),
		Down:      down != 0,
		Repeat:    repeat != 0,
		Composing: composing != 0,
	})
	if handled {
		return 1
	}
	return 0
}

// woxGoDarwinTextInput forwards NSTextInputClient commit and marked-text changes.
//
//export woxGoDarwinTextInput
func woxGoDarwinTextInput(context C.uintptr_t, kind C.uint8_t, text *C.char) {
	window := cgo.Handle(context).Value().(*platformWindow)
	if window.options.OnTextInput != nil {
		window.options.OnTextInput(TextInputEvent{Kind: TextInputEventKind(kind), Text: C.GoString(text)})
	}
}

// woxGoDarwinPointer forwards AppKit mouse and trackpad events in logical coordinates.
//
//export woxGoDarwinPointer
func woxGoDarwinPointer(context C.uintptr_t, kind C.uint8_t, x C.float, y C.float, button C.uint8_t, scrollX C.float, scrollY C.float, modifiers C.uint8_t) {
	window := cgo.Handle(context).Value().(*platformWindow)
	if window.options.OnPointer != nil {
		window.options.OnPointer(PointerEvent{
			Kind:      PointerEventKind(kind),
			Position:  Point{X: float32(x), Y: float32(y)},
			Button:    PointerButton(button),
			Scroll:    Point{X: float32(scrollX), Y: float32(scrollY)},
			Modifiers: KeyModifiers(modifiers),
		})
	}
}
