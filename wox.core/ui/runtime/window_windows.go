//go:build windows

package woxui

import (
	"context"
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

	webviewruntime "wox/ui/runtime/internal/webview"
	"wox/util"
	"wox/util/ime"
	"wox/util/osvariant"

	"github.com/lxn/win"
)

const (
	windowClassName         = "WoxGoUIWindow"
	windowCommandMessage    = win.WM_APP + 1
	windowTextInputMessage  = win.WM_APP + 2
	runtimeCallMessage      = win.WM_APP + 3
	windowBlurGuardDuration = 300 * time.Millisecond
	// Keep the renderer warm for quick launcher toggles before accepting the measured cold-show penalty.
	windowsRendererTrimDelay       = 30 * time.Second
	windowsForegroundRestoreDelay1 = 30 * time.Millisecond
	windowsForegroundRestoreDelay2 = 200 * time.Millisecond
	wsExNoRedirectionBitmap        = 0x00200000
	errorClassAlreadyExists        = syscall.Errno(1410)
	wmIMEStartComposition          = 0x010D
	wmIMEEndComposition            = 0x010E
	wmIMEComposition               = 0x010F
	wmIMEChar                      = 0x0286
	wmGetObject                    = 0x003D
	gcsCompositionString           = 0x0008
	gcsResultString                = 0x0800
	cfsCandidatePosition           = 0x0040
	unicodeNoCharacter             = 0xFFFF
	wmMouseHorizontalWheel         = 0x020E
	pointerScrollLine              = 40
	windowsMouseActivateNoActivate = 3
	dwmwaUseImmersiveDark          = 20
	dwmwaWindowCorner              = 33
	dwmwaSystemBackdrop            = 38
	dwmWindowCornerRound           = 2
	// DWM_SYSTEMBACKDROP_TYPE starts with AUTO=0 and NONE=1; keep these material values aligned with the Windows SDK.
	dwmSystemBackdropNone    = 1
	dwmSystemBackdropMica    = 2
	dwmSystemBackdropAcrylic = 3
	dwmSystemBackdropMicaAlt = 4
	dwmSystemBackdropWox     = dwmSystemBackdropAcrylic
	wcaAccentPolicy          = 19
	accentBlurBehind         = 3
	accentAcrylicBlurBehind  = 4
	win10DarkAcrylicTint     = 0xCC202020
	win10LightAcrylicTint    = 0xCCF5F5F5
	windowsWSSizeBox         = uint32(0x00040000)
	windowsWMSizing          = uint32(0x0214)
	windowsWMNCActivate      = uint32(0x0086)
	windowsResizeGrip        = float32(10)
)

var (
	registerWindowClassOnce              sync.Once
	registerWindowClassErr               error
	stickyChangedOnce                    sync.Once
	stickyChangedMessage                 uint32
	windowProcedureCallback              = syscall.NewCallback(windowProcedure)
	nativeWindows                        sync.Map
	setProcessDPIAwarenessContext        = syscall.NewLazyDLL("user32.dll").NewProc("SetProcessDpiAwarenessContext")
	setThreadDPIAwarenessContext         = syscall.NewLazyDLL("user32.dll").NewProc("SetThreadDpiAwarenessContext")
	setProcessDPIAware                   = syscall.NewLazyDLL("user32.dll").NewProc("SetProcessDPIAware")
	getUpdateRect                        = syscall.NewLazyDLL("user32.dll").NewProc("GetUpdateRect")
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
	registerWindowMessageW               = syscall.NewLazyDLL("user32.dll").NewProc("RegisterWindowMessageW")
	isWindowProc                         = syscall.NewLazyDLL("user32.dll").NewProc("IsWindow")
	allowSetForegroundWindow             = syscall.NewLazyDLL("user32.dll").NewProc("AllowSetForegroundWindow")
	dpiAwarenessContextPerMonitorAwareV2 = ^uintptr(3)
	platformRuntime                      struct {
		sync.Mutex
		running           bool
		messageLoopActive bool
		uiThreadID        uint32
		callWindow        win.HWND
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
	windowCommandSetTopmost
	windowCommandSetAppearance
	windowCommandSetFontFamily
	windowCommandPickFile
	windowCommandSaveFile
	windowCommandSetPointerPassthrough
	windowCommandOpenExternalURL
	windowCommandWriteClipboardText
	windowCommandWriteClipboardImage
	windowCommandShowWebView
	windowCommandHideWebView
	windowCommandResetWebView
	windowCommandWebViewGoBack
	windowCommandWebViewGoForward
	windowCommandWebViewReload
	windowCommandWebViewOpenDevTools
	windowCommandWebViewOpenInBrowser
	windowCommandWebViewNavigationState
	windowCommandShowNativeFilePreview
	windowCommandHideNativeFilePreview
	windowCommandTrimRenderer
	windowCommandRestoreForeground
	windowCommandConfirmActivation
	windowCommandClose
)

type windowCommand struct {
	kind                        windowCommandKind
	bounds                      Rect
	size                        Size
	hideOnBlur                  bool
	topmost                     bool
	darkAppearance              bool
	fontFamily                  string
	fileDialog                  FileDialogOptions
	saveFile                    SaveFileOptions
	pointerPassthrough          bool
	externalURL                 string
	clipboardText               string
	clipboard                   *clipboardImage
	webView                     WebViewContent
	webViewBounds               Rect
	nativeFilePath              string
	nativeFileBounds            Rect
	nativeFilePreviewGeneration uint64
	restoreForeground           win.HWND
	reply                       chan windowCommandResult
}

type windowCommandResult struct {
	epoch      FocusEpoch
	bounds     Rect
	path       string
	navigation WebViewNavigationState
	err        error
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

	renderer                    *nativeRenderer
	rendererTrimTimer           *time.Timer
	foregroundRestoreTimers     [2]*time.Timer
	activationConfirmTimers     [2]*time.Timer
	webView                     *webviewruntime.Controller
	nativeFilePreview           *windowsFilePreview
	nativeFilePreviewGeneration uint64
	focus                       focusRuntime
	// nativeDialogActive keeps hide-on-blur from treating a Wox-owned file
	// picker as a real focus loss. IFileDialog is modal but not a child or
	// owned HWND, so isWithinFocusDomain cannot see it.
	nativeDialogActive bool
	darkAppearance     bool
	scale              float32
	// suppressDPIBounds keeps explicit programmatic bounds from being replaced by
	// Windows' drag-oriented WM_DPICHANGED suggestion during SetWindowPos.
	suppressDPIBounds bool

	inputState         TextInputState
	pointerCursor      PointerCursor
	pointerPassthrough bool
	// webViewCursorKnown distinguishes an intentional CSS cursor:none from a cursor not reported yet.
	webViewCursor         win.HCURSOR
	webViewCursorKnown    bool
	webViewPointerOver    bool
	inputHighSurrogate    uint16
	inputComposing        bool
	pointerInside         bool
	pointerPosition       Point
	pointerScreen         Point
	pointerScreenKnown    bool
	damageHistory         bufferDamageHistory
	animationFramePending bool
	animationVsyncRunning bool
	// animationVsyncEpoch retires a wait loop that is still blocked inside DwmFlush when
	// stopAnimationFrames runs, so a replacement goroutine cannot coexist with it.
	animationVsyncEpoch uint64

	smallIcon win.HICON
	bigIcon   win.HICON
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
	desktop, err := CaptureWindowsVirtualDesktop()
	if err != nil {
		return err
	}
	defer desktop.Close()
	var nativeBounds win.RECT
	if !win.GetWindowRect(hwnd, &nativeBounds) {
		return errors.New("woxui: failed to read Windows capture bounds")
	}
	crop := image.Rect(
		int(nativeBounds.Left)-desktop.Bounds.Min.X,
		int(nativeBounds.Top)-desktop.Bounds.Min.Y,
		int(nativeBounds.Right)-desktop.Bounds.Min.X,
		int(nativeBounds.Bottom)-desktop.Bounds.Min.Y,
	).Intersect(desktop.Image.Bounds())
	if crop.Empty() {
		return errors.New("woxui: Windows capture bounds are empty")
	}
	return writeWindowsCapturePNG(path, desktop.Image.SubImage(crop))
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
	platformRuntime.callWindow = 0
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
		platformRuntime.callWindow = 0
		platformRuntime.windowCount = 0
		platformRuntime.runErr = nil
		pendingCalls := platformRuntime.calls
		platformRuntime.calls = nil
		platformRuntime.Unlock()
		for _, call := range pendingCalls {
			if call.done != nil {
				call.done <- errors.New("window runtime stopped before UI callback ran")
			}
		}
	}()

	// DoDragDrop requires OLE initialization in addition to the COM apartment.
	comResult := win.OleInitialize()
	if !win.SUCCEEDED(comResult) {
		return fmt.Errorf("initialize OLE failed with HRESULT 0x%08X", uint32(comResult))
	}
	defer win.OleUninitialize()

	if err := ensureWindowClass(); err != nil {
		return err
	}
	callWindow, err := createRuntimeCallWindow()
	if err != nil {
		return err
	}
	platformRuntime.Lock()
	platformRuntime.callWindow = callWindow
	platformRuntime.Unlock()
	defer win.DestroyWindow(callWindow)
	message := new(win.MSG)
	var messagePinner runtime.Pinner
	// DispatchMessageW can re-enter Go and grow the goroutine stack while User32 still holds MSG's address.
	// Pin the shared message so User32 never writes back through a stale stack pointer after the callback.
	messagePinner.Pin(message)
	defer messagePinner.Unpin()
	win.PeekMessage(message, 0, 0, 0, win.PM_NOREMOVE)
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
	for {
		result := win.GetMessage(message, 0, 0, 0)
		if result == 0 {
			return nil
		}
		if result == -1 {
			return errors.New("GetMessage failed")
		}
		win.TranslateMessage(message)
		win.DispatchMessage(message)
	}
}

