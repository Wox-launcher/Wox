//go:build windows

package woxui

import (
	"errors"
	"fmt"
	"image"
	"math"
	"runtime"
	"runtime/cgo"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/lxn/win"
)

const (
	windowClassName         = "WoxGoUIWindow"
	windowCommandMessage    = win.WM_APP + 1
	windowTextInputMessage  = win.WM_APP + 2
	runtimeCallMessage      = win.WM_APP + 3
	windowBlurGuardDuration = 300 * time.Millisecond
	wsExNoRedirectionBitmap = 0x00200000
	errorClassAlreadyExists = syscall.Errno(1410)
	wmIMEStartComposition   = 0x010D
	wmIMEEndComposition     = 0x010E
	wmIMEComposition        = 0x010F
	wmIMEChar               = 0x0286
	wmGetObject             = 0x003D
	gcsCompositionString    = 0x0008
	gcsResultString         = 0x0800
	cfsCandidatePosition    = 0x0040
	unicodeNoCharacter      = 0xFFFF
	wmMouseHorizontalWheel  = 0x020E
	pointerScrollLine       = 40
	dwmwaUseImmersiveDark   = 20
	dwmwaWindowCorner       = 33
	dwmwaSystemBackdrop     = 38
	dwmWindowCornerRound    = 2
	dwmSystemBackdropMica   = 3
	wcaAccentPolicy         = 19
	accentAcrylicBlurBehind = 4
)

var (
	registerWindowClassOnce              sync.Once
	registerWindowClassErr               error
	windowProcedureCallback              = syscall.NewCallback(windowProcedure)
	nativeWindows                        sync.Map
	setProcessDPIAwarenessContext        = syscall.NewLazyDLL("user32.dll").NewProc("SetProcessDpiAwarenessContext")
	setThreadDPIAwarenessContext         = syscall.NewLazyDLL("user32.dll").NewProc("SetThreadDpiAwarenessContext")
	setProcessDPIAware                   = syscall.NewLazyDLL("user32.dll").NewProc("SetProcessDPIAware")
	enumDisplayMonitors                  = syscall.NewLazyDLL("user32.dll").NewProc("EnumDisplayMonitors")
	getDPIForMonitor                     = syscall.NewLazyDLL("shcore.dll").NewProc("GetDpiForMonitor")
	monitorBoundsCallback                = syscall.NewCallback(findMonitorForLogicalBounds)
	immGetContext                        = syscall.NewLazyDLL("imm32.dll").NewProc("ImmGetContext")
	immReleaseContext                    = syscall.NewLazyDLL("imm32.dll").NewProc("ImmReleaseContext")
	immGetCompositionString              = syscall.NewLazyDLL("imm32.dll").NewProc("ImmGetCompositionStringW")
	immSetCandidateWindow                = syscall.NewLazyDLL("imm32.dll").NewProc("ImmSetCandidateWindow")
	shellExecuteW                        = syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW")
	dwmSetWindowAttribute                = syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute")
	dwmExtendFrameIntoClientArea         = syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmExtendFrameIntoClientArea")
	setWindowCompositionAttribute        = syscall.NewLazyDLL("user32.dll").NewProc("SetWindowCompositionAttribute")
	postThreadMessageW                   = syscall.NewLazyDLL("user32.dll").NewProc("PostThreadMessageW")
	dpiAwarenessContextPerMonitorAwareV2 = ^uintptr(3)
	platformRuntime                      struct {
		sync.Mutex
		running           bool
		messageLoopActive bool
		uiThreadID        uint32
		windowCount       int
		runErr            error
		nextCallID        uintptr
		calls             map[uintptr]windowsRuntimeCall
	}
)

type windowsRuntimeCall struct {
	fn   func()
	done chan error
}

type windowCommandKind uint8

const (
	windowCommandShow windowCommandKind = iota
	windowCommandHide
	windowCommandSetBounds
	windowCommandSetPhysicalBounds
	windowCommandGetBounds
	windowCommandCenter
	windowCommandStartDragging
	windowCommandMinimize
	windowCommandSetHideOnBlur
	windowCommandSetAppearance
	windowCommandSetFontFamily
	windowCommandPickFile
	windowCommandOpenExternalURL
	windowCommandWriteClipboardText
	windowCommandWriteClipboardImage
	windowCommandShowWebView
	windowCommandHideWebView
	windowCommandClose
)

type windowCommand struct {
	kind           windowCommandKind
	bounds         Rect
	size           Size
	hideOnBlur     bool
	darkAppearance bool
	fontFamily     string
	fileDialog     FileDialogOptions
	externalURL    string
	clipboardText  string
	clipboard      *clipboardImage
	webView        WebViewContent
	webViewBounds  Rect
	reply          chan windowCommandResult
}

