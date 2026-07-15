//go:build windows

package woxui

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/win"
)

const (
	windowClassName         = "WoxGoUIWindow"
	windowCommandMessage    = win.WM_APP + 1
	windowBlurGuardDuration = 300 * time.Millisecond
	wsExNoRedirectionBitmap = 0x00200000
	errorClassAlreadyExists = syscall.Errno(1410)
)

var (
	registerWindowClassOnce              sync.Once
	registerWindowClassErr               error
	windowProcedureCallback              = syscall.NewCallback(windowProcedure)
	nativeWindows                        sync.Map
	setProcessDPIAwarenessContext        = syscall.NewLazyDLL("user32.dll").NewProc("SetProcessDpiAwarenessContext")
	setThreadDPIAwarenessContext         = syscall.NewLazyDLL("user32.dll").NewProc("SetThreadDpiAwarenessContext")
	setProcessDPIAware                   = syscall.NewLazyDLL("user32.dll").NewProc("SetProcessDPIAware")
	dpiAwarenessContextPerMonitorAwareV2 = ^uintptr(3)
	platformRuntime                      struct {
		sync.Mutex
		running           bool
		messageLoopActive bool
		uiThreadID        uint32
		windowCount       int
		runErr            error
	}
)

type windowCommandKind uint8

const (
	windowCommandShow windowCommandKind = iota
	windowCommandHide
	windowCommandClose
)

type windowCommand struct {
	kind  windowCommandKind
	reply chan windowCommandResult
}

type windowCommandResult struct {
	epoch FocusEpoch
	err   error
}

type focusRuntime struct {
	epoch                 FocusEpoch
	visible               bool
	active                bool
	activationConfirmed   bool
	blurGuardUntil        time.Time
	previousForeground    win.HWND
	restorePreviousOnHide bool
}

// platformWindow owns one Win32 window and its DirectComposition surface.
type platformWindow struct {
	options WindowOptions

	mu         sync.Mutex
	hwnd       win.HWND
	uiThreadID uint32
	pending    []windowCommand
	done       chan struct{}

	renderer *nativeRenderer
	focus    focusRuntime
	scale    float32
}

// platformRun owns the Win32 message pump on the caller's OS main thread.
func platformRun(start func() error) (runErr error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	enablePerMonitorDPIAwareness()

	platformRuntime.Lock()
	if platformRuntime.running {
		platformRuntime.Unlock()
		return errors.New("window runtime is already running")
	}
	platformRuntime.running = true
	platformRuntime.messageLoopActive = false
	platformRuntime.uiThreadID = win.GetCurrentThreadId()
	platformRuntime.windowCount = 0
	platformRuntime.runErr = nil
	platformRuntime.Unlock()
	defer func() {
		platformRuntime.Lock()
		if runErr == nil {
			runErr = platformRuntime.runErr
		}
		platformRuntime.running = false
		platformRuntime.messageLoopActive = false
		platformRuntime.uiThreadID = 0
		platformRuntime.windowCount = 0
		platformRuntime.runErr = nil
		platformRuntime.Unlock()
	}()

	comResult := win.CoInitializeEx(nil, win.COINIT_APARTMENTTHREADED)
	if !win.SUCCEEDED(comResult) {
		return fmt.Errorf("initialize COM failed with HRESULT 0x%08X", uint32(comResult))
	}
	defer win.CoUninitialize()

	if err := ensureWindowClass(); err != nil {
		return err
	}
	if start == nil {
		return errors.New("window runtime start callback is nil")
	}
	if err := start(); err != nil {
		nativeWindows.Range(func(_, value any) bool {
			win.DestroyWindow(value.(*platformWindow).hwnd)
			return true
		})
		return err
	}

	platformRuntime.Lock()
	if platformRuntime.windowCount == 0 {
		platformRuntime.Unlock()
		return nil
	}
	platformRuntime.messageLoopActive = true
	platformRuntime.Unlock()
	var message win.MSG
	for {
		result := win.GetMessage(&message, 0, 0, 0)
		if result == 0 {
			return nil
		}
		if result == -1 {
			return errors.New("GetMessage failed")
		}
		win.TranslateMessage(&message)
		win.DispatchMessage(&message)
	}
}