func platformCall(fn func()) error {
	done := make(chan error, 1)
	queued, err := queuePlatformCall(fn, done)
	if err != nil || !queued {
		return err
	}
	return <-done
}

// platformPost schedules fn on the native UI thread without making a COM callback wait for it.
func platformPost(fn func()) error {
	_, err := queuePlatformCall(fn, nil)
	return err
}

// queuePlatformCall executes directly on the UI thread or posts one runtime callback.
func queuePlatformCall(fn func(), done chan error) (bool, error) {
	platformRuntime.Lock()
	if !platformRuntime.running {
		platformRuntime.Unlock()
		return false, errors.New("window runtime is not running")
	}
	uiThreadID := platformRuntime.uiThreadID
	if uiThreadID == win.GetCurrentThreadId() {
		platformRuntime.Unlock()
		fn()
		return false, nil
	}
	platformRuntime.nextCallID++
	callID := platformRuntime.nextCallID
	platformRuntime.calls[callID] = windowsRuntimeCall{fn: fn, done: done}
	callWindow := platformRuntime.callWindow
	platformRuntime.Unlock()

	// Window messages survive nested modal loops; thread messages can be removed
	// without dispatch and leave synchronous callers blocked forever.
	if callWindow == 0 || win.PostMessage(callWindow, runtimeCallMessage, callID, 0) == 0 {
		platformRuntime.Lock()
		delete(platformRuntime.calls, callID)
		platformRuntime.Unlock()
		return false, fmt.Errorf("post UI callback: %w", syscall.GetLastError())
	}
	return true, nil
}

// createRuntimeCallWindow creates a message-only window for cross-thread UI calls.
func createRuntimeCallWindow() (win.HWND, error) {
	className, err := syscall.UTF16PtrFromString(windowClassName)
	if err != nil {
		return 0, err
	}
	hwnd := win.CreateWindowEx(0, className, nil, 0, 0, 0, 0, 0, win.HWND_MESSAGE, 0, win.GetModuleHandle(nil), nil)
	if hwnd == 0 {
		return 0, fmt.Errorf("create runtime call window: %w", syscall.GetLastError())
	}
	return hwnd, nil
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
	if call.done != nil {
		call.done <- nil
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
		options:        options,
		uiThreadID:     uiThreadID,
		done:           make(chan struct{}),
		darkAppearance: DefaultAppearanceIsDark(),
	}
	var pointerScreen win.POINT
	if win.GetCursorPos(&pointerScreen) {
		window.pointerScreen = Point{X: float32(pointerScreen.X), Y: float32(pointerScreen.Y)}
		window.pointerScreenKnown = true
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

// SetPhysicalBounds positions a Windows-only pixel-coordinate surface such as the screenshot overlay.
func (w *Window) SetPhysicalBounds(bounds Rect) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return errors.New("window bounds must have a positive size")
	}
	return w.native.setPhysicalBounds(bounds)
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

func (w *platformWindow) setTopmost(enabled bool) error {
	return w.call(windowCommand{kind: windowCommandSetTopmost, topmost: enabled}).err
}

func (w *platformWindow) focusReadyForBlur() bool {
	return w.focus.visible && w.focus.activationConfirmed && !time.Now().Before(w.focus.blurGuardUntil)
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

func (w *platformWindow) saveFile(options SaveFileOptions) (string, error) {
	result := w.call(windowCommand{kind: windowCommandSaveFile, saveFile: options})
	return result.path, result.err
}

func (w *platformWindow) setPointerPassthrough(enabled bool) error {
	return w.call(windowCommand{kind: windowCommandSetPointerPassthrough, pointerPassthrough: enabled}).err
}

func (w *platformWindow) openExternalURL(rawURL string) error {
	return w.call(windowCommand{kind: windowCommandOpenExternalURL, externalURL: rawURL}).err
}

func (w *platformWindow) showNativeFilePreview(path string, bounds Rect, generation uint64) error {
	return w.call(windowCommand{kind: windowCommandShowNativeFilePreview, nativeFilePath: path, nativeFileBounds: bounds, nativeFilePreviewGeneration: generation}).err
}

func (w *platformWindow) hideNativeFilePreview(generation uint64) error {
	return w.call(windowCommand{kind: windowCommandHideNativeFilePreview, nativeFilePreviewGeneration: generation}).err
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

func (w *platformWindow) invalidateRect(rect Rect) error {
	w.mu.Lock()
	hwnd := w.hwnd
	scale := w.scale
	w.mu.Unlock()
	if hwnd == 0 {
		return errors.New("window is closed")
	}
	if scale <= 0 {
		scale = 1
	}
	damage := win.RECT{
		Left: int32(math.Floor(float64(rect.X * scale))), Top: int32(math.Floor(float64(rect.Y * scale))),
		Right: int32(math.Ceil(float64((rect.X + rect.Width) * scale))), Bottom: int32(math.Ceil(float64((rect.Y + rect.Height) * scale))),
	}
	if damage.Right <= damage.Left || damage.Bottom <= damage.Top {
		return w.invalidate()
	}
	if !win.InvalidateRect(hwnd, &damage, false) {
		return errors.New("failed to invalidate window rectangle")
	}
	return nil
}

func (w *platformWindow) displayListDamageCullingEnabled() bool {
	return w.webView == nil || !w.webView.Visible()
}

// requestAnimationFrame waits for DWM vsync instead of immediately invalidating the HWND.
func (w *platformWindow) requestAnimationFrame() error {
	w.mu.Lock()
	if w.hwnd == 0 || !w.focus.visible {
		w.mu.Unlock()
		return errors.New("window is closed")
	}
	w.animationFramePending = true
	if w.animationVsyncRunning {
		w.mu.Unlock()
		return nil
	}
	w.animationVsyncRunning = true
	hwnd := w.hwnd
	epoch := w.animationVsyncEpoch
	w.mu.Unlock()
	go w.runWindowsAnimationVsync(hwnd, epoch)
	return nil
}

// stopAnimationFrames cancels the DWM wait loop started by requestAnimationFrame.
// Bumping the epoch is what actually retires the loop: clearing the running flag alone
// would let the next requestAnimationFrame start a second waiter while the first is
// still parked in DwmFlush, and both would then keep invalidating.
func (w *platformWindow) stopAnimationFrames() error {
	w.mu.Lock()
	w.animationFramePending = false
	w.animationVsyncRunning = false
	w.animationVsyncEpoch++
	w.mu.Unlock()
	return nil
}

// runWindowsAnimationVsync waits for DWM composition instead of invalidating immediately.
// Present1(1, 0) returns at once when DXGI reports OCCLUDED, so a bare invalidate() would spin.
func (w *platformWindow) runWindowsAnimationVsync(hwnd win.HWND, epoch uint64) {
	for {
		if dwmFlush.Find() == nil {
			_, _, _ = dwmFlush.Call()
		} else {
			time.Sleep(16 * time.Millisecond)
		}
		w.mu.Lock()
		// A stale epoch means a replacement waiter now owns the shared flags, so this
		// retired loop must exit without clearing them.
		if w.animationVsyncEpoch != epoch {
			w.mu.Unlock()
			return
		}
		if w.hwnd != hwnd {
			w.animationFramePending = false
			w.animationVsyncRunning = false
			w.mu.Unlock()
			return
		}
		pending := w.animationFramePending
		w.animationFramePending = false
		w.mu.Unlock()
		if pending {
			_ = w.invalidate()
		}
	}
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

func (w *platformWindow) setPointerCursor(cursor PointerCursor) error {
	w.mu.Lock()
	if w.hwnd == 0 {
		w.mu.Unlock()
		return errors.New("window is closed")
	}
	w.pointerCursor = cursor
	w.mu.Unlock()
	win.SetCursor(windowsPointerCursor(cursor))
	return nil
}

func windowsPointerCursor(cursor PointerCursor) win.HCURSOR {
	switch cursor {
	case PointerCursorText:
		return win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_IBEAM))
	case PointerCursorMove:
		return win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_SIZEALL))
	case PointerCursorCrosshair:
		return win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_CROSS))
	case PointerCursorResizeHorizontal:
		return win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_SIZEWE))
	case PointerCursorResizeVertical:
		return win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_SIZENS))
	case PointerCursorResizeNWSE:
		return win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_SIZENWSE))
	case PointerCursorResizeNESW:
		return win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_SIZENESW))
	case PointerCursorHand:
		return win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_HAND))
	}
	return win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_ARROW))
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