type windowCommandResult struct {
	epoch  FocusEpoch
	bounds Rect
	path   string
	err    error
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
	closedOnce sync.Once
	hwnd       win.HWND
	uiThreadID uint32
	pending    []windowCommand
	done       chan struct{}

	renderer *nativeRenderer
	webView  *windowsWebView
	focus    focusRuntime
	scale    float32

	inputState         TextInputState
	inputHighSurrogate uint16
	inputComposing     bool
	pointerInside      bool
	pointerPosition    Point
}

type candidateForm struct {
	Index        uint32
	Style        uint32
	CurrentPoint win.POINT
	Area         win.RECT
}

type windowsMargins struct {
	left   int32
	right  int32
	top    int32
	bottom int32
}

type windowsAccentPolicy struct {
	state         uint32
	flags         uint32
	gradientColor uint32
	animationID   uint32
}

type windowsCompositionAttributeData struct {
	attribute uint32
	data      uintptr
	size      uintptr
}

type monitorBoundsSearch struct {
	bounds   Rect
	bestArea float64
	scale    float32
}

func (w *platformWindow) capturePNG(path string) error {
	w.mu.Lock()
	hwnd := w.hwnd
	w.mu.Unlock()
	if hwnd == 0 {
		return errors.New("woxui: Windows window is not initialized")
	}
	desktop, virtualBounds, err := captureWindowsVirtualDesktop()
	if err != nil {
		return err
	}
	var nativeBounds win.RECT
	if !win.GetWindowRect(hwnd, &nativeBounds) {
		return errors.New("woxui: failed to read Windows capture bounds")
	}
	crop := image.Rect(
		int(nativeBounds.Left)-virtualBounds.Min.X,
		int(nativeBounds.Top)-virtualBounds.Min.Y,
		int(nativeBounds.Right)-virtualBounds.Min.X,
		int(nativeBounds.Bottom)-virtualBounds.Min.Y,
	).Intersect(desktop.Bounds())
	if crop.Empty() {
		return errors.New("woxui: Windows capture bounds are empty")
	}
	return writeScreenshotPNG(path, desktop.SubImage(crop))
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
	platformRuntime.nextCallID = 0
	platformRuntime.calls = map[uintptr]windowsRuntimeCall{}
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
		pendingCalls := platformRuntime.calls
		platformRuntime.calls = nil
		platformRuntime.Unlock()
		for _, call := range pendingCalls {
			call.done <- errors.New("window runtime stopped before UI callback ran")
		}
	}()

	comResult := win.CoInitializeEx(nil, win.COINIT_APARTMENTTHREADED)
	if !win.SUCCEEDED(comResult) {
		return fmt.Errorf("initialize COM failed with HRESULT 0x%08X", uint32(comResult))
	}
	defer win.CoUninitialize()

	if err := ensureWindowClass(); err != nil {
		return err
	}
	var queueMessage win.MSG
	win.PeekMessage(&queueMessage, 0, 0, 0, win.PM_NOREMOVE)
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
		if message.Message == runtimeCallMessage {
			runWindowsRuntimeCall(message.WParam)
			continue
		}
		win.TranslateMessage(&message)
		win.DispatchMessage(&message)
	}
}

func platformCall(fn func()) error {
	platformRuntime.Lock()
	if !platformRuntime.running {
		platformRuntime.Unlock()
		return errors.New("window runtime is not running")
	}
	uiThreadID := platformRuntime.uiThreadID
	if uiThreadID == win.GetCurrentThreadId() {
		platformRuntime.Unlock()
		fn()
		return nil
	}
	platformRuntime.nextCallID++
	callID := platformRuntime.nextCallID
	done := make(chan error, 1)
	platformRuntime.calls[callID] = windowsRuntimeCall{fn: fn, done: done}
	platformRuntime.Unlock()

	posted, _, postErr := postThreadMessageW.Call(uintptr(uiThreadID), runtimeCallMessage, callID, 0)
	if posted == 0 {
		platformRuntime.Lock()
		delete(platformRuntime.calls, callID)
		platformRuntime.Unlock()
		return fmt.Errorf("post UI callback: %w", postErr)
	}
	return <-done
}

func runWindowsRuntimeCall(callID uintptr) {
	platformRuntime.Lock()
	call, ok := platformRuntime.calls[callID]
	if ok {
		delete(platformRuntime.calls, callID)
	}
	platformRuntime.Unlock()
	if !ok {
		return
	}
	call.fn()
	call.done <- nil
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
	result := w.call(windowCommand{kind: windowCommandShow})
	return result.epoch, result.err
}