// openPlatformWindow creates a hidden window on the runtime thread.
func openPlatformWindow(options WindowOptions) (*platformWindow, error) {
	platformRuntime.Lock()
	running := platformRuntime.running
	uiThreadID := platformRuntime.uiThreadID
	platformRuntime.Unlock()
	if !running || uiThreadID != win.GetCurrentThreadId() {
		return nil, errors.New("windows must be opened from the Run callback")
	}

	window := &platformWindow{
		options:    options,
		uiThreadID: uiThreadID,
		done:       make(chan struct{}),
	}
	if err := window.createNativeWindow(); err != nil {
		close(window.done)
		return nil, err
	}
	return window, nil
}

func (w *platformWindow) show() (FocusEpoch, error) {
	result := w.call(windowCommandShow)
	return result.epoch, result.err
}

func (w *platformWindow) hide() error {
	return w.call(windowCommandHide).err
}

func (w *platformWindow) invalidate() error {
	w.mu.Lock()
	hwnd := w.hwnd
	w.mu.Unlock()
	if hwnd == 0 {
		return errors.New("window is closed")
	}
	if !win.InvalidateRect(hwnd, nil, false) {
		return errors.New("failed to invalidate window")
	}
	return nil
}

func (w *platformWindow) close() error {
	return w.call(windowCommandClose).err
}

// createNativeWindow publishes the HWND only after CreateWindowEx has completed its synchronous messages.
func (w *platformWindow) createNativeWindow() error {
	instance := win.GetModuleHandle(nil)
	className, err := syscall.UTF16PtrFromString(windowClassName)
	if err != nil {
		return err
	}
	title, err := syscall.UTF16PtrFromString(w.options.Title)
	if err != nil {
		return err
	}

	scale := primaryDisplayScale()
	width := logicalToPhysical(w.options.Size.Width, scale)
	height := logicalToPhysical(w.options.Size.Height, scale)
	x := (win.GetSystemMetrics(win.SM_CXSCREEN) - int32(width)) / 2
	y := (win.GetSystemMetrics(win.SM_CYSCREEN) - int32(height)) / 3
	hwnd := win.CreateWindowEx(
		win.WS_EX_TOPMOST|win.WS_EX_TOOLWINDOW|wsExNoRedirectionBitmap,
		className,
		title,
		win.WS_POPUP,
		x,
		y,
		int32(width),
		int32(height),
		0,
		0,
		instance,
		nil,
	)
	if hwnd == 0 {
		return fmt.Errorf("create native window failed: %w", syscall.GetLastError())
	}
	w.mu.Lock()
	w.hwnd = hwnd
	w.mu.Unlock()
	dpi := win.GetDpiForWindow(hwnd)
	if dpi != 0 {
		scale = float32(dpi) / 96
	}
	w.scale = scale
	var client win.RECT
	if !win.GetClientRect(hwnd, &client) {
		win.DestroyWindow(hwnd)
		w.mu.Lock()
		w.hwnd = 0
		w.mu.Unlock()
		return errors.New("get initial client size failed")
	}
	renderer, err := newNativeRenderer(uintptr(hwnd), int(client.Right-client.Left), int(client.Bottom-client.Top))
	if err != nil {
		win.DestroyWindow(hwnd)
		w.mu.Lock()
		w.hwnd = 0
		w.mu.Unlock()
		return err
	}
	w.renderer = renderer
	nativeWindows.Store(uintptr(hwnd), w)
	platformRuntime.Lock()
	platformRuntime.windowCount++
	platformRuntime.Unlock()
	win.InvalidateRect(hwnd, nil, false)
	return nil
}

// enablePerMonitorDPIAwareness keeps native sizes in physical pixels while the public API stays logical.
func enablePerMonitorDPIAwareness() {
	processAware := false
	if setProcessDPIAwarenessContext.Find() == nil {
		result, _, _ := setProcessDPIAwarenessContext.Call(dpiAwarenessContextPerMonitorAwareV2)
		processAware = result != 0
	}
	if !processAware && setProcessDPIAware.Find() == nil {
		_, _, _ = setProcessDPIAware.Call()
	}
	if setThreadDPIAwarenessContext.Find() == nil {
		_, _, _ = setThreadDPIAwarenessContext.Call(dpiAwarenessContextPerMonitorAwareV2)
	}
}

// primaryDisplayScale returns the initial scale before the window has an HWND.
func primaryDisplayScale() float32 {
	dc := win.GetDC(0)
	if dc == 0 {
		return 1
	}
	dpi := win.GetDeviceCaps(dc, win.LOGPIXELSX)
	win.ReleaseDC(0, dc)
	if dpi <= 0 {
		return 1
	}
	return float32(dpi) / 96
}

