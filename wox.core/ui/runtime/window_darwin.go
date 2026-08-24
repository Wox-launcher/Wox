//go:build darwin

package woxui

/*
#cgo CFLAGS: -fblocks -Wno-deprecated-declarations
#cgo LDFLAGS: -framework Cocoa -framework QuartzCore -framework CoreText -framework CoreGraphics -framework CoreVideo -framework IOSurface -framework WebKit
#include <stdlib.h>
#include "native_darwin.h"
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime"
	"runtime/cgo"
	"strings"
	"sync"
	"time"
	"unsafe"

	webviewruntime "wox/ui/runtime/internal/webview"
	"wox/util"
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
	mu              sync.Mutex
	renderMu        sync.Mutex
	native          *C.WoxDarwinWindow
	options         WindowOptions
	handle          cgo.Handle
	closing         bool
	closed          bool
	renderErr       error
	fontFamily      string
	renderWake      chan struct{}
	renderStop      chan struct{}
	renderDone      chan struct{}
	renderStopped   bool
	renderErrLogged bool
	pendingFrame    *darwinRenderFrame
	pendingDamage   Rect
	damagePending   bool
	fullDamage      bool
	webView         *webviewruntime.Controller
}

type darwinRenderFrame struct {
	frame                 FrameInfo
	displayList           *DisplayList
	fontFamily            string
	buildCost             time.Duration
	coalescedFrameCount   uint64
	firstCoalescedFrameID uint64
	lastCoalescedFrameID  uint64
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
	nonactivating := C.int32_t(0)
	if options.Nonactivating {
		nonactivating = 1
	}
	resizable := C.int32_t(0)
	if options.Resizable {
		resizable = 1
	}
	window.native = C.wox_darwin_window_create(
		title,
		C.float(options.Size.Width),
		C.float(options.Size.Height),
		hideOnBlur,
		C.int32_t(options.Role),
		nonactivating,
		resizable,
		C.float(options.AspectRatio),
		C.uintptr_t(window.handle),
	)
	if window.native == nil {
		window.handle.Delete()
		return nil, errors.New("woxui: failed to create AppKit window or native renderer")
	}
	if options.Topmost {
		_ = C.wox_darwin_window_set_topmost(window.native, 1)
	}
	window.webView = webviewruntime.New(&darwinWebViewDriver{window: window})
	window.startRenderWorker()
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

// runLockedOnMain runs op with renderMu held on the AppKit main thread.
// Deadlock fix: these operations dispatch to the main thread synchronously
// inside their native calls while holding renderMu. Acquiring the lock off the
// main thread could deadlock against a main-thread UI callback that also takes
// renderMu (for example a query response applying window bounds while a hide
// that owns the lock waits for the main queue). Hopping to the main thread
// before locking keeps one lock order: renderMu is only held by the main
// thread or by the render worker, which never blocks on the main queue.
func (w *platformWindow) runLockedOnMain(op func() error) error {
	var err error
	if callErr := platformCall(func() {
		w.renderMu.Lock()
		defer w.renderMu.Unlock()
		err = op()
	}); callErr != nil {
		return callErr
	}
	return err
}

func (w *platformWindow) hide() error {
	return w.runLockedOnMain(func() error {
		native, err := w.openNative()
		if err != nil {
			return err
		}
		if C.wox_darwin_window_hide(native) != 0 {
			return errors.New("woxui: failed to hide macOS window")
		}
		w.mu.Lock()
		dropped := w.pendingFrame
		w.pendingFrame = nil
		w.mu.Unlock()
		if dropped != nil {
			w.logRenderDiagnostic(fmt.Sprintf("event=frame_dropped reason=window_hidden frameId=%d", dropped.displayList.frameID))
			if w.options.frameMetrics != nil {
				w.options.frameMetrics.dropFrame(dropped.displayList.frameID)
			}
		}
		return nil
	})
}

func (w *platformWindow) setBounds(bounds Rect) error {
	return w.runLockedOnMain(func() error {
		native, err := w.openNative()
		if err != nil {
			return err
		}
		if C.wox_darwin_window_set_bounds(native, C.float(bounds.X), C.float(bounds.Y), C.float(bounds.Width), C.float(bounds.Height)) != 0 {
			return errors.New("woxui: failed to set macOS window bounds")
		}
		return nil
	})
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
	return w.runLockedOnMain(func() error {
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
	})
}

func (w *platformWindow) center(size Size) error {
	return w.runLockedOnMain(func() error {
		native, err := w.openNative()
		if err != nil {
			return err
		}
		if C.wox_darwin_window_center(native, C.float(size.Width), C.float(size.Height)) != 0 {
			return errors.New("woxui: failed to center macOS window")
		}
		return nil
	})
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

func (w *platformWindow) startFileDrag(paths []string) (FileDragStatus, error) {
	if len(paths) == 0 {
		return FileDragStatusCancel, errors.New("file drag has no paths")
	}
	native, err := w.openNative()
	if err != nil {
		return FileDragStatusCancel, err
	}
	payload := C.CString(strings.Join(paths, "\n"))
	defer C.free(unsafe.Pointer(payload))
	result := C.wox_darwin_window_start_file_drag(native, payload)
	if result < 0 {
		return FileDragStatusCancel, errors.New("native macOS file drag failed")
	}
	if result == 3 {
		return FileDragStatusPending, nil
	}
	return FileDragStatus(result), nil
}

func (w *platformWindow) minimize() error {
	native, err := w.openNative()
	if err != nil {
		return err
	}
	if C.wox_darwin_window_minimize(native) != 0 {
		return errors.New("woxui: failed to minimize macOS window")
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

func (w *platformWindow) focusReadyForBlur() bool {
	return true
}

func (w *platformWindow) setAppearance(isDark bool) error {
	native, err := w.openNative()
	if err != nil {
		return err
	}
	nativeDark := C.int32_t(0)
	if isDark {
		nativeDark = 1
	}
	if C.wox_darwin_window_set_appearance(native, nativeDark) != 0 {
		return errors.New("woxui: failed to update macOS window appearance")
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

func (w *platformWindow) saveFile(options SaveFileOptions) (string, error) {
	native, err := w.openNative()
	if err != nil {
		return "", err
	}
	title := C.CString(options.Title)
	defaultName := C.CString(options.DefaultFileName)
	extension := C.CString(options.Extension)
	defer C.free(unsafe.Pointer(title))
	defer C.free(unsafe.Pointer(defaultName))
	defer C.free(unsafe.Pointer(extension))
	var path *C.char
	result := C.wox_darwin_window_save_file(native, title, defaultName, extension, &path)
	if result == 1 {
		return "", nil
	}
	if result != 0 || path == nil {
		return "", errors.New("woxui: failed to open macOS save dialog")
	}
	defer C.free(unsafe.Pointer(path))
	return C.GoString(path), nil
}

func (w *platformWindow) setPointerPassthrough(enabled bool) error {
	native, err := w.openNative()
	if err != nil {
		return err
	}
	nativeEnabled := C.int32_t(0)
	if enabled {
		nativeEnabled = 1
	}
	if C.wox_darwin_window_set_pointer_passthrough(native, nativeEnabled) != 0 {
		return errors.New("woxui: failed to update macOS pointer passthrough")
	}
	return nil
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
		shouldLog := !w.renderErrLogged
		w.renderErrLogged = true
		w.mu.Unlock()
		if shouldLog {
			w.logRenderDiagnostic(fmt.Sprintf("event=invalidation_rejected reason=sticky_render_error error=%q", err.Error()))
		}
		return err
	}
	w.fullDamage = true
	w.pendingDamage = Rect{}
	w.damagePending = true
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

func (w *platformWindow) invalidateRect(rect Rect) error {
	w.mu.Lock()
	if w.renderErr != nil {
		err := w.renderErr
		shouldLog := !w.renderErrLogged
		w.renderErrLogged = true
		w.mu.Unlock()
		if shouldLog {
			w.logRenderDiagnostic(fmt.Sprintf("event=invalidation_rejected reason=sticky_render_error error=%q", err.Error()))
		}
		return err
	}
	if !w.fullDamage {
		w.pendingDamage = unionRects(w.pendingDamage, rect)
	}
	w.damagePending = true
	w.mu.Unlock()
	native, err := w.openNative()
	if err != nil {
		return err
	}
	if C.wox_darwin_window_invalidate(native) != 0 {
		return errors.New("woxui: failed to invalidate macOS window rectangle")
	}
	return nil
}

func (*platformWindow) displayListDamageCullingEnabled() bool { return false }

func (w *platformWindow) requestAnimationFrame() error {
	native, err := w.openNative()
	if err != nil {
		return err
	}
	if C.wox_darwin_window_request_animation_frame(native) != 0 {
		return errors.New("woxui: failed to request a macOS animation frame")
	}
	return nil
}

func (w *platformWindow) stopAnimationFrames() error {
	native, err := w.openNative()
	if err != nil {
		return err
	}
	if C.wox_darwin_window_stop_animation_frames(native) != 0 {
		return errors.New("woxui: failed to stop macOS animation frames")
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

func (w *platformWindow) setPointerCursor(cursor PointerCursor) error {
	native, err := w.openNative()
	if err != nil {
		return err
	}
	if C.wox_darwin_window_set_pointer_cursor(native, C.uint8_t(cursor)) != 0 {
		return errors.New("woxui: failed to update macOS pointer cursor")
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
	if style.Family == FontFamilyMonospace {
		fontFamily = "Menlo"
	}
	nativeFontFamily := C.CString(fontFamily)
	defer C.free(unsafe.Pointer(nativeFontFamily))
	var width C.float
	var height C.float
	var baseline C.float
	result := C.wox_darwin_window_measure_text(native, nativeText, nativeFontFamily, C.float(style.Size), C.uint8_t(style.Weight), C.uint8_t(boolByte(style.Italic)), &width, &height, &baseline)
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
	w.stopRenderWorkerLocked()
	renderDone := w.renderDone
	w.mu.Unlock()
	if renderDone != nil {
		<-renderDone
	}

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
	webView := w.webView
	w.webView = nil
	w.closing = false
	w.closed = true
	handle := w.handle
	w.handle = 0
	onClosed := w.options.OnClosed
	w.mu.Unlock()
	if webView != nil {
		webView.Close()
	}
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
	// Positive frame statuses are recoverable skips or compositor backpressure. Latching
	// them would reject every later invalidation even after the IOSurface pool recovers.
	if result >= 0 {
		return
	}
	var firstError error
	w.mu.Lock()
	if w.renderErr == nil {
		w.renderErr = fmt.Errorf("woxui: %s failed with status %d", operation, int32(result))
		firstError = w.renderErr
	}
	w.mu.Unlock()
	if firstError != nil {
		w.logRenderDiagnostic(fmt.Sprintf("event=renderer_error_latched operation=%q status=%d futureInvalidationsRejected=true error=%q", operation, int32(result), firstError.Error()))
	}
}

// logRenderDiagnostic routes macOS renderer diagnostics through Wox's configured debug logger.
func (w *platformWindow) logRenderDiagnostic(message string) {
	util.GetLogger().Debug(context.Background(), fmt.Sprintf("darwin_render window=%p title=%q %s", w, w.options.Title, message))
}

// darwinFrameStatusName keeps native skip and backpressure statuses readable in diagnostic logs.
func darwinFrameStatusName(result int32) string {
	switch result {
	case int32(C.WOX_DARWIN_FRAME_SKIPPED):
		return "native_skipped"
	case int32(C.WOX_DARWIN_FRAME_SURFACE_BUSY):
		return "surface_busy"
	default:
		return "native_error"
	}
}

// startRenderWorker owns ordinary frame encoding so AppKit callbacks can return after building the display list.
func (w *platformWindow) startRenderWorker() {
	w.mu.Lock()
	w.renderWake = make(chan struct{}, 1)
	w.renderStop = make(chan struct{})
	w.renderDone = make(chan struct{})
	w.mu.Unlock()
	go w.renderLoop()
}

// stopRenderWorkerLocked discards queued ordinary frames before native renderer destruction.
func (w *platformWindow) stopRenderWorkerLocked() {
	if w.renderStopped || w.renderStop == nil {
		return
	}
	w.renderStopped = true
	dropped := w.pendingFrame
	w.pendingFrame = nil
	if dropped != nil {
		w.logRenderDiagnostic(fmt.Sprintf("event=frame_dropped reason=renderer_stopped frameId=%d", dropped.displayList.frameID))
		if w.options.frameMetrics != nil {
			w.options.frameMetrics.dropFrame(dropped.displayList.frameID)
		}
	}
	close(w.renderStop)
}

// renderLoop serializes renderer access and consumes only the newest frame queued while encoding.
func (w *platformWindow) renderLoop() {
	// Objective-C autorelease pools are thread-affine across the frame's cgo calls.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(w.renderDone)
	trimTimer := time.NewTimer(time.Hour)
	if !trimTimer.Stop() {
		<-trimTimer.C
	}
	defer trimTimer.Stop()
	var trimTimerC <-chan time.Time
	trimRetryDelay := 100 * time.Millisecond
	trimRetryCount := 0
	resetTrimTimer := func(delay time.Duration) {
		if !trimTimer.Stop() {
			select {
			case <-trimTimer.C:
			default:
			}
		}
		trimTimer.Reset(delay)
		trimTimerC = trimTimer.C
	}
	for {
		select {
		case <-w.renderStop:
			return
		case <-w.renderWake:
			for {
				w.mu.Lock()
				frame := w.pendingFrame
				w.pendingFrame = nil
				stopped := w.renderStopped
				w.mu.Unlock()
				if stopped {
					return
				}
				if frame == nil {
					break
				}
				w.logCoalescedFrames(frame)
				w.encodeFrame(frame, false)
			}
			// Tracked result refreshes can produce a frame every second, so trim before that cadence.
			trimRetryDelay = 100 * time.Millisecond
			trimRetryCount = 0
			resetTrimTimer(250 * time.Millisecond)
		case <-trimTimerC:
			trimTimerC = nil
			if w.trimRenderSurfaces() > 2 && trimRetryCount < 4 {
				resetTrimTimer(trimRetryDelay)
				trimRetryDelay = min(trimRetryDelay*2, time.Second)
				trimRetryCount++
			}
		}
	}
}

// queueFrame replaces an obsolete unencoded frame instead of letting rendering fall behind input.
func (w *platformWindow) queueFrame(frame *darwinRenderFrame) {
	w.mu.Lock()
	if w.closed || w.closing || w.renderStopped {
		closed := w.closed
		closing := w.closing
		stopped := w.renderStopped
		w.mu.Unlock()
		w.logRenderDiagnostic(fmt.Sprintf("event=frame_dropped reason=renderer_unavailable frameId=%d closed=%t closing=%t stopped=%t", frame.displayList.frameID, closed, closing, stopped))
		if w.options.frameMetrics != nil {
			w.options.frameMetrics.dropFrame(frame.displayList.frameID)
		}
		return
	}
	replaced := w.pendingFrame
	if replaced != nil {
		frame.displayList.SetNativeDamage(mergeFrameDamage(frame.displayList.NativeDamage(), replaced.displayList.NativeDamage()))
		frame.coalescedFrameCount = replaced.coalescedFrameCount + 1
		frame.firstCoalescedFrameID = replaced.firstCoalescedFrameID
		if frame.firstCoalescedFrameID == 0 {
			frame.firstCoalescedFrameID = replaced.displayList.frameID
		}
		frame.lastCoalescedFrameID = replaced.displayList.frameID
	}
	w.pendingFrame = frame
	wake := w.renderWake
	w.mu.Unlock()
	if replaced != nil && w.options.frameMetrics != nil {
		w.options.frameMetrics.coalesceFrame(replaced.displayList.frameID)
	}
	signalRenderWake(wake)
}

// logCoalescedFrames records one summary for a burst instead of logging every replaced frame.
func (w *platformWindow) logCoalescedFrames(frame *darwinRenderFrame) {
	if frame == nil || frame.coalescedFrameCount == 0 {
		return
	}
	damage := frame.displayList.NativeDamage()
	w.logRenderDiagnostic(fmt.Sprintf(
		"event=frames_coalesced frameId=%d replacedCount=%d firstReplacedFrameId=%d lastReplacedFrameId=%d damage=%.1f,%.1f %.1fx%.1f",
		frame.displayList.frameID,
		frame.coalescedFrameCount,
		frame.firstCoalescedFrameID,
		frame.lastCoalescedFrameID,
		damage.X,
		damage.Y,
		damage.Width,
		damage.Height,
	))
}

// signalRenderWake coalesces renderer activity notifications.
func signalRenderWake(wake chan struct{}) {
	select {
	case wake <- struct{}{}:
	default:
	}
}

func (w *platformWindow) drawFrame(frame FrameInfo) {
	frame.Damage = w.consumePendingDamage()
	frameStart := time.Now()
	displayList := &DisplayList{}
	if w.options.frameMetrics != nil {
		displayList.frameID = w.options.frameMetrics.beginFrame()
	}
	if w.options.OnFrame != nil {
		w.options.OnFrame(displayList, frame)
	}
	buildCost := time.Since(frameStart)
	w.mu.Lock()
	fontFamily := w.fontFamily
	w.mu.Unlock()
	w.queueFrame(&darwinRenderFrame{frame: frame, displayList: displayList, fontFamily: fontFamily, buildCost: buildCost})
}

// drawFrameSync keeps resize and capture frames ordered with their native window operation.
func (w *platformWindow) drawFrameSync(frame FrameInfo, transactional bool) {
	frame.Damage = w.consumePendingDamage()
	frameStart := time.Now()
	displayList := &DisplayList{}
	if w.options.frameMetrics != nil {
		displayList.frameID = w.options.frameMetrics.beginFrame()
	}
	if w.options.OnFrame != nil {
		w.options.OnFrame(displayList, frame)
	}
	buildCost := time.Since(frameStart)
	w.mu.Lock()
	fontFamily := w.fontFamily
	// The synchronous frame is newer than every ordinary frame still waiting to encode.
	replaced := w.pendingFrame
	w.pendingFrame = nil
	wake := w.renderWake
	w.mu.Unlock()
	if replaced != nil {
		displayList.SetNativeDamage(mergeFrameDamage(displayList.NativeDamage(), replaced.displayList.NativeDamage()))
		w.logRenderDiagnostic(fmt.Sprintf("event=frame_dropped reason=synchronous_frame_replaced frameId=%d replacementFrameId=%d", replaced.displayList.frameID, displayList.frameID))
	}
	if replaced != nil && w.options.frameMetrics != nil {
		w.options.frameMetrics.coalesceFrame(replaced.displayList.frameID)
	}
	w.encodeFrameLocked(&darwinRenderFrame{frame: frame, displayList: displayList, fontFamily: fontFamily, buildCost: buildCost}, transactional)
	signalRenderWake(wake)
}

func (w *platformWindow) consumePendingDamage() Rect {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.damagePending || w.fullDamage {
		w.pendingDamage = Rect{}
		w.damagePending = false
		w.fullDamage = false
		return Rect{}
	}
	damage := w.pendingDamage
	w.pendingDamage = Rect{}
	w.damagePending = false
	return damage
}

// restoreFrameDamage keeps a skipped partial frame's cleanup region in the next frame.
func (w *platformWindow) restoreFrameDamage(damage Rect) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if damage.Width <= 0 || damage.Height <= 0 {
		w.fullDamage = true
		w.pendingDamage = Rect{}
	} else if !w.fullDamage {
		w.pendingDamage = unionRects(w.pendingDamage, damage)
	}
	w.damagePending = true
}

func mergeFrameDamage(current, replaced Rect) Rect {
	if current.Width <= 0 || current.Height <= 0 || replaced.Width <= 0 || replaced.Height <= 0 {
		return Rect{}
	}
	return unionRects(current, replaced)
}

// encodeFrame records and submits one display list while holding exclusive renderer ownership.
func (w *platformWindow) encodeFrame(renderFrame *darwinRenderFrame, transactional bool) {
	w.renderMu.Lock()
	defer w.renderMu.Unlock()
	w.encodeFrameLocked(renderFrame, transactional)
}

// encodeFrameLocked records a frame for callers that already own the renderer transaction.
func (w *platformWindow) encodeFrameLocked(renderFrame *darwinRenderFrame, transactional bool) {
	nativeStart := time.Now()
	native, err := w.openNative()
	if err != nil {
		w.logRenderDiagnostic(fmt.Sprintf("event=frame_dropped reason=native_window_unavailable frameId=%d error=%q", renderFrame.displayList.frameID, err.Error()))
		if w.options.frameMetrics != nil {
			w.options.frameMetrics.dropFrame(renderFrame.displayList.frameID)
		}
		return
	}
	pool := C.wox_darwin_autorelease_pool_push()
	defer C.wox_darwin_autorelease_pool_pop(pool)
	frameStart := time.Now()
	frame := renderFrame.frame
	displayList := renderFrame.displayList
	nativeDamage := displayList.NativeDamage()
	transactionalFrame := C.int32_t(0)
	if transactional {
		transactionalFrame = 1
	}
	nativeFontFamily := C.CString(renderFrame.fontFamily)
	defer C.free(unsafe.Pointer(nativeFontFamily))
	nativeMonospaceFontFamily := C.CString("Menlo")
	defer C.free(unsafe.Pointer(nativeMonospaceFontFamily))
	beginStart := time.Now()
	result := C.wox_darwin_window_begin_frame(
		native,
		C.uint64_t(displayList.frameID),
		C.float(frame.Size.Width),
		C.float(frame.Size.Height),
		C.float(frame.Scale),
		C.float(nativeDamage.X),
		C.float(nativeDamage.Y),
		C.float(nativeDamage.Width),
		C.float(nativeDamage.Height),
		C.uint8_t(displayList.clearColor.R),
		C.uint8_t(displayList.clearColor.G),
		C.uint8_t(displayList.clearColor.B),
		C.uint8_t(displayList.clearColor.A),
	)
	beginCost := time.Since(beginStart)
	if result > 0 {
		// Preserve cleanup damage for the next naturally scheduled frame. Scheduling
		// an immediate retry here can starve AppKit while every surface is still busy.
		reason := darwinFrameStatusName(int32(result))
		if result == C.WOX_DARWIN_FRAME_SURFACE_BUSY {
			w.restoreFrameDamage(nativeDamage)
		}
		w.logRenderDiagnostic(fmt.Sprintf(
			"event=frame_dropped reason=%s frameId=%d status=%d damage=%.1f,%.1f %.1fx%.1f size=%.1fx%.1f scale=%.2f",
			reason,
			displayList.frameID,
			int32(result),
			nativeDamage.X,
			nativeDamage.Y,
			nativeDamage.Width,
			nativeDamage.Height,
			frame.Size.Width,
			frame.Size.Height,
			frame.Scale,
		))
		if w.options.frameMetrics != nil {
			if result == C.WOX_DARWIN_FRAME_SURFACE_BUSY {
				w.options.frameMetrics.backpressureFrame(displayList.frameID)
			} else {
				w.options.frameMetrics.dropFrame(displayList.frameID)
			}
		}
		return
	}
	if result < 0 {
		if w.options.frameMetrics != nil {
			w.recordNativeResourceMetrics(native, displayList)
			w.options.frameMetrics.finishNativeFrame(displayList.frameID, time.Since(nativeStart), -1, false)
		}
		w.recordRenderError("begin macOS frame", result)
		return
	}

	encodeStart := time.Now()
	var textCost time.Duration
	var imageCost time.Duration
	var textCount int
	var imageCount int
	commandIndex := -1
	encodeFailed := false
	var failedCommandKind displayCommandKind
	displayList.forEachCommand(func(command displayCommand) bool {
		commandIndex++
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
		case displayCommandFillConvexPolygon:
			result = C.wox_darwin_window_fill_convex_polygon(
				native,
				(*C.float)(unsafe.Pointer(&command.points[0])),
				C.int32_t(len(command.points)),
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
			commandStart := time.Now()
			text := C.CString(command.text)
			drawFontFamily := nativeFontFamily
			if command.style.Family == FontFamilyMonospace {
				drawFontFamily = nativeMonospaceFontFamily
			}
			result = C.wox_darwin_window_draw_text(
				native,
				text,
				drawFontFamily,
				C.float(command.rect.X),
				C.float(command.rect.Y),
				C.float(command.rect.Width),
				C.float(command.rect.Height),
				C.float(command.style.Size),
				C.uint8_t(command.style.Weight),
				C.uint8_t(boolByte(command.style.Italic)),
				C.uint8_t(command.color.R),
				C.uint8_t(command.color.G),
				C.uint8_t(command.color.B),
				C.uint8_t(command.color.A),
			)
			C.free(unsafe.Pointer(text))
			textCost += time.Since(commandStart)
			textCount++
		case displayCommandDrawImage:
			commandStart := time.Now()
			result = C.wox_darwin_window_draw_image(
				native,
				C.uint64_t(command.image.id),
				(*C.uint8_t)(unsafe.Pointer(&command.image.pixels[0])),
				C.int32_t(command.image.Width),
				C.int32_t(command.image.Height),
				C.int32_t(command.image.Width*4),
				C.float(command.rect.X),
				C.float(command.rect.Y),
				C.float(command.rect.Width),
				C.float(command.rect.Height),
				C.float(command.rotation),
				C.float(command.radius),
			)
			imageCost += time.Since(commandStart)
			imageCount++
		case displayCommandBeginEmbeddedSurfaceOverlay:
			result = C.wox_darwin_window_begin_embedded_surface_overlay(native)
		case displayCommandSetClipRect:
			result = C.wox_darwin_window_set_clip_rect(native, C.float(command.rect.X), C.float(command.rect.Y), C.float(command.rect.Width), C.float(command.rect.Height))
		case displayCommandClearClip:
			result = C.wox_darwin_window_clear_clip(native)
		}
		if result != 0 {
			encodeFailed = true
			failedCommandKind = command.kind
			return false
		}
		return true
	})
	encodeCost := time.Since(encodeStart)

	endStart := time.Now()
	endResult := C.wox_darwin_window_end_frame(native, transactionalFrame)
	endCost := time.Since(endStart)
	if encodeFailed {
		reason := darwinFrameStatusName(int32(result))
		if w.options.frameMetrics != nil {
			w.recordNativeResourceMetrics(native, displayList)
			if result == C.WOX_DARWIN_FRAME_SURFACE_BUSY {
				w.options.frameMetrics.finishBackpressuredFrame(displayList.frameID, time.Since(nativeStart), -1)
			} else {
				w.options.frameMetrics.finishNativeFrame(displayList.frameID, time.Since(nativeStart), -1, false)
			}
		}
		if result > 0 {
			if result == C.WOX_DARWIN_FRAME_SURFACE_BUSY {
				w.restoreFrameDamage(nativeDamage)
			}
			w.logRenderDiagnostic(fmt.Sprintf("event=frame_dropped reason=%s stage=encode frameId=%d commandIndex=%d commandKind=%d status=%d", reason, displayList.frameID, commandIndex, failedCommandKind, int32(result)))
			return
		}
		w.logRenderDiagnostic(fmt.Sprintf("event=frame_encode_failed reason=%s frameId=%d commandIndex=%d commandKind=%d status=%d", reason, displayList.frameID, commandIndex, failedCommandKind, int32(result)))
		w.recordRenderError("encode macOS frame", result)
		return
	}
	result = endResult
	if result > 0 {
		if result == C.WOX_DARWIN_FRAME_SURFACE_BUSY {
			w.restoreFrameDamage(nativeDamage)
		}
		w.logRenderDiagnostic(fmt.Sprintf("event=frame_dropped reason=%s stage=end frameId=%d status=%d", darwinFrameStatusName(int32(result)), displayList.frameID, int32(result)))
	} else if result < 0 {
		w.logRenderDiagnostic(fmt.Sprintf("event=frame_end_failed reason=%s frameId=%d status=%d", darwinFrameStatusName(int32(result)), displayList.frameID, int32(result)))
		w.recordRenderError("present macOS frame", result)
	}
	if w.options.frameMetrics != nil {
		w.recordNativeResourceMetrics(native, displayList)
		if result == C.WOX_DARWIN_FRAME_SURFACE_BUSY {
			w.options.frameMetrics.finishBackpressuredFrame(displayList.frameID, time.Since(nativeStart)-endCost, endCost)
		} else {
			w.options.frameMetrics.finishNativeFrame(displayList.frameID, time.Since(nativeStart)-endCost, endCost, result == 0)
		}
	}
	totalCost := renderFrame.buildCost + time.Since(frameStart)
	if totalCost >= 16*time.Millisecond {
		log.Printf("darwin frame timing: totalUs=%d buildUs=%d beginUs=%d encodeUs=%d textUs=%d imageUs=%d endUs=%d commands=%d text=%d image=%d size=%.0fx%.0f scale=%.2f transactional=%t",
			totalCost.Microseconds(), renderFrame.buildCost.Microseconds(), beginCost.Microseconds(), encodeCost.Microseconds(), textCost.Microseconds(), imageCost.Microseconds(), endCost.Microseconds(), len(displayList.commands), textCount, imageCount, frame.Size.Width, frame.Size.Height, frame.Scale, transactional)
	}
}

// rendererResourcesFromNative copies one native encode-stat snapshot into portable metrics.
func rendererResourcesFromNative(stats C.WoxRendererResourceStats) FrameRendererResourceMetrics {
	return FrameRendererResourceMetrics{
		TextRasterizations: int(stats.text_rasterizations),
		ImageCreates:       int(stats.image_creates),
		ImageUploads:       int(stats.image_uploads),
		CacheHits:          int(stats.cache_hits),
		CacheEvictions:     int(stats.cache_evictions),
		ResidentBytes:      int64(stats.resident_bytes),
	}
}

// recordNativeResourceMetrics stores actual native cache hits instead of the uncached baseline.
func (w *platformWindow) recordNativeResourceMetrics(native *C.WoxDarwinWindow, displayList *DisplayList) {
	if w.options.frameMetrics == nil || displayList == nil {
		return
	}
	var stats C.WoxRendererResourceStats
	if native != nil && C.wox_darwin_window_take_frame_resource_stats(native, &stats) == 0 {
		w.options.frameMetrics.recordRendererResources(displayList.frameID, rendererResourcesFromNative(stats))
		return
	}
	w.options.frameMetrics.recordEncodedResources(displayList)
}

// testCachedCGImageOwnsNativePixels wraps the native lifetime check for Go tests.
func testCachedCGImageOwnsNativePixels() int32 {
	return int32(C.wox_darwin_test_cached_image_owns_pixels())
}

// testLargeImageAdmission wraps the native large-image admission policy check for Go tests.
func testLargeImageAdmission() int32 {
	return int32(C.wox_darwin_test_large_image_admission())
}

// trimRenderSurfaces releases idle back buffers without changing active triple-buffer behavior.
func (w *platformWindow) trimRenderSurfaces() int {
	w.renderMu.Lock()
	defer w.renderMu.Unlock()
	native, err := w.openNative()
	if err != nil {
		return 0
	}
	pool := C.wox_darwin_autorelease_pool_push()
	defer C.wox_darwin_autorelease_pool_pop(pool)
	return int(C.wox_darwin_window_trim_render_surfaces(native, 2))
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

//export woxGoDarwinProtocolURL
func woxGoDarwinProtocolURL(_ C.uintptr_t, rawURL *C.char) {
	if rawURL == nil {
		return
	}
	dispatchProtocolURL(C.GoString(rawURL))
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

//export woxGoDarwinFrameSync
func woxGoDarwinFrameSync(context C.uintptr_t, width C.float, height C.float, pixelWidth C.int32_t, pixelHeight C.int32_t, scale C.float, transactional C.int32_t) {
	window := cgo.Handle(context).Value().(*platformWindow)
	window.drawFrameSync(FrameInfo{
		Size:      Size{Width: float32(width), Height: float32(height)},
		PixelSize: PixelSize{Width: int(pixelWidth), Height: int(pixelHeight)},
		Scale:     float32(scale),
	}, transactional != 0)
}

const (
	darwinRenderDiagnosticWindowUnavailable  = 1
	darwinRenderDiagnosticRendererReplaced   = 2
	darwinRenderDiagnosticGenerationMismatch = 3
	darwinRenderDiagnosticStaleSequence      = 4
	darwinRenderDiagnosticRecovered          = 5
)

// darwinRenderDiagnosticName keeps asynchronous native presentation outcomes searchable.
func darwinRenderDiagnosticName(event uint8) string {
	switch event {
	case darwinRenderDiagnosticWindowUnavailable:
		return "presentation_window_unavailable"
	case darwinRenderDiagnosticRendererReplaced:
		return "presentation_renderer_replaced"
	case darwinRenderDiagnosticGenerationMismatch:
		return "presentation_generation_mismatch"
	case darwinRenderDiagnosticStaleSequence:
		return "presentation_stale_sequence"
	case darwinRenderDiagnosticRecovered:
		return "presentation_recovered"
	default:
		return "presentation_unknown"
	}
}

// woxGoDarwinPresentationDiagnostic records asynchronous Core Animation presentation rejections and recovery.
//
//export woxGoDarwinPresentationDiagnostic
func woxGoDarwinPresentationDiagnostic(context C.uintptr_t, frameID C.uint64_t, event C.uint8_t, rendererKind C.uint8_t, sequence C.uint64_t, generation C.uint64_t, currentGeneration C.uint64_t) {
	window := cgo.Handle(context).Value().(*platformWindow)
	renderer := "background"
	if rendererKind != 0 {
		renderer = "overlay"
	}
	window.logRenderDiagnostic(fmt.Sprintf(
		"event=%s frameId=%d renderer=%s sequence=%d generation=%d currentGeneration=%d",
		darwinRenderDiagnosticName(uint8(event)),
		uint64(frameID),
		renderer,
		uint64(sequence),
		uint64(generation),
		uint64(currentGeneration),
	))
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

//export woxGoDarwinFileDrop
func woxGoDarwinFileDrop(context C.uintptr_t, paths *C.char) {
	window := cgo.Handle(context).Value().(*platformWindow)
	if window.options.OnFileDrop == nil || paths == nil {
		return
	}
	values := splitFileDropPayload(C.GoString(paths))
	if len(values) > 0 {
		window.options.OnFileDrop(values)
	}
}

//export woxGoDarwinFileDragEnded
func woxGoDarwinFileDragEnded(context C.uintptr_t, status C.int32_t) {
	window := cgo.Handle(context).Value().(*platformWindow)
	if window.options.OnFileDragEnded != nil {
		window.options.OnFileDragEnded(FileDragStatus(status))
	}
}