func (w *platformWindow) hide() error {
	return w.call(windowCommand{kind: windowCommandHide}).err
}

func (w *platformWindow) setBounds(bounds Rect) error {
	return w.call(windowCommand{kind: windowCommandSetBounds, bounds: bounds}).err
}

func (w *platformWindow) setPhysicalBounds(bounds Rect) error {
	return w.call(windowCommand{kind: windowCommandSetPhysicalBounds, bounds: bounds}).err
}

func (w *platformWindow) bounds() (Rect, error) {
	result := w.call(windowCommand{kind: windowCommandGetBounds})
	return result.bounds, result.err
}

func (w *platformWindow) center(size Size) error {
	return w.call(windowCommand{kind: windowCommandCenter, size: size}).err
}

func (w *platformWindow) startDragging() error {
	return w.call(windowCommand{kind: windowCommandStartDragging}).err
}

func (w *platformWindow) minimize() error {
	return w.call(windowCommand{kind: windowCommandMinimize}).err
}

func (w *platformWindow) setHideOnBlur(enabled bool) error {
	return w.call(windowCommand{kind: windowCommandSetHideOnBlur, hideOnBlur: enabled}).err
}

func (w *platformWindow) setAppearance(isDark bool) error {
	return w.call(windowCommand{kind: windowCommandSetAppearance, darkAppearance: isDark}).err
}

func (w *platformWindow) setFontFamily(family string) error {
	return w.call(windowCommand{kind: windowCommandSetFontFamily, fontFamily: family}).err
}

func (w *platformWindow) pickFile(options FileDialogOptions) (string, error) {
	result := w.call(windowCommand{kind: windowCommandPickFile, fileDialog: options})
	return result.path, result.err
}

func (w *platformWindow) openExternalURL(rawURL string) error {
	return w.call(windowCommand{kind: windowCommandOpenExternalURL, externalURL: rawURL}).err
}

func (w *platformWindow) showWebView(content WebViewContent, bounds Rect) error {
	return w.call(windowCommand{kind: windowCommandShowWebView, webView: content, webViewBounds: bounds}).err
}

func (w *platformWindow) hideWebView() error {
	return w.call(windowCommand{kind: windowCommandHideWebView}).err
}

func (w *platformWindow) writeClipboardText(text string) error {
	return w.call(windowCommand{kind: windowCommandWriteClipboardText, clipboardText: text}).err
}