// windowsWindowStyle keeps every Wox window frameless. WS_THICKFRAME makes
// Desktop Acrylic go glass-like on the active window and opaque when it
// blurs, which is why Notes looked different from WebView after gaining focus.
// Resizable windows already resize through WM_NCHITTEST.
func windowsWindowStyle(_ WindowOptions) uint32 {
	return uint32(win.WS_POPUP)
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
	exStyle := uint32(win.WS_EX_TOOLWINDOW | wsExNoRedirectionBitmap)
	if w.options.Topmost || w.options.Role == WindowRoleScreenshot {
		exStyle |= win.WS_EX_TOPMOST
	}
	if w.options.Nonactivating {
		exStyle |= win.WS_EX_NOACTIVATE
	}
	// Normal management windows use the taskbar; transient launcher surfaces keep utility/topmost semantics.
	if w.options.Role == WindowRoleApplication {
		exStyle = uint32(win.WS_EX_APPWINDOW | wsExNoRedirectionBitmap)
	}
	style := windowsWindowStyle(w.options)
	hwnd := win.CreateWindowEx(
		exStyle,
		className,
		title,
		style,
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
	win.DragAcceptFiles(hwnd, true)
	w.mu.Lock()
	w.hwnd = hwnd
	w.mu.Unlock()
	dpi := win.GetDpiForWindow(hwnd)
	scale = windowsWindowScale(w.options.Role, dpi)
	w.scale = scale
	var client win.RECT
	if !win.GetClientRect(hwnd, &client) {
		win.DestroyWindow(hwnd)
		w.mu.Lock()
		w.hwnd = 0
		w.mu.Unlock()
		return errors.New("get initial client size failed")
	}
	// Screenshot windows never host WebView or other embedded native surfaces, so their virtual-
	// desktop-sized renderer does not need the second double-buffered composition swap chain.
	enableEmbeddedSurfaceOverlay := windowsRendererNeedsEmbeddedSurfaceOverlay(w.options.Role) && !w.options.Nonactivating
	renderer, err := newNativeRenderer(uintptr(hwnd), int(client.Right-client.Left), int(client.Bottom-client.Top), enableEmbeddedSurfaceOverlay)
	if err != nil {
		win.DestroyWindow(hwnd)
		w.mu.Lock()
		w.hwnd = 0
		w.mu.Unlock()
		return err
	}
	w.renderer = renderer
	if windowsWindowUsesSystemBackdrop(w.options) {
		applyWindowsBackdrop(hwnd, w.darkAppearance)
	}
	nativeWindows.Store(uintptr(hwnd), w)
	platformRuntime.Lock()
	platformRuntime.windowCount++
	platformRuntime.Unlock()
	w.applyWindowIcon()
	win.InvalidateRect(hwnd, nil, false)
	return nil
}

func windowsRendererNeedsEmbeddedSurfaceOverlay(role WindowRole) bool {
	return role != WindowRoleScreenshot
}

// Screenshot windows cover the desktop with captured pixels, so a system backdrop is unnecessary.
// More importantly, DWM can expose that backdrop for one refresh while the first swap-chain frame
// is still being composed, which appears as a full-screen gray flash when capture starts.
// Nonactivating is not part of this decision: tooltips and other overlays keep
// Desktop Acrylic the same way macOS keeps NSVisualEffectView on non-key windows.
func windowsWindowUsesSystemBackdrop(options WindowOptions) bool {
	return options.Role != WindowRoleScreenshot
}

func nativeWindowMaterialAvailable() bool {
	return true
}

// windowsPhysicalMinSize converts a logical MinSize to the physical tracking size Windows expects.
func windowsPhysicalMinSize(min Size, scale float32) (width, height int32) {
	if scale <= 0 {
		scale = 1
	}
	if min.Width > 0 {
		width = int32(logicalToPhysical(min.Width, scale))
	}
	if min.Height > 0 {
		height = int32(logicalToPhysical(min.Height, scale))
	}
	return width, height
}

// applyWindowsMinTrackSize raises the system tracking floor so live resize cannot go below MinSize.
func applyWindowsMinTrackSize(info *win.MINMAXINFO, minWidth, minHeight int32) {
	if info == nil {
		return
	}
	if minWidth > info.PtMinTrackSize.X {
		info.PtMinTrackSize.X = minWidth
	}
	if minHeight > info.PtMinTrackSize.Y {
		info.PtMinTrackSize.Y = minHeight
	}
}

// constrainWindowsMinSize keeps the dragged edge from shrinking past the physical minimum.
func constrainWindowsMinSize(edge uintptr, bounds *win.RECT, minWidth, minHeight int32) {
	if bounds == nil {
		return
	}
	width := bounds.Right - bounds.Left
	height := bounds.Bottom - bounds.Top
	if minWidth > 0 && width < minWidth {
		// WMSZ_LEFT / WMSZ_TOPLEFT / WMSZ_BOTTOMLEFT keep the right edge fixed.
		if edge == 1 || edge == 4 || edge == 7 {
			bounds.Left = bounds.Right - minWidth
		} else {
			bounds.Right = bounds.Left + minWidth
		}
	}
	if minHeight > 0 && height < minHeight {
		// WMSZ_TOP / WMSZ_TOPLEFT / WMSZ_TOPRIGHT keep the bottom edge fixed.
		if edge == 3 || edge == 4 || edge == 5 {
			bounds.Top = bounds.Bottom - minHeight
		} else {
			bounds.Bottom = bounds.Top + minHeight
		}
	}
}

// constrainWindowsAspectRatio adjusts the dragged edge while preserving the requested window ratio.
func constrainWindowsAspectRatio(edge uintptr, bounds *win.RECT, ratio float32) {
	if bounds == nil || ratio <= 0 {
		return
	}
	width := bounds.Right - bounds.Left
	height := bounds.Bottom - bounds.Top
	if width <= 0 || height <= 0 {
		return
	}
	if edge == 3 || edge == 6 {
		targetWidth := int32(math.Round(float64(float32(height) * ratio)))
		delta := targetWidth - width
		bounds.Left -= delta / 2
		bounds.Right += delta - delta/2
		return
	}
	targetHeight := int32(math.Round(float64(float32(width) / ratio)))
	if edge == 3 || edge == 4 || edge == 5 {
		bounds.Top = bounds.Bottom - targetHeight
	} else {
		bounds.Bottom = bounds.Top + targetHeight
	}
}

// windowsResizeHitTest exposes native resize edges for frameless resizable windows.
func windowsResizeHitTest(position win.POINT, bounds win.RECT, grip int32) uintptr {
	left := position.X <= grip
	right := position.X >= bounds.Right-grip
	top := position.Y <= grip
	bottom := position.Y >= bounds.Bottom-grip
	switch {
	case top && left:
		return win.HTTOPLEFT
	case top && right:
		return win.HTTOPRIGHT
	case bottom && left:
		return win.HTBOTTOMLEFT
	case bottom && right:
		return win.HTBOTTOMRIGHT
	case left:
		return win.HTLEFT
	case right:
		return win.HTRIGHT
	case top:
		return win.HTTOP
	case bottom:
		return win.HTBOTTOM
	default:
		return win.HTCLIENT
	}
}

// applyWindowsBackdrop is the Windows implementation of the process default
// material. Callers must not pick a different backdrop per window.
func applyWindowsBackdrop(hwnd win.HWND, isDark bool) {
	dark := int32(0)
	if isDark {
		dark = 1
	}
	if dwmSetWindowAttribute.Find() == nil {
		_, _, _ = dwmSetWindowAttribute.Call(uintptr(hwnd), dwmwaUseImmersiveDark, uintptr(unsafe.Pointer(&dark)), unsafe.Sizeof(dark))
	}
	if osvariant.GetCurrentPlatformVariant() == "win11" {
		applyWindows11Backdrop(hwnd)
		return
	}
	applyWindowsAccentBackdrop(hwnd, isDark)
}

func windowsUsesAccentBackdrop(platformVariant string) bool {
	return platformVariant != "win11"
}

func applyWindows11Backdrop(hwnd win.HWND) {
	corner := int32(dwmWindowCornerRound)
	backdrop := int32(dwmSystemBackdropWox)
	// Desktop Acrylic needs the frame extended into the client; Mica is a
	// wallpaper-derived opaque material and would hide the desktop behind Wox.
	margins := windowsMargins{left: -1, right: -1, top: -1, bottom: -1}
	if dwmExtendFrameIntoClientArea.Find() == nil {
		_, _, _ = dwmExtendFrameIntoClientArea.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&margins)))
	}
	if dwmSetWindowAttribute.Find() == nil {
		_, _, _ = dwmSetWindowAttribute.Call(uintptr(hwnd), dwmwaWindowCorner, uintptr(unsafe.Pointer(&corner)), unsafe.Sizeof(corner))
		_, _, _ = dwmSetWindowAttribute.Call(uintptr(hwnd), dwmwaSystemBackdrop, uintptr(unsafe.Pointer(&backdrop)), unsafe.Sizeof(backdrop))
	}
	windowsForceActiveBackdrop(hwnd)
}