func logicalToPhysical(value, scale float32) int {
	return max(1, int(value*scale+0.5))
}

// ensureWindowClass registers the shared process-wide class exactly once.
func ensureWindowClass() error {
	registerWindowClassOnce.Do(func() {
		className, err := syscall.UTF16PtrFromString(windowClassName)
		if err != nil {
			registerWindowClassErr = err
			return
		}

		windowClass := win.WNDCLASSEX{
			CbSize:        uint32(unsafe.Sizeof(win.WNDCLASSEX{})),
			Style:         win.CS_HREDRAW | win.CS_VREDRAW,
			LpfnWndProc:   windowProcedureCallback,
			HInstance:     win.GetModuleHandle(nil),
			HCursor:       win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_ARROW)),
			LpszClassName: className,
		}
		if win.RegisterClassEx(&windowClass) == 0 {
			lastError := syscall.GetLastError()
			if !errors.Is(lastError, errorClassAlreadyExists) {
				registerWindowClassErr = fmt.Errorf("register window class failed: %w", lastError)
			}
		}
	})
	return registerWindowClassErr
}

// windowProcedure serializes window, renderer, and focus transitions on the UI thread.
func windowProcedure(hwnd win.HWND, message uint32, wParam, lParam uintptr) uintptr {
	value, ok := nativeWindows.Load(uintptr(hwnd))
	if !ok {
		return win.DefWindowProc(hwnd, message, wParam, lParam)
	}
	window := value.(*platformWindow)

	switch message {
	case windowCommandMessage:
		window.drainCommands()
		return 0
	case win.WM_SIZE:
		if window.renderer != nil {
			width := int(win.LOWORD(uint32(lParam)))
			height := int(win.HIWORD(uint32(lParam)))
			if err := window.renderer.resize(width, height); err != nil {
				window.setRunError(err)
				win.PostMessage(hwnd, win.WM_CLOSE, 0, 0)
			}
		}
		return 0
	case win.WM_DPICHANGED:
		dpi := uint32(win.LOWORD(uint32(wParam)))
		if dpi == 0 {
			dpi = 96
		}
		oldScale := window.scale
		if oldScale <= 0 {
			oldScale = 1
		}
		window.scale = float32(dpi) / 96
		var bounds win.RECT
		if win.GetWindowRect(hwnd, &bounds) {
			logicalWidth := float32(bounds.Right-bounds.Left) / oldScale
			logicalHeight := float32(bounds.Bottom-bounds.Top) / oldScale
			win.SetWindowPos(
				hwnd,
				0,
				bounds.Left,
				bounds.Top,
				int32(logicalToPhysical(logicalWidth, window.scale)),
				int32(logicalToPhysical(logicalHeight, window.scale)),
				win.SWP_NOACTIVATE|win.SWP_NOZORDER,
			)
		}
		win.InvalidateRect(hwnd, nil, false)
		return 0
	case win.WM_PAINT:
		var paint win.PAINTSTRUCT
		win.BeginPaint(hwnd, &paint)
		window.drawFrame(hwnd)
		win.EndPaint(hwnd, &paint)
		return 0
	case win.WM_ERASEBKGND:
		return 1
	case win.WM_ACTIVATE:
		if win.LOWORD(uint32(wParam)) == win.WA_INACTIVE {
			window.handleBlur(win.HWND(lParam))
		} else {
			window.confirmActivation()
		}
		return 0
	case win.WM_ACTIVATEAPP:
		if wParam == 0 {
			window.handleBlur(0)
		} else {
			window.confirmActivation()
		}
		return 0
	case win.WM_SETFOCUS:
		window.confirmActivation()
		return 0
	case win.WM_KILLFOCUS:
		window.handleBlur(win.HWND(wParam))
		return 0
	case win.WM_CLOSE:
		window.hideNative()
		win.DestroyWindow(hwnd)
		return 0
	case win.WM_NCDESTROY:
		result := win.DefWindowProc(hwnd, message, wParam, lParam)
		window.destroyNativeResources()
		nativeWindows.Delete(uintptr(hwnd))
		platformRuntime.Lock()
		if platformRuntime.windowCount > 0 {
			platformRuntime.windowCount--
		}
		shouldQuit := platformRuntime.windowCount == 0 && platformRuntime.messageLoopActive
		platformRuntime.Unlock()
		if shouldQuit {
			win.PostQuitMessage(0)
		}
		return result
	}

	return win.DefWindowProc(hwnd, message, wParam, lParam)
}