func (w *platformWindow) writeClipboardImage(image *clipboardImage) error {
	return w.call(windowCommand{kind: windowCommandWriteClipboardImage, clipboard: image}).err
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

// setTextInputState stores logical editor geometry for the next native IME interaction.
func (w *platformWindow) setTextInputState(state TextInputState) error {
	w.mu.Lock()
	if w.hwnd == 0 {
		w.mu.Unlock()
		return errors.New("window is closed")
	}
	w.inputState = state
	hwnd := w.hwnd
	w.mu.Unlock()
	if win.PostMessage(hwnd, windowTextInputMessage, 0, 0) == 0 {
		return errors.New("failed to post text input state")
	}
	return nil
}

// measureText stays on the UI thread because the renderer is destroyed with its HWND.
func (w *platformWindow) measureText(text string, style TextStyle) (TextMetrics, error) {
	if win.GetCurrentThreadId() != w.uiThreadID {
		// ponytail: Route this through the command queue only when background layout exists.
		return TextMetrics{}, errors.New("text measurement must run on the Windows UI thread")
	}
	if w.renderer == nil {
		return TextMetrics{}, errors.New("window is closed")
	}
	return w.renderer.measureText(text, style)
}

func (w *platformWindow) close() error {
	return w.call(windowCommand{kind: windowCommandClose}).err
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
	exStyle := uint32(win.WS_EX_TOPMOST | win.WS_EX_TOOLWINDOW | wsExNoRedirectionBitmap)
	// Normal management windows use the taskbar; transient launcher surfaces keep utility/topmost semantics.
	if w.options.Role == WindowRoleApplication {
		exStyle = uint32(win.WS_EX_APPWINDOW | wsExNoRedirectionBitmap)
	}
	hwnd := win.CreateWindowEx(
		exStyle,
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
	applyWindowsBackdrop(hwnd, true)
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

// applyWindowsBackdrop uses Mica on Windows 11 and the existing Acrylic fallback on older systems.
func applyWindowsBackdrop(hwnd win.HWND, isDark bool) {
	dark := int32(0)
	if isDark {
		dark = 1
	}
	corner := int32(dwmWindowCornerRound)
	backdrop := int32(dwmSystemBackdropMica)
	margins := windowsMargins{left: -1, right: -1, top: -1, bottom: -1}
	if dwmSetWindowAttribute.Find() == nil {
		_, _, _ = dwmSetWindowAttribute.Call(uintptr(hwnd), dwmwaUseImmersiveDark, uintptr(unsafe.Pointer(&dark)), unsafe.Sizeof(dark))
		_, _, _ = dwmSetWindowAttribute.Call(uintptr(hwnd), dwmwaWindowCorner, uintptr(unsafe.Pointer(&corner)), unsafe.Sizeof(corner))
	}
	if dwmExtendFrameIntoClientArea.Find() == nil {
		_, _, _ = dwmExtendFrameIntoClientArea.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&margins)))
	}
	if dwmSetWindowAttribute.Find() == nil {
		result, _, _ := dwmSetWindowAttribute.Call(uintptr(hwnd), dwmwaSystemBackdrop, uintptr(unsafe.Pointer(&backdrop)), unsafe.Sizeof(backdrop))
		if int32(result) >= 0 {
			return
		}
	}
	if setWindowCompositionAttribute.Find() != nil {
		return
	}
	tint := uint32(0xCCF5F5F5)
	if isDark {
		tint = 0xCC202020
	}
	policy := windowsAccentPolicy{state: accentAcrylicBlurBehind, flags: 2, gradientColor: tint}
	data := windowsCompositionAttributeData{attribute: wcaAccentPolicy, data: uintptr(unsafe.Pointer(&policy)), size: unsafe.Sizeof(policy)}
	_, _, _ = setWindowCompositionAttribute.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&data)))
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
	case wmGetObject:
		if result := windowsAccessibilityObject(uintptr(hwnd), wParam, lParam); result != 0 {
			return result
		}
	case windowCommandMessage:
		window.drainCommands()
		return 0
	case windowTextInputMessage:
		if window.inputComposing {
			window.updateIMECandidatePosition(hwnd)
		}
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
	case win.WM_MOUSEMOVE:
		position := window.logicalPointerPosition(lParam)
		if !window.pointerInside {
			window.pointerInside = true
			win.TrackMouseEvent(&win.TRACKMOUSEEVENT{CbSize: uint32(unsafe.Sizeof(win.TRACKMOUSEEVENT{})), DwFlags: win.TME_LEAVE, HwndTrack: hwnd})
			window.emitPointer(PointerEvent{Kind: PointerEnter, Position: position, Modifiers: windowsKeyModifiers()})
		}
		window.emitPointer(PointerEvent{Kind: PointerMove, Position: position, Modifiers: windowsKeyModifiers()})
		return 0
	case win.WM_MOUSELEAVE:
		window.pointerInside = false
		window.emitPointer(PointerEvent{Kind: PointerLeave, Position: window.pointerPosition, Modifiers: windowsKeyModifiers()})
		return 0
	case win.WM_LBUTTONDOWN, win.WM_RBUTTONDOWN, win.WM_MBUTTONDOWN:
		win.SetCapture(hwnd)
		window.emitPointer(PointerEvent{Kind: PointerDown, Position: window.logicalPointerPosition(lParam), Button: windowsPointerButton(message), Modifiers: windowsKeyModifiers()})
		return 0
	case win.WM_LBUTTONUP, win.WM_RBUTTONUP, win.WM_MBUTTONUP:
		win.ReleaseCapture()
		window.emitPointer(PointerEvent{Kind: PointerUp, Position: window.logicalPointerPosition(lParam), Button: windowsPointerButton(message), Modifiers: windowsKeyModifiers()})
		return 0
	case win.WM_MOUSEWHEEL, wmMouseHorizontalWheel:
		position := win.POINT{X: win.GET_X_LPARAM(lParam), Y: win.GET_Y_LPARAM(lParam)}
		win.ScreenToClient(hwnd, &position)
		delta := float32(int16(win.HIWORD(uint32(wParam)))) / 120 * pointerScrollLine
		scroll := Point{Y: delta}
		if message == wmMouseHorizontalWheel {
			scroll = Point{X: delta}
		}
		window.emitPointer(PointerEvent{Kind: PointerScroll, Position: window.logicalPoint(position), Scroll: scroll, Modifiers: windowsKeyModifiers()})
		return 0
	case win.WM_KEYDOWN, win.WM_SYSKEYDOWN:
		if window.emitKey(wParam, true, lParam&(1<<30) != 0) {
			return 0
		}
	case win.WM_KEYUP, win.WM_SYSKEYUP:
		if window.emitKey(wParam, false, false) {
			return 0
		}
	case win.WM_CHAR:
		if window.handleUTF16Character(uint16(wParam)) {
			return 0
		}
	case win.WM_UNICHAR:
		if wParam == unicodeNoCharacter {
			return 1
		}
		if window.emitCommittedText(string(rune(wParam))) {
			return 0
		}
	case wmIMEStartComposition:
		if window.textInputEnabled() {
			window.inputComposing = true
			window.updateIMECandidatePosition(hwnd)
			return 0
		}
	case wmIMEComposition:
		if window.textInputEnabled() {
			window.handleIMEComposition(hwnd, lParam)
			return 0
		}
	case wmIMEEndComposition:
		if window.textInputEnabled() {
			window.endIMEComposition()
			return 0
		}
	case wmIMEChar:
		if window.textInputEnabled() {
			return 0
		}
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
		removeWindowsAccessibility(uintptr(hwnd))
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

