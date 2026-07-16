//go:build linux

package woxui

/*
#cgo CFLAGS: -std=c11 -Wall -Wextra -Werror
#cgo pkg-config: gtk+-3.0 epoxy x11
#cgo LDFLAGS: -ldl -lm
#include <stdlib.h>
#include "native_linux.h"
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

type linuxRunState struct {
	start     func() error
	err       error
	mu        sync.Mutex
	accepting bool
	windows   []*platformWindow
}

var linuxRuntime struct {
	sync.Mutex
	current *linuxRunState
}

type platformWindow struct {
	mu         sync.Mutex
	native     *C.WoxLinuxWindow
	options    WindowOptions
	handle     cgo.Handle
	closing    bool
	closed     bool
	renderErr  error
	fontFamily string
}

// GTK and its OpenGL context stay on the process main thread for the runtime lifetime.
func init() {
	runtime.LockOSThread()
}

func platformRun(start func() error) error {
	state := &linuxRunState{start: start, accepting: true}
	linuxRuntime.Lock()
	if linuxRuntime.current != nil {
		linuxRuntime.Unlock()
		return errors.New("woxui: Run is already active on Linux")
	}
	linuxRuntime.current = state
	linuxRuntime.Unlock()
	defer func() {
		linuxRuntime.Lock()
		linuxRuntime.current = nil
		linuxRuntime.Unlock()
	}()

	handle := cgo.NewHandle(state)
	result := C.wox_linux_run(C.uintptr_t(handle))
	handle.Delete()
	if state.err != nil {
		return state.err
	}
	if result == -2 {
		return errors.New("woxui: GTK could not connect to a Linux display")
	}
	if result != 0 {
		return fmt.Errorf("woxui: GTK event loop failed with status %d", int32(result))
	}
	return nil
}

func openPlatformWindow(options WindowOptions) (*platformWindow, error) {
	linuxRuntime.Lock()
	run := linuxRuntime.current
	linuxRuntime.Unlock()
	if run != nil {
		run.mu.Lock()
		accepting := run.accepting
		run.mu.Unlock()
		if !accepting {
			run = nil
		}
	}
	if run == nil {
		return nil, errors.New("woxui: Open must be called from Run's start callback or a UI callback on Linux")
	}

	window := &platformWindow{options: options}
	window.handle = cgo.NewHandle(window)
	title := C.CString(options.Title)
	defer C.free(unsafe.Pointer(title))
	hideOnBlur := C.int32_t(0)
	if options.HideOnBlur {
		hideOnBlur = 1
	}
	window.native = C.wox_linux_window_create(
		title,
		C.float(options.Size.Width),
		C.float(options.Size.Height),
		hideOnBlur,
		C.uintptr_t(window.handle),
	)
	if window.native == nil {
		window.handle.Delete()
		return nil, errors.New("woxui: failed to create GTK window or OpenGL renderer")
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
	epoch := C.wox_linux_window_show(native)
	if epoch == 0 {
		return 0, errors.New("woxui: failed to show Linux window")
	}
	return FocusEpoch(epoch), nil
}

func (w *platformWindow) hide() error {
	native, err := w.openNative()
	if err != nil {
		return err
	}
	if C.wox_linux_window_hide(native) != 0 {
		return errors.New("woxui: failed to hide Linux window")
	}
	return nil
}

func (w *platformWindow) setBounds(bounds Rect) error {
	native, err := w.openNative()
	if err != nil {
		return err
	}
	if C.wox_linux_window_set_bounds(native, C.float(bounds.X), C.float(bounds.Y), C.float(bounds.Width), C.float(bounds.Height)) != 0 {
		return errors.New("woxui: failed to set Linux window bounds")
	}
	return nil
}

func (w *platformWindow) center(size Size) error {
	native, err := w.openNative()
	if err != nil {
		return err
	}
	if C.wox_linux_window_center(native, C.float(size.Width), C.float(size.Height)) != 0 {
		return errors.New("woxui: failed to center Linux window")
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
	if C.wox_linux_window_set_hide_on_blur(native, nativeEnabled) != 0 {
		return errors.New("woxui: failed to update Linux hide-on-blur behavior")
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
	result := C.wox_linux_window_pick_file(native, directory, &path)
	if result == 1 {
		return "", nil
	}
	if result != 0 {
		return "", errors.New("woxui: failed to open Linux file dialog")
	}
	if path == nil {
		return "", errors.New("woxui: Linux file dialog returned no path")
	}
	defer C.wox_linux_free_string(path)
	return C.GoString(path), nil
}

func (w *platformWindow) openExternalURL(rawURL string) error {
	native, err := w.openNative()
	if err != nil {
		return err
	}
	nativeURL := C.CString(rawURL)
	defer C.free(unsafe.Pointer(nativeURL))
	if C.wox_linux_window_open_external_url(native, nativeURL) != 0 {
		return errors.New("woxui: failed to open external URL on Linux")
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
	result := C.wox_linux_window_show_webview(native, url, html, css, cacheDisabled, cacheKey, C.float(bounds.X), C.float(bounds.Y), C.float(bounds.Width), C.float(bounds.Height))
	if result == -2 {
		return fmt.Errorf("%w: install WebKitGTK 4.1 or 4.0", ErrWebViewUnavailable)
	}
	if result != 0 {
		return errors.New("woxui: failed to show Linux WebView")
	}
	return nil
}

func (w *platformWindow) hideWebView() error {
	native, err := w.openNative()
	if err != nil {
		return err
	}
	if C.wox_linux_window_hide_webview(native) != 0 {
		return errors.New("woxui: failed to hide Linux WebView")
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
	if C.wox_linux_window_write_clipboard_text(native, nativeText) != 0 {
		return errors.New("woxui: failed to write Linux clipboard text")
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
	if C.wox_linux_window_write_clipboard_image(
		native,
		(*C.uint8_t)(unsafe.Pointer(&image.pixels[0])),
		C.int32_t(image.width),
		C.int32_t(image.height),
		C.int32_t(image.stride),
	) != 0 {
		return errors.New("woxui: failed to write Linux clipboard image")
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
	if C.wox_linux_window_invalidate(native) != 0 {
		return errors.New("woxui: failed to invalidate Linux window")
	}
	return nil
}

// setTextInputState updates GtkIMContext activation and candidate geometry on the GTK thread.
func (w *platformWindow) setTextInputState(state TextInputState) error {
	native, err := w.openNative()
	if err != nil {
		return err
	}
	enabled := C.int32_t(0)
	if state.Enabled {
		enabled = 1
	}
	if C.wox_linux_window_set_text_input_state(
		native,
		enabled,
		C.float(state.CursorRect.X),
		C.float(state.CursorRect.Y),
		C.float(state.CursorRect.Width),
		C.float(state.CursorRect.Height),
	) != 0 {
		return errors.New("woxui: failed to update Linux text input state")
	}
	return nil
}

// measureText uses Pango on the GTK thread so it matches the native renderer.
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
	result := C.wox_linux_window_measure_text(native, nativeText, nativeFontFamily, C.float(style.Size), C.uint8_t(style.Weight), &width, &height, &baseline)
	if result != 0 {
		return TextMetrics{}, errors.New("woxui: failed to measure Linux text")
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

	if native == nil || C.wox_linux_window_close(native) != 0 {
		w.mu.Lock()
		w.closing = false
		w.mu.Unlock()
		return errors.New("woxui: failed to close Linux window")
	}
	w.markClosed()
	return nil
}

func (w *platformWindow) openNative() (*C.WoxLinuxWindow, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed || w.closing || w.native == nil {
		return nil, errors.New("woxui: window is closed")
	}
	return w.native, nil
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
	w.mu.Unlock()
	if handle != 0 {
		handle.Delete()
	}
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
	result := C.wox_linux_window_begin_frame(
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
		w.recordRenderError("begin OpenGL frame", result)
		return
	}

	for _, command := range displayList.commands {
		switch command.kind {
		case displayCommandFillRoundedRect:
			result = C.wox_linux_window_fill_rounded_rect(
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
			result = C.wox_linux_window_stroke_rounded_rect(
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
			result = C.wox_linux_window_draw_text(
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
			result = C.wox_linux_window_draw_image(
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
			result = C.wox_linux_window_set_clip_rect(native, C.float(command.rect.X), C.float(command.rect.Y), C.float(command.rect.Width), C.float(command.rect.Height))
		case displayCommandClearClip:
			result = C.wox_linux_window_clear_clip(native)
		}
		if result != 0 {
			_ = C.wox_linux_window_end_frame(native)
			w.recordRenderError("encode OpenGL frame", result)
			return
		}
	}

	if result = C.wox_linux_window_end_frame(native); result != 0 {
		w.recordRenderError("finish OpenGL frame", result)
	}
}

//export woxGoLinuxStart
func woxGoLinuxStart(context C.uintptr_t) C.int32_t {
	state := cgo.Handle(context).Value().(*linuxRunState)
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

//export woxGoLinuxFrame
func woxGoLinuxFrame(context C.uintptr_t, width C.float, height C.float, pixelWidth C.int32_t, pixelHeight C.int32_t, scale C.float) {
	window := cgo.Handle(context).Value().(*platformWindow)
	window.drawFrame(FrameInfo{
		Size:      Size{Width: float32(width), Height: float32(height)},
		PixelSize: PixelSize{Width: int(pixelWidth), Height: int(pixelHeight)},
		Scale:     float32(scale),
	})
}

//export woxGoLinuxFocus
func woxGoLinuxFocus(context C.uintptr_t, epoch C.uint64_t, active C.int32_t) {
	window := cgo.Handle(context).Value().(*platformWindow)
	if window.options.OnFocus != nil {
		window.options.OnFocus(FocusEvent{Epoch: FocusEpoch(epoch), Active: active != 0})
	}
}

//export woxGoLinuxDestroyed
func woxGoLinuxDestroyed(context C.uintptr_t, epoch C.uint64_t, active C.int32_t) {
	window := cgo.Handle(context).Value().(*platformWindow)
	if active != 0 && window.options.OnFocus != nil {
		window.options.OnFocus(FocusEvent{Epoch: FocusEpoch(epoch), Active: false})
	}
	window.markClosed()
}

// woxGoLinuxKey forwards a normalized GDK key event into the window callback.
//
//export woxGoLinuxKey
func woxGoLinuxKey(context C.uintptr_t, key *C.char, modifiers C.uint8_t, down C.int32_t, repeat C.int32_t, composing C.int32_t) C.int32_t {
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

// woxGoLinuxTextInput forwards GtkIMContext commit and preedit changes.
//
//export woxGoLinuxTextInput
func woxGoLinuxTextInput(context C.uintptr_t, kind C.uint8_t, text *C.char) {
	window := cgo.Handle(context).Value().(*platformWindow)
	if window.options.OnTextInput != nil {
		window.options.OnTextInput(TextInputEvent{Kind: TextInputEventKind(kind), Text: C.GoString(text)})
	}
}

// woxGoLinuxPointer forwards GDK mouse and trackpad events in logical coordinates.
//
//export woxGoLinuxPointer
func woxGoLinuxPointer(context C.uintptr_t, kind C.uint8_t, x C.float, y C.float, button C.uint8_t, scrollX C.float, scrollY C.float, modifiers C.uint8_t) {
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