// call posts work to the UI thread while still allowing callbacks already on that thread to act directly.
func (w *platformWindow) call(kind windowCommandKind) windowCommandResult {
	w.mu.Lock()
	if w.hwnd == 0 {
		w.mu.Unlock()
		return windowCommandResult{err: errors.New("window is closed")}
	}
	if w.uiThreadID == win.GetCurrentThreadId() {
		w.mu.Unlock()
		return w.executeCommand(kind)
	}

	reply := make(chan windowCommandResult, 1)
	w.pending = append(w.pending, windowCommand{kind: kind, reply: reply})
	if win.PostMessage(w.hwnd, windowCommandMessage, 0, 0) == 0 {
		w.pending = w.pending[:len(w.pending)-1]
		w.mu.Unlock()
		return windowCommandResult{err: errors.New("failed to post window command")}
	}
	w.mu.Unlock()

	select {
	case result := <-reply:
		return result
	case <-w.done:
		select {
		case result := <-reply:
			return result
		default:
		}
		return windowCommandResult{err: errors.New("window closed before command completed")}
	}
}

// drainCommands swaps the queue before execution so callbacks can enqueue more work safely.
func (w *platformWindow) drainCommands() {
	w.mu.Lock()
	commands := w.pending
	w.pending = nil
	w.mu.Unlock()

	for index, command := range commands {
		if command.kind == windowCommandClose {
			command.reply <- windowCommandResult{}
			for _, remaining := range commands[index+1:] {
				remaining.reply <- windowCommandResult{err: errors.New("window closed before command completed")}
			}
			w.hideNative()
			win.DestroyWindow(w.hwnd)
			return
		}
		command.reply <- w.executeCommand(command.kind)
	}
}

func (w *platformWindow) executeCommand(kind windowCommandKind) windowCommandResult {
	switch kind {
	case windowCommandShow:
		return windowCommandResult{epoch: w.showNative()}
	case windowCommandHide:
		w.hideNative()
		return windowCommandResult{epoch: w.focus.epoch}
	case windowCommandClose:
		w.hideNative()
		win.DestroyWindow(w.hwnd)
		return windowCommandResult{}
	default:
		return windowCommandResult{err: errors.New("unknown window command")}
	}
}

// showNative combines show, foreground activation, and keyboard focus into one epoch.
func (w *platformWindow) showNative() FocusEpoch {
	if w.focus.active {
		w.setActive(false)
	}
	w.focus.epoch++
	w.focus.visible = true
	w.focus.activationConfirmed = false
	w.focus.blurGuardUntil = time.Now().Add(windowBlurGuardDuration)

	foreground := win.GetForegroundWindow()
	if foreground != 0 && !w.isWithinFocusDomain(foreground) {
		w.focus.previousForeground = normalizeRootWindow(foreground)
		w.focus.restorePreviousOnHide = true
	}

	win.ShowWindow(w.hwnd, win.SW_SHOW)
	activateWindow(w.hwnd)
	if w.isWithinFocusDomain(win.GetForegroundWindow()) {
		w.confirmActivation()
	}
	win.InvalidateRect(w.hwnd, nil, false)
	return w.focus.epoch
}

// hideNative ends the current epoch and only restores a foreground window Wox still owns.
func (w *platformWindow) hideNative() {
	if !w.focus.visible {
		return
	}

	shouldRestore := w.focus.restorePreviousOnHide && w.isWithinFocusDomain(win.GetForegroundWindow())
	previous := w.focus.previousForeground
	w.focus.visible = false
	w.focus.activationConfirmed = false
	w.focus.restorePreviousOnHide = false
	w.focus.previousForeground = 0
	w.setActive(false)
	win.ShowWindow(w.hwnd, win.SW_HIDE)

	if shouldRestore && previous != 0 {
		win.BringWindowToTop(previous)
		win.SetForegroundWindow(previous)
	}
}

// confirmActivation only accepts focus after Windows reports this focus domain as foreground.
func (w *platformWindow) confirmActivation() {
	if !w.focus.visible || !w.isWithinFocusDomain(win.GetForegroundWindow()) {
		return
	}
	w.focus.activationConfirmed = true
	w.setActive(true)
}