func (w *platformWindow) textInputEnabled() bool {
	w.mu.Lock()
	enabled := w.inputState.Enabled
	w.mu.Unlock()
	return enabled && w.options.OnTextInput != nil
}

func (w *platformWindow) emitKey(virtualKey uintptr, down bool, repeat bool) bool {
	if w.options.OnKey == nil {
		return false
	}
	return w.options.OnKey(KeyEvent{
		Key:       windowsKey(virtualKey),
		Modifiers: windowsKeyModifiers(),
		Down:      down,
		Repeat:    repeat,
		Composing: w.inputComposing,
	})
}

func (w *platformWindow) emitTextInput(kind TextInputEventKind, text string) bool {
	if !w.textInputEnabled() {
		return false
	}
	w.options.OnTextInput(TextInputEvent{Kind: kind, Text: text})
	return true
}

func (w *platformWindow) emitCommittedText(text string) bool {
	if text == "" {
		return false
	}
	w.inputComposing = false
	return w.emitTextInput(TextInputCommit, text)
}

// handleUTF16Character combines WM_CHAR surrogate pairs before exposing UTF-8 text.
func (w *platformWindow) handleUTF16Character(value uint16) bool {
	if !w.textInputEnabled() {
		return false
	}
	if value >= 0xD800 && value <= 0xDBFF {
		w.inputHighSurrogate = value
		return true
	}
	var character rune
	if value >= 0xDC00 && value <= 0xDFFF && w.inputHighSurrogate != 0 {
		character = utf16.DecodeRune(rune(w.inputHighSurrogate), rune(value))
	} else {
		character = rune(value)
	}
	w.inputHighSurrogate = 0
	if character < 0x20 || character == 0x7F {
		return true
	}
	return w.emitCommittedText(string(character))
}

// handleIMEComposition translates IMM composition and result strings into the shared event model.
func (w *platformWindow) handleIMEComposition(hwnd win.HWND, flags uintptr) bool {
	if !w.textInputEnabled() {
		return false
	}
	handled := false
	if flags&gcsResultString != 0 {
		if text, ok := readIMEString(hwnd, gcsResultString); ok {
			handled = w.emitCommittedText(text) || handled
		}
	}
	if flags&gcsCompositionString != 0 {
		if text, ok := readIMEString(hwnd, gcsCompositionString); ok {
			w.inputComposing = text != ""
			handled = w.emitTextInput(TextInputCompose, text) || handled
		}
	}
	return handled
}

func (w *platformWindow) endIMEComposition() bool {
	if !w.inputComposing {
		return false
	}
	w.inputComposing = false
	return w.emitTextInput(TextInputCompose, "")
}

// updateIMECandidatePosition converts the logical caret rectangle to Win32 client pixels.
func (w *platformWindow) updateIMECandidatePosition(hwnd win.HWND) {
	w.mu.Lock()
	state := w.inputState
	scale := w.scale
	w.mu.Unlock()
	if !state.Enabled {
		return
	}
	context, _, _ := immGetContext.Call(uintptr(hwnd))
	if context == 0 {
		return
	}
	defer immReleaseContext.Call(uintptr(hwnd), context)
	form := candidateForm{
		Style: cfsCandidatePosition,
		CurrentPoint: win.POINT{
			X: int32(state.CursorRect.X * scale),
			Y: int32((state.CursorRect.Y + state.CursorRect.Height) * scale),
		},
	}
	immSetCandidateWindow.Call(context, uintptr(unsafe.Pointer(&form)))
}