// windowsForceActiveBackdrop keeps Desktop Acrylic on the live material.
// Notifications never become the foreground window, and ordinary windows
// otherwise switch to an opaque inactive tint as soon as they blur.
func windowsForceActiveBackdrop(hwnd win.HWND) {
	if hwnd == 0 {
		return
	}
	win.SendMessage(hwnd, windowsWMNCActivate, 1, 0)
}

func applyWindowsAccentBackdrop(hwnd win.HWND, isDark bool) {
	if tryApplyWindowsAccent(hwnd, accentAcrylicBlurBehind, windows10AcrylicTint(isDark), 2) ||
		tryApplyWindowsAccent(hwnd, accentBlurBehind, windows10AcrylicTint(isDark), 0) {
		margins := windowsMargins{left: -1, right: -1, top: -1, bottom: -1}
		if dwmExtendFrameIntoClientArea.Find() == nil {
			_, _, _ = dwmExtendFrameIntoClientArea.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&margins)))
		}
	}
}

func windows10AcrylicTint(isDark bool) uint32 {
	if isDark {
		return win10DarkAcrylicTint
	}
	return win10LightAcrylicTint
}

func tryApplyWindowsAccent(hwnd win.HWND, state, gradientColor, flags uint32) bool {
	if setWindowCompositionAttribute.Find() != nil {
		return false
	}
	policy := windowsAccentPolicy{state: state, flags: flags, gradientColor: gradientColor}
	data := windowsCompositionAttributeData{attribute: wcaAccentPolicy, data: uintptr(unsafe.Pointer(&policy)), size: unsafe.Sizeof(policy)}
	result, _, _ := setWindowCompositionAttribute.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&data)))
	return result != 0
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

// windowsWindowScale keeps virtual-desktop screenshot coordinates pixel-exact across monitors.
func windowsWindowScale(role WindowRole, dpi uint32) float32 {
	if role == WindowRoleScreenshot {
		return 1
	}
	if dpi == 0 {
		return 1
	}
	return float32(dpi) / 96
}

func logicalToPhysical(value, scale float32) int {
	return max(1, int(value*scale+0.5))
}

func windowsResizeNeedsPreparedFrame(currentWidth, currentHeight, width, height int) bool {
	return width >= currentWidth && height >= currentHeight && (width > currentWidth || height > currentHeight)
}

// windowsSuggestedDPIBounds validates the rectangle supplied by WM_DPICHANGED.
func windowsSuggestedDPIBounds(parameter uintptr) (win.RECT, bool) {
	if parameter == 0 {
		return win.RECT{}, false
	}
	bounds := *(*win.RECT)(unsafe.Pointer(parameter))
	return bounds, bounds.Right > bounds.Left && bounds.Bottom > bounds.Top
}

// finishWindowResize avoids rebuilding a frame that was already prepared at the target size.
func finishWindowResize(hwnd win.HWND, prepared bool) {
	if prepared {
		win.RedrawWindow(hwnd, nil, 0, win.RDW_VALIDATE|win.RDW_NOINTERNALPAINT)
		return
	}
	win.RedrawWindow(hwnd, nil, 0, win.RDW_INVALIDATE|win.RDW_UPDATENOW)
}

// prepareWindowForResize presents the target-size frame while the old HWND still clips it.
func (w *platformWindow) prepareWindowForResize(width, height int) bool {
	if !w.focus.visible || w.renderer == nil || w.renderer.handle == nil || width <= 0 || height <= 0 {
		return false
	}
	var client win.RECT
	if !win.GetClientRect(w.hwnd, &client) {
		return false
	}
	currentWidth := int(client.Right - client.Left)
	currentHeight := int(client.Bottom - client.Top)
	// Wox uses a borderless WS_POPUP, so its window and client sizes are identical.
	// Preparing a smaller dimension would expose the backdrop before the HWND catches up.
	if !windowsResizeNeedsPreparedFrame(currentWidth, currentHeight, width, height) {
		return false
	}
	w.damageHistory.reset()
	if err := w.renderer.resize(width, height); err != nil {
		if !isRecoverableRendererError(err) {
			util.GetLogger().Warn(context.Background(), fmt.Sprintf("prepare Windows window resize failed; using normal resize: %s", err.Error()))
			return false
		}
		if err = w.recoverWindowsRenderer(err); err != nil {
			util.GetLogger().Warn(context.Background(), fmt.Sprintf("prepare Windows window resize failed; using normal resize: %s", err.Error()))
			return false
		}
	}
	scale := w.scale
	if scale <= 0 {
		scale = 1
	}
	displayList := w.buildWindowsDisplayList(PixelSize{Width: width, Height: height}, scale, Rect{})
	err := w.renderWindowsDisplayList(&displayList, scale)
	if isRecoverableRendererError(err) {
		if err = w.recoverWindowsRenderer(err); err == nil {
			displayList = w.buildWindowsDisplayList(PixelSize{Width: width, Height: height}, scale, Rect{})
			err = w.renderWindowsDisplayList(&displayList, scale)
		}
	}
	if err != nil {
		util.GetLogger().Warn(context.Background(), fmt.Sprintf("prepare Windows window resize failed; using normal resize: %s", err.Error()))
		return false
	}
	if w.options.frameMetrics != nil {
		w.options.frameMetrics.markPreparedWindowResize(displayList.frameID)
	}
	if dwmFlush.Find() == nil {
		// Do not expose the larger HWND until DWM has consumed the prepared surface.
		_, _, _ = dwmFlush.Call()
	}
	return true
}

// WindowsHandle returns the HWND used by Windows-only native integrations.
func (w *Window) WindowsHandle() uintptr {
	if w == nil || w.native == nil {
		return 0
	}
	return uintptr(w.native.hwnd)
}