// handleBlur ignores internal native surfaces and transient messages from the current show transaction.
func (w *platformWindow) handleBlur(nextWindow win.HWND) {
	if !w.focus.visible || w.isWithinFocusDomain(nextWindow) {
		return
	}
	if !w.focus.activationConfirmed || time.Now().Before(w.focus.blurGuardUntil) {
		return
	}

	w.focus.restorePreviousOnHide = false
	w.focus.previousForeground = 0
	w.setActive(false)
	if w.options.HideOnBlur {
		w.hideNative()
	}
}

func (w *platformWindow) setActive(active bool) {
	if w.focus.active == active {
		return
	}
	w.focus.active = active
	if w.options.OnFocus != nil {
		w.options.OnFocus(FocusEvent{Epoch: w.focus.epoch, Active: active})
	}
}

// isWithinFocusDomain treats child and owned native windows as internal focus transfers.
func (w *platformWindow) isWithinFocusDomain(candidate win.HWND) bool {
	if candidate == 0 || w.hwnd == 0 {
		return false
	}
	selfRoot := normalizeRootWindow(w.hwnd)
	candidateRoot := normalizeRootWindow(candidate)
	return selfRoot == candidateRoot || win.IsChild(selfRoot, candidate) || win.IsChild(selfRoot, candidateRoot)
}

func normalizeRootWindow(hwnd win.HWND) win.HWND {
	if hwnd == 0 {
		return 0
	}
	root := win.GetAncestor(hwnd, win.GA_ROOTOWNER)
	if root == 0 {
		return hwnd
	}
	return root
}

// activateWindow uses thread-input attachment only after the cheap foreground request fails.
func activateWindow(hwnd win.HWND) bool {
	if win.SetForegroundWindow(hwnd) {
		win.SetFocus(hwnd)
		win.BringWindowToTop(hwnd)
		return true
	}

	foreground := win.GetForegroundWindow()
	currentThread := win.GetCurrentThreadId()
	foregroundThread := win.GetWindowThreadProcessId(foreground, nil)
	attached := foreground != 0 && foregroundThread != 0 && foregroundThread != currentThread && win.AttachThreadInput(int32(foregroundThread), int32(currentThread), true)
	win.SetForegroundWindow(hwnd)
	win.SetFocus(hwnd)
	win.BringWindowToTop(hwnd)
	if attached {
		win.AttachThreadInput(int32(foregroundThread), int32(currentThread), false)
	}
	return win.GetForegroundWindow() == hwnd
}

// drawFrame rebuilds the minimal display list only when Windows requests a paint.
func (w *platformWindow) drawFrame(hwnd win.HWND) {
	if w.renderer == nil {
		return
	}

	var client win.RECT
	if !win.GetClientRect(hwnd, &client) {
		return
	}
	displayList := DisplayList{}
	pixelSize := PixelSize{
		Width:  int(client.Right - client.Left),
		Height: int(client.Bottom - client.Top),
	}
	scale := w.scale
	if scale <= 0 {
		scale = 1
	}
	if w.options.OnFrame != nil {
		w.options.OnFrame(&displayList, FrameInfo{
			Size: Size{
				Width:  float32(pixelSize.Width) / scale,
				Height: float32(pixelSize.Height) / scale,
			},
			PixelSize: pixelSize,
			Scale:     scale,
		})
	}
	if err := w.renderer.render(&displayList, scale); err != nil {
		w.setRunError(err)
		win.PostMessage(hwnd, win.WM_CLOSE, 0, 0)
	}
}

// destroyNativeResources releases GPU state before invalidating the HWND-backed command queue.
func (w *platformWindow) destroyNativeResources() {
	if w.renderer != nil {
		w.renderer.destroy()
		w.renderer = nil
	}

	w.mu.Lock()
	w.hwnd = 0
	pending := w.pending
	w.pending = nil
	w.mu.Unlock()
	close(w.done)
	for _, command := range pending {
		command.reply <- windowCommandResult{err: errors.New("window closed before command completed")}
	}
}

func (w *platformWindow) setRunError(err error) {
	if err == nil {
		return
	}
	platformRuntime.Lock()
	if platformRuntime.runErr == nil {
		platformRuntime.runErr = err
	}
	platformRuntime.Unlock()
}