// readIMEString copies one UTF-16 IMM payload while its input context is held.
func readIMEString(hwnd win.HWND, kind uintptr) (string, bool) {
	context, _, _ := immGetContext.Call(uintptr(hwnd))
	if context == 0 {
		return "", false
	}
	defer immReleaseContext.Call(uintptr(hwnd), context)
	byteCount, _, _ := immGetCompositionString.Call(context, kind, 0, 0)
	if int32(byteCount) < 0 {
		return "", false
	}
	if byteCount == 0 {
		return "", true
	}
	buffer := make([]uint16, int(byteCount)/2+1)
	written, _, _ := immGetCompositionString.Call(context, kind, uintptr(unsafe.Pointer(&buffer[0])), byteCount)
	if int32(written) < 0 {
		return "", false
	}
	return syscall.UTF16ToString(buffer[:int(written)/2]), true
}

func windowsKey(virtualKey uintptr) Key {
	if virtualKey >= 'A' && virtualKey <= 'Z' {
		return Key(string(rune(virtualKey - 'A' + 'a')))
	}
	if virtualKey >= '0' && virtualKey <= '9' {
		return Key(string(rune(virtualKey)))
	}
	switch virtualKey {
	case win.VK_BACK:
		return KeyBackspace
	case win.VK_TAB:
		return KeyTab
	case win.VK_RETURN:
		return KeyEnter
	case win.VK_ESCAPE:
		return KeyEscape
	case win.VK_SPACE:
		return KeySpace
	case win.VK_PRIOR:
		return KeyPageUp
	case win.VK_NEXT:
		return KeyPageDown
	case win.VK_END:
		return KeyEnd
	case win.VK_HOME:
		return KeyHome
	case win.VK_LEFT:
		return KeyArrowLeft
	case win.VK_UP:
		return KeyArrowUp
	case win.VK_RIGHT:
		return KeyArrowRight
	case win.VK_DOWN:
		return KeyArrowDown
	case win.VK_DELETE:
		return KeyDelete
	default:
		return KeyUnknown
	}
}

func windowsKeyModifiers() KeyModifiers {
	var modifiers KeyModifiers
	if win.GetKeyState(win.VK_SHIFT) < 0 {
		modifiers |= KeyModifierShift
	}
	if win.GetKeyState(win.VK_CONTROL) < 0 {
		modifiers |= KeyModifierControl
	}
	if win.GetKeyState(win.VK_MENU) < 0 {
		modifiers |= KeyModifierAlt
	}
	if win.GetKeyState(win.VK_LWIN) < 0 || win.GetKeyState(win.VK_RWIN) < 0 {
		modifiers |= KeyModifierMeta
	}
	return modifiers
}

func (w *platformWindow) logicalPointerPosition(lParam uintptr) Point {
	return w.logicalPoint(win.POINT{X: win.GET_X_LPARAM(lParam), Y: win.GET_Y_LPARAM(lParam)})
}

func (w *platformWindow) logicalPoint(point win.POINT) Point {
	scale := w.scale
	if scale <= 0 {
		scale = 1
	}
	position := Point{X: float32(point.X) / scale, Y: float32(point.Y) / scale}
	w.pointerPosition = position
	return position
}

func (w *platformWindow) emitPointer(event PointerEvent) {
	if w.options.OnPointer != nil {
		w.options.OnPointer(event)
	}
}

func windowsPointerButton(message uint32) PointerButton {
	switch message {
	case win.WM_LBUTTONDOWN, win.WM_LBUTTONUP:
		return PointerButtonPrimary
	case win.WM_RBUTTONDOWN, win.WM_RBUTTONUP:
		return PointerButtonSecondary
	case win.WM_MBUTTONDOWN, win.WM_MBUTTONUP:
		return PointerButtonMiddle
	default:
		return PointerButtonNone
	}
}

// call posts work to the UI thread while still allowing callbacks already on that thread to act directly.
func (w *platformWindow) call(command windowCommand) windowCommandResult {
	w.mu.Lock()
	if w.hwnd == 0 {
		w.mu.Unlock()
		return windowCommandResult{err: errors.New("window is closed")}
	}
	if w.uiThreadID == win.GetCurrentThreadId() {
		w.mu.Unlock()
		return w.executeCommand(command)
	}

	reply := make(chan windowCommandResult, 1)
	command.reply = reply
	w.pending = append(w.pending, command)
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
		command.reply <- w.executeCommand(command)
	}
}