// windowsStickyChangedMessage registers the notification shared with WoxWindowHook64.dll.
func windowsStickyChangedMessage() uint32 {
	stickyChangedOnce.Do(func() {
		name, err := syscall.UTF16PtrFromString("Wox.WindowHook.StickyChanged.v1")
		if err != nil {
			return
		}
		result, _, _ := registerWindowMessageW.Call(uintptr(unsafe.Pointer(name)))
		stickyChangedMessage = uint32(result)
	})
	return stickyChangedMessage
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
	if message == runtimeCallMessage {
		runWindowsRuntimeCall(wParam)
		return 0
	}
	value, ok := nativeWindows.Load(uintptr(hwnd))
	if !ok {
		return win.DefWindowProc(hwnd, message, wParam, lParam)
	}
	window := value.(*platformWindow)
	if stickyMessage := windowsStickyChangedMessage(); stickyMessage != 0 && message == stickyMessage {
		if window.options.OnStickyWindowChanged != nil {
			window.options.OnStickyWindowChanged(wParam)
		}
		return 0
	}

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
	case win.WM_DROPFILES:
		window.handleFileDrop(win.HDROP(wParam))
		return 0
	case win.WM_SIZE:
		if window.renderer != nil {
			window.damageHistory.reset()
			width := int(win.LOWORD(uint32(lParam)))
			height := int(win.HIWORD(uint32(lParam)))
			err := window.renderer.resize(width, height)
			if isRecoverableRendererError(err) {
				err = window.recoverWindowsRenderer(err)
				if err == nil {
					win.InvalidateRect(hwnd, nil, false)
				}
			}
			if err != nil {
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
		// Windows supplies a DPI-correct rectangle for the in-progress move. Using
		// it avoids deriving a second size from GetWindowRect while the drag is crossing monitors.
		window.mu.Lock()
		window.scale = windowsWindowScale(window.options.Role, dpi)
		suppressDPIBounds := window.suppressDPIBounds || window.options.Role == WindowRoleScreenshot
		window.mu.Unlock()
		if !suppressDPIBounds {
			if bounds, ok := windowsSuggestedDPIBounds(lParam); ok {
				win.SetWindowPos(
					hwnd,
					0,
					bounds.Left, bounds.Top,
					bounds.Right-bounds.Left, bounds.Bottom-bounds.Top,
					win.SWP_NOACTIVATE|win.SWP_NOZORDER,
				)
			}
		}
		win.InvalidateRect(hwnd, nil, false)
		return 0
	case win.WM_PAINT:
		updateRect, hasUpdate := windowsUpdateRect(hwnd)
		var paint win.PAINTSTRUCT
		win.BeginPaint(hwnd, &paint)
		if hasUpdate {
			window.drawFrame(hwnd, updateRect)
		}
		win.EndPaint(hwnd, &paint)
		return 0
	case win.WM_ERASEBKGND:
		return 1
	case win.WM_NCCALCSIZE:
		if window.options.Resizable {
			return 0
		}
	case win.WM_NCHITTEST:
		if window.pointerPassthrough {
			return ^uintptr(0)
		}
		if window.options.Resizable {
			position := win.POINT{X: win.GET_X_LPARAM(lParam), Y: win.GET_Y_LPARAM(lParam)}
			var bounds win.RECT
			if win.ScreenToClient(hwnd, &position) && win.GetClientRect(hwnd, &bounds) {
				grip := int32(math.Ceil(float64(windowsResizeGrip * max(window.scale, 1))))
				if hit := windowsResizeHitTest(position, bounds, grip); hit != win.HTCLIENT {
					return hit
				}
			}
		}
	case win.WM_GETMINMAXINFO:
		if lParam != 0 {
			minWidth, minHeight := windowsPhysicalMinSize(window.options.MinSize, window.scale)
			applyWindowsMinTrackSize((*win.MINMAXINFO)(unsafe.Pointer(lParam)), minWidth, minHeight)
			return 0
		}
	case windowsWMSizing:
		if lParam != 0 {
			rect := (*win.RECT)(unsafe.Pointer(lParam))
			if window.options.AspectRatio > 0 {
				constrainWindowsAspectRatio(wParam, rect, window.options.AspectRatio)
			}
			minWidth, minHeight := windowsPhysicalMinSize(window.options.MinSize, window.scale)
			if minWidth > 0 || minHeight > 0 {
				constrainWindowsMinSize(wParam, rect, minWidth, minHeight)
			}
			if window.options.AspectRatio > 0 || minWidth > 0 || minHeight > 0 {
				return 1
			}
		}
	case win.WM_MOUSEACTIVATE:
		if window.options.Nonactivating {
			return windowsMouseActivateNoActivate
		}
	case win.WM_SETCURSOR:
		switch win.LOWORD(uint32(lParam)) {
		case win.HTTOPLEFT, win.HTBOTTOMRIGHT:
			win.SetCursor(windowsPointerCursor(PointerCursorResizeNWSE))
			return 1
		case win.HTTOPRIGHT, win.HTBOTTOMLEFT:
			win.SetCursor(windowsPointerCursor(PointerCursorResizeNESW))
			return 1
		case win.HTLEFT, win.HTRIGHT:
			win.SetCursor(windowsPointerCursor(PointerCursorResizeHorizontal))
			return 1
		case win.HTTOP, win.HTBOTTOM:
			win.SetCursor(windowsPointerCursor(PointerCursorResizeVertical))
			return 1
		}
		win.SetCursor(window.resolvedPointerCursor())
		return 1
	case win.WM_MOUSEMOVE:
		var screenPosition win.POINT
		screenPositionKnown := win.GetCursorPos(&screenPosition)
		screenPoint := Point{X: float32(screenPosition.X), Y: float32(screenPosition.Y)}
		pointerMoved := !screenPositionKnown || pointerPositionChanged(window.pointerScreen, screenPoint, window.pointerScreenKnown)
		if screenPositionKnown {
			window.pointerScreen = screenPoint
			window.pointerScreenKnown = true
		}
		position := window.logicalPointerPosition(lParam)
		if !window.pointerInside {
			window.pointerInside = true
			win.TrackMouseEvent(&win.TRACKMOUSEEVENT{CbSize: uint32(unsafe.Sizeof(win.TRACKMOUSEEVENT{})), DwFlags: win.TME_LEAVE, HwndTrack: hwnd})
			window.emitPointer(PointerEvent{Kind: PointerEnter, Position: position, Modifiers: windowsKeyModifiers()})
		}
		if pointerMoved {
			window.emitPointer(PointerEvent{Kind: PointerMove, Position: position, Modifiers: windowsKeyModifiers()})
		}
		return 0
	case win.WM_MOUSELEAVE:
		window.pointerInside = false
		window.emitPointer(PointerEvent{Kind: PointerLeave, Position: window.pointerPosition, Modifiers: windowsKeyModifiers()})
		return 0
	case win.WM_LBUTTONDOWN, win.WM_RBUTTONDOWN, win.WM_MBUTTONDOWN:
		// Reclaim native focus before Host routing; embedded surfaces transfer it back when they handle the press.
		if !window.options.Nonactivating {
			win.SetFocus(hwnd)
		}
		win.SetCapture(hwnd)
		window.emitPointer(PointerEvent{Kind: PointerDown, Position: window.logicalPointerPosition(lParam), Button: windowsPointerButton(message), Modifiers: windowsKeyModifiers()})
		return 0
	case win.WM_LBUTTONUP, win.WM_RBUTTONUP, win.WM_MBUTTONUP:
		win.ReleaseCapture()
		window.emitPointer(PointerEvent{Kind: PointerUp, Position: window.logicalPointerPosition(lParam), Button: windowsPointerButton(message), Modifiers: windowsKeyModifiers()})
		return 0
	case win.WM_XBUTTONDOWN, win.WM_XBUTTONUP:
		if window.handleWebViewXButton(win.HIWORD(uint32(wParam)), message == win.WM_XBUTTONDOWN) {
			return 1
		}
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
	case windowsWMNCActivate:
		if windowsWindowUsesSystemBackdrop(window.options) {
			return win.DefWindowProc(hwnd, message, 1, lParam)
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
	case win.WM_DESTROY:
		// UI Automation requires its provider map to be disconnected while the
		// HWND is still in WM_DESTROY. Waiting until WM_NCDESTROY can block the
		// only message-pump thread after native teardown has already started.
		removeWindowsAccessibility(uintptr(hwnd))
		return win.DefWindowProc(hwnd, message, wParam, lParam)
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

func (w *platformWindow) resolvedPointerCursor() win.HCURSOR {
	if w.webViewPointerOver && w.webViewCursorKnown {
		return w.webViewCursor
	}
	return windowsPointerCursor(w.pointerCursor)
}

// handleFileDrop converts the Windows HDROP payload before handing it to the portable window contract.
func (w *platformWindow) handleFileDrop(drop win.HDROP) {
	if drop == 0 {
		return
	}
	defer win.DragFinish(drop)
	count := win.DragQueryFile(drop, ^uint(0), nil, 0)
	if count == 0 || w.options.OnFileDrop == nil {
		return
	}
	paths := make([]string, 0, count)
	for index := uint(0); index < count; index++ {
		length := win.DragQueryFile(drop, index, nil, 0)
		if length == 0 {
			continue
		}
		buffer := make([]uint16, length+1)
		if win.DragQueryFile(drop, index, &buffer[0], length+1) == 0 {
			continue
		}
		paths = append(paths, syscall.UTF16ToString(buffer))
	}
	if len(paths) > 0 {
		w.options.OnFileDrop(paths)
	}
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
	if virtualKey >= 0x70 && virtualKey <= 0x87 {
		return Key(fmt.Sprintf("f%d", virtualKey-0x70+1))
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
	case win.VK_MENU, win.VK_LMENU, win.VK_RMENU:
		return KeyAlt
	case win.VK_LWIN, win.VK_RWIN:
		return KeyMeta
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
	if result, handled := w.executeWebViewCommand(command); handled {
		return result
	}
	switch command.kind {
	case windowCommandShow:
		epoch, err := w.showNative()
		return windowCommandResult{epoch: epoch, err: err}
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
	case windowCommandSetTopmost:
		return windowCommandResult{err: w.setTopmostNative(command.topmost)}
	case windowCommandSetAppearance:
		w.darkAppearance = command.darkAppearance
		if windowsWindowUsesSystemBackdrop(w.options) {
			applyWindowsBackdrop(w.hwnd, command.darkAppearance)
		}
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
		return w.withOwnedNativeDialog(func() windowCommandResult {
			path, err := pickFileNative(uintptr(w.hwnd), command.fileDialog)
			return windowCommandResult{path: path, err: err}
		})
	case windowCommandSaveFile:
		return w.withOwnedNativeDialog(func() windowCommandResult {
			path, err := saveFileNative(uintptr(w.hwnd), command.saveFile)
			return windowCommandResult{path: path, err: err}
		})
	case windowCommandSetPointerPassthrough:
		w.pointerPassthrough = command.pointerPassthrough
		exStyle := win.GetWindowLong(w.hwnd, win.GWL_EXSTYLE)
		if command.pointerPassthrough {
			exStyle |= win.WS_EX_LAYERED | win.WS_EX_TRANSPARENT
		} else {
			exStyle &^= win.WS_EX_TRANSPARENT
		}
		win.SetWindowLong(w.hwnd, win.GWL_EXSTYLE, exStyle)
		win.SetWindowPos(w.hwnd, 0, 0, 0, 0, 0, win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOZORDER|win.SWP_NOACTIVATE|win.SWP_FRAMECHANGED)
		return windowCommandResult{}
	case windowCommandOpenExternalURL:
		return windowCommandResult{err: openExternalURLNative(w.hwnd, command.externalURL)}
	case windowCommandWriteClipboardText:
		return windowCommandResult{err: writeClipboardTextNative(uintptr(w.hwnd), command.clipboardText)}
	case windowCommandWriteClipboardImage:
		return windowCommandResult{err: writeClipboardImageNative(uintptr(w.hwnd), command.clipboard)}
	case windowCommandShowNativeFilePreview:
		if command.nativeFilePreviewGeneration < w.nativeFilePreviewGeneration {
			return windowCommandResult{}
		}
		w.nativeFilePreviewGeneration = command.nativeFilePreviewGeneration
		if w.nativeFilePreview == nil || w.nativeFilePreview.path != command.nativeFilePath {
			if w.nativeFilePreview != nil {
				w.nativeFilePreview.destroy()
			}
			preview, err := newWindowsFilePreview(uintptr(w.hwnd), command.nativeFilePath, command.nativeFileBounds, w.scale)
			if err != nil {
				w.nativeFilePreview = nil
				return windowCommandResult{err: err}
			}
			w.nativeFilePreview = preview
		}
		return windowCommandResult{err: w.nativeFilePreview.show(command.nativeFileBounds, w.scale)}
	case windowCommandHideNativeFilePreview:
		if command.nativeFilePreviewGeneration < w.nativeFilePreviewGeneration {
			return windowCommandResult{}
		}
		w.nativeFilePreviewGeneration = command.nativeFilePreviewGeneration
		if w.nativeFilePreview == nil {
			return windowCommandResult{}
		}
		err := w.nativeFilePreview.hide()
		w.nativeFilePreview.destroy()
		w.nativeFilePreview = nil
		return windowCommandResult{err: err}
	case windowCommandTrimRenderer:
		if w.focus.visible || w.renderer == nil {
			return windowCommandResult{}
		}
		return windowCommandResult{err: w.renderer.trim()}
	case windowCommandRestoreForeground:
		if w.focus.visible || !isRestorableForegroundWindow(command.restoreForeground) {
			return windowCommandResult{}
		}
		foreground := win.GetForegroundWindow()
		// A different Wox window may have taken focus during the hide transition. Do not
		// let a stale retry pull focus away from that window or from a user's new app.
		if foreground != 0 && !w.isWithinFocusDomain(foreground) {
			return windowCommandResult{}
		}
		if normalizeRootWindow(foreground) == normalizeRootWindow(command.restoreForeground) {
			return windowCommandResult{}
		}
		activateWindow(command.restoreForeground)
		return windowCommandResult{}
	case windowCommandConfirmActivation:
		w.confirmActivation()
		return windowCommandResult{}
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
	// Keep w.scale in sync with the target monitor before SetWindowPos. Windows
	// sends WM_DPICHANGED synchronously while the window lands on a differently
	// scaled monitor; preserve the explicit bounds while that message is handled.
	w.mu.Lock()
	w.scale = scale
	w.suppressDPIBounds = true
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		w.suppressDPIBounds = false
		w.mu.Unlock()
	}()
	x := int32(math.Round(float64(bounds.X * scale)))
	y := int32(math.Round(float64(bounds.Y * scale)))
	width := int32(logicalToPhysical(bounds.Width, scale))
	height := int32(logicalToPhysical(bounds.Height, scale))
	prepared := w.prepareWindowForResize(int(width), int(height))
	if !win.SetWindowPos(w.hwnd, 0, x, y, width, height, win.SWP_NOACTIVATE|win.SWP_NOZORDER) {
		return errors.New("failed to set Windows window bounds")
	}
	w.syncScaleFromNativeWindow()
	finishWindowResize(w.hwnd, prepared)
	return nil
}

func (w *platformWindow) setPhysicalBoundsNative(bounds Rect) error {
	w.mu.Lock()
	w.suppressDPIBounds = true
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		w.suppressDPIBounds = false
		w.mu.Unlock()
	}()
	width := int32(math.Round(float64(bounds.Width)))
	height := int32(math.Round(float64(bounds.Height)))
	prepared := w.prepareWindowForResize(int(width), int(height))
	if !win.SetWindowPos(w.hwnd, 0, int32(math.Round(float64(bounds.X))), int32(math.Round(float64(bounds.Y))), width, height, win.SWP_NOACTIVATE|win.SWP_NOZORDER) {
		return errors.New("failed to set physical Windows window bounds")
	}
	w.syncScaleFromNativeWindow()
	finishWindowResize(w.hwnd, prepared)
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
	// Keep w.scale consistent with the monitor the window is centered on, so a
	// later move to a differently scaled monitor (or a synchronous WM_DPICHANGED)
	// does not back-compute the logical size from the previous monitor's scale.
	w.mu.Lock()
	w.scale = scale
	w.suppressDPIBounds = true
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		w.suppressDPIBounds = false
		w.mu.Unlock()
	}()
	width := int32(logicalToPhysical(size.Width, scale))
	height := int32(logicalToPhysical(size.Height, scale))
	width = min(width, info.RcWork.Right-info.RcWork.Left)
	height = min(height, info.RcWork.Bottom-info.RcWork.Top)
	x := info.RcWork.Left + (info.RcWork.Right-info.RcWork.Left-width)/2
	y := info.RcWork.Top + (info.RcWork.Bottom-info.RcWork.Top-height)/2
	prepared := w.prepareWindowForResize(int(width), int(height))
	if !win.SetWindowPos(w.hwnd, 0, x, y, width, height, win.SWP_NOACTIVATE|win.SWP_NOZORDER) {
		return errors.New("failed to center Windows window")
	}
	w.syncScaleFromNativeWindow()
	finishWindowResize(w.hwnd, prepared)
	return nil
}

// syncScaleFromNativeWindow refreshes the render scale before a programmatic frame.
func (w *platformWindow) syncScaleFromNativeWindow() {
	dpi := win.GetDpiForWindow(w.hwnd)
	if dpi == 0 && w.options.Role != WindowRoleScreenshot {
		return
	}
	w.mu.Lock()
	w.scale = windowsWindowScale(w.options.Role, dpi)
	w.mu.Unlock()
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

// showNative combines renderer restoration, show, foreground activation, and keyboard focus into one epoch.
func (w *platformWindow) showNative() (FocusEpoch, error) {
	w.stopForegroundRestoreTimers()
	w.stopActivationConfirmTimers()
	if w.rendererTrimTimer != nil {
		w.rendererTrimTimer.Stop()
		w.rendererTrimTimer = nil
	}
	if w.renderer == nil {
		return w.focus.epoch, errors.New("window is closed")
	}
	if w.renderer.handle == nil {
		if err := w.renderer.recreate(); err != nil {
			return w.focus.epoch, fmt.Errorf("restore renderer: %w", err)
		}
	}
	if w.focus.active {
		// Starting a new focus epoch is not a real focus loss.
		w.focus.active = false
	}
	w.focus.epoch++
	w.focus.visible = true
	w.focus.activationConfirmed = false
	w.focus.blurGuardUntil = time.Now().Add(windowBlurGuardDuration)

	foreground := win.GetForegroundWindow()
	if foreground != 0 && !w.isWithinFocusDomain(foreground) {
		ime.CaptureInputMethodBeforeActivation()
		w.focus.previousForeground = normalizeRootWindow(foreground)
		w.focus.restorePreviousOnHide = true
	}

	showCommand := int32(win.SW_SHOW)
	if win.IsIconic(w.hwnd) {
		showCommand = win.SW_RESTORE
	}
	w.syncScaleFromNativeWindow()
	var client win.RECT
	if win.GetClientRect(w.hwnd, &client) {
		// A monitor move can recreate the swap chain before this call. Render every
		// window before DWM exposes it so the launcher does not reveal an empty frame.
		w.drawFrame(w.hwnd, client)
	}
	if w.options.Role == WindowRoleScreenshot && dwmFlush.Find() == nil {
		// Present1 only queues the first frame. Waiting for DWM prevents the hidden
		// composition surface from becoming visible one refresh before its content.
		_, _, _ = dwmFlush.Call()
	}
	if w.options.Nonactivating {
		win.ShowWindow(w.hwnd, win.SW_SHOWNOACTIVATE)
	} else {
		win.ShowWindow(w.hwnd, showCommand)
		if activateWindow(w.hwnd) {
			w.confirmActivation()
		}
	}
	if w.options.Topmost || w.options.Role == WindowRoleScreenshot {
		win.SetWindowPos(w.hwnd, windowsHWNDTopmost, 0, 0, 0, 0, windowsTopmostShowFlags(w.options.Nonactivating))
	}
	if w.isWithinFocusDomain(win.GetForegroundWindow()) {
		w.confirmActivation()
	} else if !w.options.Nonactivating {
		// Re-Show of an already visible launcher after Settings close often skips
		// WM_ACTIVATE, and GetForegroundWindow can still name the dying HWND.
		w.scheduleActivationConfirm()
	}
	w.synchronizeBackdropAfterShow()
	win.InvalidateRect(w.hwnd, nil, false)
	return w.focus.epoch, nil
}

// synchronizeBackdropAfterShow replaces the backdrop policy cached while the HWND was hidden.
func (w *platformWindow) synchronizeBackdropAfterShow() {
	if !windowsWindowUsesSystemBackdrop(w.options) {
		return
	}
	if osvariant.GetCurrentPlatformVariant() == "win11" && dwmSetWindowAttribute.Find() == nil {
		backdrop := int32(dwmSystemBackdropNone)
		_, _, _ = dwmSetWindowAttribute.Call(uintptr(w.hwnd), dwmwaSystemBackdrop, uintptr(unsafe.Pointer(&backdrop)), unsafe.Sizeof(backdrop))
	}
	applyWindowsBackdrop(w.hwnd, w.darkAppearance)
	if dwmFlush.Find() == nil {
		_, _, _ = dwmFlush.Call()
	}
}

// hideNative ends the current epoch and only restores a foreground window Wox still owns.
func (w *platformWindow) hideNative() {
	if !w.focus.visible {
		return
	}

	shouldRestore := w.focus.restorePreviousOnHide && w.isWithinFocusDomain(win.GetForegroundWindow())
	previous := w.focus.previousForeground
	w.stopForegroundRestoreTimers()
	w.stopActivationConfirmTimers()
	w.focus.visible = false
	w.focus.activationConfirmed = false
	w.focus.restorePreviousOnHide = false
	w.focus.previousForeground = 0
	w.setActive(false)
	_ = w.stopAnimationFrames()
	win.ShowWindow(w.hwnd, win.SW_HIDE)
	_ = w.renderer.clearImageCache()
	if w.rendererTrimTimer != nil {
		w.rendererTrimTimer.Stop()
	}
	w.rendererTrimTimer = time.AfterFunc(windowsRendererTrimDelay, func() {
		_ = w.call(windowCommand{kind: windowCommandTrimRenderer})
	})

	if shouldRestore && isRestorableForegroundWindow(previous) {
		activateWindow(previous)
		w.scheduleForegroundRestore(previous)
	}
}

// stopForegroundRestoreTimers cancels retries from an earlier hide before a new focus epoch starts.
func (w *platformWindow) stopForegroundRestoreTimers() {
	for index, timer := range w.foregroundRestoreTimers {
		if timer != nil {
			timer.Stop()
			w.foregroundRestoreTimers[index] = nil
		}
	}
}

// stopActivationConfirmTimers cancels a previous show's deferred focus confirmation.
func (w *platformWindow) stopActivationConfirmTimers() {
	for index, timer := range w.activationConfirmTimers {
		if timer != nil {
			timer.Stop()
			w.activationConfirmTimers[index] = nil
		}
	}
}

// scheduleActivationConfirm retries native focus confirmation after a show that skipped WM_ACTIVATE.
func (w *platformWindow) scheduleActivationConfirm() {
	w.stopActivationConfirmTimers()
	delays := [...]time.Duration{windowsForegroundRestoreDelay1, windowsForegroundRestoreDelay2}
	for index, delay := range delays {
		w.activationConfirmTimers[index] = time.AfterFunc(delay, func() {
			_ = w.call(windowCommand{kind: windowCommandConfirmActivation})
		})
	}
}

// scheduleForegroundRestore retries activation after Windows finishes processing the hide transition.
func (w *platformWindow) scheduleForegroundRestore(hwnd win.HWND) {
	delays := [...]time.Duration{windowsForegroundRestoreDelay1, windowsForegroundRestoreDelay2}
	for index, delay := range delays {
		w.foregroundRestoreTimers[index] = time.AfterFunc(delay, func() {
			_ = w.call(windowCommand{kind: windowCommandRestoreForeground, restoreForeground: hwnd})
		})
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

// handleBlur ignores internal native surfaces, Wox-owned file pickers,
// nonactivating overlays such as tooltips, and transient messages from the
// current show transaction.
func (w *platformWindow) handleBlur(nextWindow win.HWND) {
	if !w.focus.visible || w.nativeDialogActive || w.isWithinFocusDomain(nextWindow) || isNonactivatingNativeWindow(nextWindow) {
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

// withOwnedNativeDialog marks a Wox-owned modal picker so nested WM_KILLFOCUS
// and WM_ACTIVATE messages do not hide the launcher.
func (w *platformWindow) withOwnedNativeDialog(fn func() windowCommandResult) windowCommandResult {
	w.nativeDialogActive = true
	defer func() { w.nativeDialogActive = false }()
	return fn()
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

const (
	windowsHWNDTopmost   = win.HWND(^uintptr(0))
	windowsHWNDNoTopmost = win.HWND(^uintptr(0) - 1)
)

// setTopmostNative toggles WS_EX_TOPMOST and restacks the HWND without activating it.
func (w *platformWindow) setTopmostNative(enabled bool) error {
	if w.hwnd == 0 {
		return errors.New("window is closed")
	}
	w.options.Topmost = enabled
	exStyle := win.GetWindowLong(w.hwnd, win.GWL_EXSTYLE)
	if enabled {
		exStyle |= win.WS_EX_TOPMOST
	} else {
		exStyle &^= win.WS_EX_TOPMOST
	}
	win.SetWindowLong(w.hwnd, win.GWL_EXSTYLE, exStyle)
	insertAfter := windowsHWNDNoTopmost
	if enabled {
		insertAfter = windowsHWNDTopmost
	}
	win.SetWindowPos(w.hwnd, insertAfter, 0, 0, 0, 0, win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOACTIVATE|win.SWP_FRAMECHANGED)
	return nil
}

// windowsTopmostShowFlags raises a just-shown topmost window. Nonactivating
// overlays keep SWP_NOACTIVATE so tooltips cannot steal launcher focus and
// trigger hide-on-lost-focus.
func windowsTopmostShowFlags(nonactivating bool) uint32 {
	flags := uint32(win.SWP_NOMOVE | win.SWP_NOSIZE)
	if nonactivating {
		flags |= win.SWP_NOACTIVATE
	}
	return flags
}

// isNonactivatingNativeWindow reports Wox tooltip and recording chrome that must
// not count as a real focus loss. Those HWNDs are not owned by the launcher, so
// isWithinFocusDomain cannot see them.
func isNonactivatingNativeWindow(hwnd win.HWND) bool {
	if hwnd == 0 {
		return false
	}
	value, ok := nativeWindows.Load(uintptr(hwnd))
	if !ok {
		return false
	}
	window, ok := value.(*platformWindow)
	return ok && window != nil && window.options.Nonactivating
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

// isRestorableForegroundWindow rejects stale or minimized targets captured before Wox was shown.
func isRestorableForegroundWindow(hwnd win.HWND) bool {
	if hwnd == 0 {
		return false
	}
	result, _, _ := isWindowProc.Call(uintptr(hwnd))
	return result != 0 && !win.IsIconic(hwnd)
}

// activateWindow mirrors the old Flutter focus handoff with direct, thread-input, and permission fallbacks.
func activateWindow(hwnd win.HWND) bool {
	if hwnd == 0 {
		return false
	}
	if win.SetForegroundWindow(hwnd) {
		win.SetFocus(hwnd)
		win.BringWindowToTop(hwnd)
		return true
	}

	// Attach to the current foreground thread. Attaching to hwnd's own thread is a
	// no-op when this runs on the window thread after Settings close or a smoke Show.
	currentThread := win.GetCurrentThreadId()
	foregroundThread := win.GetWindowThreadProcessId(win.GetForegroundWindow(), nil)
	attached := foregroundThread != 0 && foregroundThread != currentThread && win.AttachThreadInput(int32(foregroundThread), int32(currentThread), true)
	win.SetForegroundWindow(hwnd)
	win.SetFocus(hwnd)
	win.BringWindowToTop(hwnd)
	if attached {
		win.AttachThreadInput(int32(foregroundThread), int32(currentThread), false)
	}
	if normalizeRootWindow(win.GetForegroundWindow()) == normalizeRootWindow(hwnd) {
		return true
	}

	// Windows can still reject the request while the hide/show activation transition is settling.
	_, _, _ = allowSetForegroundWindow.Call(uintptr(^uint32(0)))
	win.SetForegroundWindow(hwnd)
	win.BringWindowToTop(hwnd)
	return normalizeRootWindow(win.GetForegroundWindow()) == normalizeRootWindow(hwnd)
}

// drawFrame rebuilds the minimal display list only when Windows requests a paint.
func (w *platformWindow) drawFrame(hwnd win.HWND, paint win.RECT) {
	if w.renderer == nil || w.renderer.handle == nil {
		return
	}

	var client win.RECT
	if !win.GetClientRect(hwnd, &client) {
		return
	}
	pixelSize := PixelSize{
		Width:  int(client.Right - client.Left),
		Height: int(client.Bottom - client.Top),
	}
	scale := w.scale
	if scale <= 0 {
		scale = 1
	}
	currentDamage, currentFull := windowsPaintDamage(paint, client, scale)
	damage := w.damageHistory.accumulate(currentDamage, currentFull)
	displayList := w.buildWindowsDisplayList(pixelSize, scale, damage)
	err := w.renderWindowsDisplayList(&displayList, scale)
	if isRecoverableRendererError(err) {
		if err = w.recoverWindowsRenderer(err); err == nil {
			displayList = w.buildWindowsDisplayList(pixelSize, scale, Rect{})
			err = w.renderWindowsDisplayList(&displayList, scale)
		}
	}
	if err != nil {
		w.setRunError(err)
		win.PostMessage(hwnd, win.WM_CLOSE, 0, 0)
	}
}

// recoverWindowsRenderer replaces device-bound resources after a recoverable GPU failure.
func (w *platformWindow) recoverWindowsRenderer(cause error) error {
	util.GetLogger().Warn(context.Background(), fmt.Sprintf("recovering Windows renderer after device loss: %s", cause.Error()))
	if w.webView != nil {
		w.clearWebViewPointerState()
		w.webView.Close()
		w.webView = nil
	}
	if err := w.renderer.recreate(); err != nil {
		return fmt.Errorf("%v; recreate renderer: %w", cause, err)
	}
	w.damageHistory.reset()
	return nil
}

// buildWindowsDisplayList rebuilds a frame so device recovery can retry with full damage.
func (w *platformWindow) buildWindowsDisplayList(pixelSize PixelSize, scale float32, damage Rect) DisplayList {
	displayList := DisplayList{}
	if w.options.frameMetrics != nil {
		displayList.frameID = w.options.frameMetrics.beginFrame()
	}
	if w.options.OnFrame != nil {
		w.options.OnFrame(&displayList, FrameInfo{
			Size: Size{
				Width:  float32(pixelSize.Width) / scale,
				Height: float32(pixelSize.Height) / scale,
			},
			PixelSize: pixelSize,
			Scale:     scale,
			Damage:    damage,
		})
	}
	return displayList
}

func (w *platformWindow) renderWindowsDisplayList(displayList *DisplayList, scale float32) error {
	nativeStart := time.Now()
	err := w.renderer.render(displayList, scale)
	if w.options.frameMetrics != nil {
		w.options.frameMetrics.recordEncodedResources(displayList)
		w.options.frameMetrics.finishNativeFrame(displayList.frameID, time.Since(nativeStart), -1, err == nil)
	}
	return err
}

func windowsPaintDamage(paint, client win.RECT, scale float32) (Rect, bool) {
	if scale <= 0 || paint.Right <= paint.Left || paint.Bottom <= paint.Top || (paint.Left <= client.Left && paint.Top <= client.Top && paint.Right >= client.Right && paint.Bottom >= client.Bottom) {
		return Rect{}, true
	}
	return Rect{X: float32(paint.Left) / scale, Y: float32(paint.Top) / scale, Width: float32(paint.Right-paint.Left) / scale, Height: float32(paint.Bottom-paint.Top) / scale}, false
}

// windowsUpdateRect reads the update region before BeginPaint validates it.
func windowsUpdateRect(hwnd win.HWND) (win.RECT, bool) {
	var rect win.RECT
	result, _, _ := getUpdateRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rect)), 0)
	return rect, result != 0
}

// destroyNativeResources releases GPU state before invalidating the HWND-backed command queue.
func (w *platformWindow) destroyNativeResources() {
	w.stopForegroundRestoreTimers()
	if w.rendererTrimTimer != nil {
		w.rendererTrimTimer.Stop()
		w.rendererTrimTimer = nil
	}
	if w.nativeFilePreview != nil {
		w.nativeFilePreview.destroy()
		w.nativeFilePreview = nil
	}
	if w.webView != nil {
		w.clearWebViewPointerState()
		w.webView.Close()
		w.webView = nil
	}
	if w.renderer != nil {
		w.renderer.destroy()
		w.renderer = nil
	}
	w.destroyWindowIcons()

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