func (w *platformWindow) executeCommand(command windowCommand) windowCommandResult {
	switch command.kind {
	case windowCommandShow:
		return windowCommandResult{epoch: w.showNative()}
	case windowCommandHide:
		w.hideNative()
		return windowCommandResult{epoch: w.focus.epoch}
	case windowCommandSetBounds:
		return windowCommandResult{err: w.setBoundsNative(command.bounds)}
	case windowCommandSetPhysicalBounds:
		return windowCommandResult{err: w.setPhysicalBoundsNative(command.bounds)}
	case windowCommandGetBounds:
		bounds, err := w.boundsNative()
		return windowCommandResult{bounds: bounds, err: err}
	case windowCommandCenter:
		return windowCommandResult{err: w.centerNative(command.size)}
	case windowCommandStartDragging:
		win.ReleaseCapture()
		win.SendMessage(w.hwnd, win.WM_NCLBUTTONDOWN, win.HTCAPTION, 0)
		return windowCommandResult{}
	case windowCommandMinimize:
		win.ShowWindow(w.hwnd, win.SW_MINIMIZE)
		return windowCommandResult{}
	case windowCommandSetHideOnBlur:
		w.options.HideOnBlur = command.hideOnBlur
		return windowCommandResult{}
	case windowCommandSetAppearance:
		applyWindowsBackdrop(w.hwnd, command.darkAppearance)
		return windowCommandResult{}
	case windowCommandSetFontFamily:
		if w.renderer == nil {
			return windowCommandResult{err: errors.New("window is closed")}
		}
		err := w.renderer.setFontFamily(command.fontFamily)
		if err == nil {
			win.InvalidateRect(w.hwnd, nil, false)
		}
		return windowCommandResult{err: err}
	case windowCommandPickFile:
		path, err := pickFileNative(uintptr(w.hwnd), command.fileDialog)
		return windowCommandResult{path: path, err: err}
	case windowCommandOpenExternalURL:
		return windowCommandResult{err: openExternalURLNative(w.hwnd, command.externalURL)}
	case windowCommandWriteClipboardText:
		return windowCommandResult{err: writeClipboardTextNative(uintptr(w.hwnd), command.clipboardText)}
	case windowCommandWriteClipboardImage:
		return windowCommandResult{err: writeClipboardImageNative(uintptr(w.hwnd), command.clipboard)}
	case windowCommandShowWebView:
		if w.webView == nil {
			webView, err := newWindowsWebView(uintptr(w.hwnd))
			if err != nil {
				return windowCommandResult{err: err}
			}
			w.webView = webView
		}
		return windowCommandResult{err: w.webView.show(command.webView, command.webViewBounds, w.scale)}
	case windowCommandHideWebView:
		if w.webView == nil {
			return windowCommandResult{}
		}
		return windowCommandResult{err: w.webView.hide()}
	case windowCommandClose:
		w.hideNative()
		win.DestroyWindow(w.hwnd)
		return windowCommandResult{}
	default:
		return windowCommandResult{err: errors.New("unknown window command")}
	}
}

// openExternalURLNative keeps ShellExecute and its Win32 error convention behind the shared URL contract.
func openExternalURLNative(hwnd win.HWND, rawURL string) error {
	operation, err := syscall.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	target, err := syscall.UTF16PtrFromString(rawURL)
	if err != nil {
		return err
	}
	result, _, _ := shellExecuteW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(operation)), uintptr(unsafe.Pointer(target)), 0, 0, win.SW_SHOWNORMAL)
	if result <= 32 {
		return fmt.Errorf("ShellExecuteW failed with code %d", result)
	}
	return nil
}

// setBoundsNative converts the core's per-monitor logical coordinate space back to Win32 pixels.
func (w *platformWindow) setBoundsNative(bounds Rect) error {
	search := monitorBoundsSearch{bounds: bounds}
	if enumDisplayMonitors.Find() == nil {
		handle := cgo.NewHandle(&search)
		result, _, _ := enumDisplayMonitors.Call(0, 0, monitorBoundsCallback, uintptr(handle))
		handle.Delete()
		if result == 0 {
			return errors.New("failed to enumerate Windows monitors")
		}
	}
	scale := search.scale
	if scale <= 0 {
		scale = primaryDisplayScale()
	}
	x := int32(math.Round(float64(bounds.X * scale)))
	y := int32(math.Round(float64(bounds.Y * scale)))
	width := int32(logicalToPhysical(bounds.Width, scale))
	height := int32(logicalToPhysical(bounds.Height, scale))
	if !win.SetWindowPos(w.hwnd, 0, x, y, width, height, win.SWP_NOACTIVATE|win.SWP_NOZORDER) {
		return errors.New("failed to set Windows window bounds")
	}
	win.InvalidateRect(w.hwnd, nil, false)
	return nil
}

func (w *platformWindow) setPhysicalBoundsNative(bounds Rect) error {
	if !win.SetWindowPos(w.hwnd, 0, int32(math.Round(float64(bounds.X))), int32(math.Round(float64(bounds.Y))), int32(math.Round(float64(bounds.Width))), int32(math.Round(float64(bounds.Height))), win.SWP_NOACTIVATE|win.SWP_NOZORDER) {
		return errors.New("failed to set physical Windows window bounds")
	}
	win.InvalidateRect(w.hwnd, nil, false)
	return nil
}

func (w *platformWindow) boundsNative() (Rect, error) {
	var bounds win.RECT
	if !win.GetWindowRect(w.hwnd, &bounds) {
		return Rect{}, errors.New("failed to read Windows window bounds")
	}
	monitor := win.MonitorFromWindow(w.hwnd, win.MONITOR_DEFAULTTONEAREST)
	scale := monitorScale(monitor)
	if scale <= 0 {
		scale = 1
	}
	return Rect{
		X:      float32(bounds.Left) / scale,
		Y:      float32(bounds.Top) / scale,
		Width:  float32(bounds.Right-bounds.Left) / scale,
		Height: float32(bounds.Bottom-bounds.Top) / scale,
	}, nil
}

// centerNative centers a logical client size in the nearest monitor work area.
func (w *platformWindow) centerNative(size Size) error {
	monitor := win.MonitorFromWindow(w.hwnd, win.MONITOR_DEFAULTTONEAREST)
	if monitor == 0 {
		return errors.New("failed to resolve Windows monitor")
	}
	var info win.MONITORINFO
	info.CbSize = uint32(unsafe.Sizeof(info))
	if !win.GetMonitorInfo(monitor, &info) {
		return errors.New("failed to read Windows monitor work area")
	}
	scale := monitorScale(monitor)
	width := int32(logicalToPhysical(size.Width, scale))
	height := int32(logicalToPhysical(size.Height, scale))
	width = min(width, info.RcWork.Right-info.RcWork.Left)
	height = min(height, info.RcWork.Bottom-info.RcWork.Top)
	x := info.RcWork.Left + (info.RcWork.Right-info.RcWork.Left-width)/2
	y := info.RcWork.Top + (info.RcWork.Bottom-info.RcWork.Top-height)/2
	if !win.SetWindowPos(w.hwnd, 0, x, y, width, height, win.SWP_NOACTIVATE|win.SWP_NOZORDER) {
		return errors.New("failed to center Windows window")
	}
	win.InvalidateRect(w.hwnd, nil, false)
	return nil
}

// findMonitorForLogicalBounds mirrors the logical monitor selection used by Wox core and the UI runner.
func findMonitorForLogicalBounds(monitor win.HMONITOR, _ win.HDC, _ *win.RECT, parameter uintptr) uintptr {
	search := cgo.Handle(parameter).Value().(*monitorBoundsSearch)
	var info win.MONITORINFO
	info.CbSize = uint32(unsafe.Sizeof(info))
	if !win.GetMonitorInfo(monitor, &info) {
		return 1
	}
	scale := monitorScale(monitor)
	left := float64(search.bounds.X * scale)
	top := float64(search.bounds.Y * scale)
	right := float64((search.bounds.X + search.bounds.Width) * scale)
	bottom := float64((search.bounds.Y + search.bounds.Height) * scale)
	overlapWidth := math.Max(0, math.Min(right, float64(info.RcMonitor.Right))-math.Max(left, float64(info.RcMonitor.Left)))
	overlapHeight := math.Max(0, math.Min(bottom, float64(info.RcMonitor.Bottom))-math.Max(top, float64(info.RcMonitor.Top)))
	area := overlapWidth * overlapHeight
	if area > search.bestArea {
		search.bestArea = area
		search.scale = scale
	}
	return 1
}

// monitorScale returns the effective DPI scale for one monitor.
func monitorScale(monitor win.HMONITOR) float32 {
	if getDPIForMonitor.Find() == nil {
		var dpiX uint32
		var dpiY uint32
		result, _, _ := getDPIForMonitor.Call(uintptr(monitor), 0, uintptr(unsafe.Pointer(&dpiX)), uintptr(unsafe.Pointer(&dpiY)))
		if int32(result) == 0 && dpiX > 0 {
			return float32(dpiX) / 96
		}
	}
	return 1
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

	showCommand := int32(win.SW_SHOW)
	if win.IsIconic(w.hwnd) {
		showCommand = win.SW_RESTORE
	}
	win.ShowWindow(w.hwnd, showCommand)
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
	if w.webView != nil {
		w.webView.destroy()
		w.webView = nil
	}
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
	w.closedOnce.Do(func() {
		if w.options.OnClosed != nil {
			w.options.OnClosed()
		}
	})
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
