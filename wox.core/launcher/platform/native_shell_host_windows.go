//go:build windows

package platform

import (
	"context"
	"fmt"
	"math"
	"sync"
	"syscall"
	"time"
	"unsafe"
	"wox/common"
	launchertheme "wox/launcher/theme"
	"wox/util"

	"github.com/lxn/win"
)

const nativeShellWindowClassName = "WoxNativeLauncherShellWindow"

var (
	nativeShellClassOnce sync.Once
	nativeShellClassErr  error
	nativeShellWndProc   = syscall.NewCallback(nativeShellWindowProc)
	nativeShellEditProc  = syscall.NewCallback(nativeShellEditWindowProc)

	modDwmapi                        = syscall.NewLazyDLL("dwmapi.dll")
	procDwmSetWindowAttribute        = modDwmapi.NewProc("DwmSetWindowAttribute")
	procDwmExtendFrameIntoClientArea = modDwmapi.NewProc("DwmExtendFrameIntoClientArea")
	modGdi32                         = syscall.NewLazyDLL("gdi32.dll")
	procCreateFontW                  = modGdi32.NewProc("CreateFontW")
	procCreateSolidBrush             = modGdi32.NewProc("CreateSolidBrush")
	procDeleteObject                 = modGdi32.NewProc("DeleteObject")
	modUser32                        = syscall.NewLazyDLL("user32.dll")
	procDrawTextW                    = modUser32.NewProc("DrawTextW")
	procFillRect                     = modUser32.NewProc("FillRect")
	procSetWindowCompositionAttr     = modUser32.NewProc("SetWindowCompositionAttribute")
	modUxTheme                       = syscall.NewLazyDLL("uxtheme.dll")
	procBufferedPaintInit            = modUxTheme.NewProc("BufferedPaintInit")
	procBeginBufferedPaint           = modUxTheme.NewProc("BeginBufferedPaint")
	procBufferedPaintClear           = modUxTheme.NewProc("BufferedPaintClear")
	procEndBufferedPaint             = modUxTheme.NewProc("EndBufferedPaint")

	nativeShellControllers     sync.Map
	nativeShellEditControllers sync.Map
	bufferedPaintInitOnce      sync.Once
)

const (
	dwMWAUseImmersiveDarkMode     = 20
	dwMWAWindowCornerPreference   = 33
	dwMWASystemBackdropType       = 38
	dwMWCPRound                   = 2
	dwMSBTNone                    = 1
	dwMSBTTransientWindow         = 3
	wcaAccentPolicy               = 19
	accentEnableAcrylicBlurBehind = 4
	accentEnableHostBackdrop      = 5
	errorClassAlreadyExists       = 1410
	defaultNativeShellPaddingX    = 24
	defaultNativeShellPaddingY    = 20
	defaultNativeShellTitleY      = 18
	defaultNativeShellTitleHeight = 24
	defaultQueryBoxInnerPaddingX  = 14
	defaultResultTitleHeight      = 24
	defaultResultSubtitleHeight   = 18
	defaultResultItemSpacingY     = 6
	defaultPaneGapWidth           = 16
	defaultPreviewTitleHeight     = 28
	defaultPreviewPaddingX        = 12
	defaultPreviewPaddingY        = 8
	defaultWindowTitleFontSize    = 13
	defaultResultTitleFontSize    = 18
	defaultResultSubtitleFontSize = 12
	defaultPreviewTitleFontSize   = 16
	defaultPreviewBodyFontSize    = 13
	defaultUIFontFamily           = "Segoe UI"
	ecLeftMargin                  = 0x1
	ecRightMargin                 = 0x2
	fwNormal                      = 400
	fwSemibold                    = 600
	bpbfTopDownDIB                = 2
)

type margins struct {
	Left   int32
	Right  int32
	Top    int32
	Bottom int32
}

type accentPolicy struct {
	AccentState   uint32
	AccentFlags   uint32
	GradientColor uint32
	AnimationID   uint32
}

type windowCompositionAttribData struct {
	Attrib uint32
	Data   unsafe.Pointer
	Size   uintptr
}

type windowsNativeShellController struct {
	mu                 sync.RWMutex
	started            bool
	visible            bool
	focused            bool
	appearance         WindowAppearance
	showRequest        ShowRequest
	textInputState     TextInputState
	textChangeHandler  TextInputChangeHandler
	navigationHandler  TextInputSelectionNavigationHandler
	submitHandler      TextInputSubmitHandler
	suppressTextChange bool
	queryFrameAbsolute bool

	windowHandle win.HWND
	editControl  win.HWND
	editWndProc  uintptr
	commands     chan func()
	threadDone   chan struct{}

	queryBrush      win.HBRUSH
	titleFont       win.HFONT
	resultFont      win.HFONT
	subFont         win.HFONT
	previewFont     win.HFONT
	previewBodyFont win.HFONT
}

type WindowsNativeShellHost struct {
	controller *windowsNativeShellController
}

type WindowsNativeShellTextInput struct {
	controller *windowsNativeShellController
}

func NewWindowsNativeShellBundle() Bundle {
	controller := &windowsNativeShellController{}
	return Bundle{
		Host: &WindowsNativeShellHost{
			controller: controller,
		},
		TextInput: &WindowsNativeShellTextInput{
			controller: controller,
		},
	}
}

func NewWindowsNativeShellHost() *WindowsNativeShellHost {
	return NewWindowsNativeShellBundle().Host.(*WindowsNativeShellHost)
}

func (h *WindowsNativeShellHost) TextInputHost() *WindowsNativeShellTextInput {
	return &WindowsNativeShellTextInput{controller: h.controller}
}

func (h *WindowsNativeShellHost) Start(ctx context.Context, options StartOptions) error {
	_ = ctx
	return h.controller.start(options.Appearance)
}

func (h *WindowsNativeShellHost) Stop(ctx context.Context) error {
	_ = ctx
	return h.controller.stop()
}

func (h *WindowsNativeShellHost) Show(ctx context.Context, request ShowRequest) error {
	return h.controller.show(ctx, request)
}

func (h *WindowsNativeShellHost) Hide(ctx context.Context) error {
	_ = ctx
	return h.controller.hide()
}

func (h *WindowsNativeShellHost) IsVisible(ctx context.Context) bool {
	_ = ctx
	return h.controller.isVisible()
}

func (h *WindowsNativeShellHost) NativeWindowHandle(ctx context.Context) uintptr {
	_ = ctx
	return h.controller.nativeWindowHandle()
}

func (h *WindowsNativeShellHost) DebugSnapshot(ctx context.Context) HostDebugSnapshot {
	_ = ctx
	var frame Rect
	h.controller.mu.RLock()
	windowHandle := h.controller.windowHandle
	h.controller.mu.RUnlock()
	if windowHandle != 0 {
		var rect win.RECT
		if win.GetWindowRect(windowHandle, &rect) {
			frame = Rect{
				X:      int(rect.Left),
				Y:      int(rect.Top),
				Width:  int(rect.Right - rect.Left),
				Height: int(rect.Bottom - rect.Top),
			}
		}
	}
	return HostDebugSnapshot{
		Visible:            h.controller.isVisible(),
		NativeWindowHandle: h.controller.nativeWindowHandle(),
		WindowFrame:        frame,
	}
}

func (h *WindowsNativeShellHost) SupportsEmbeddedTextInput(ctx context.Context) bool {
	_ = ctx
	return true
}

func (t *WindowsNativeShellTextInput) Start(ctx context.Context) error {
	_ = ctx
	return nil
}

func (t *WindowsNativeShellTextInput) Stop(ctx context.Context) error {
	_ = ctx
	return nil
}

func (t *WindowsNativeShellTextInput) UpdateState(ctx context.Context, state TextInputState) error {
	_ = ctx
	return t.controller.updateTextInputState(state)
}

func (t *WindowsNativeShellTextInput) Focus(ctx context.Context) error {
	_ = ctx
	return t.controller.focusTextInput()
}

func (t *WindowsNativeShellTextInput) Blur(ctx context.Context) error {
	_ = ctx
	return t.controller.blurTextInput()
}

func (t *WindowsNativeShellTextInput) DebugSnapshot(ctx context.Context) TextInputDebugSnapshot {
	_ = ctx

	t.controller.mu.RLock()
	defer t.controller.mu.RUnlock()

	return TextInputDebugSnapshot{
		ParentWindowHandle: uintptr(win.GetParent(t.controller.editControl)),
		HostWindowHandle:   uintptr(t.controller.windowHandle),
		EditControlHandle:  uintptr(t.controller.editControl),
		Focused:            t.controller.focused,
		HostVisible:        t.controller.windowHandle != 0 && win.IsWindowVisible(t.controller.windowHandle),
		EditVisible:        t.controller.editControl != 0 && win.IsWindowVisible(t.controller.editControl),
		Frame:              t.controller.textInputState.QueryBox.Frame,
	}
}

func (t *WindowsNativeShellTextInput) SetChangeHandler(ctx context.Context, handler TextInputChangeHandler) error {
	_ = ctx
	t.controller.mu.Lock()
	t.controller.textChangeHandler = handler
	t.controller.mu.Unlock()
	return nil
}

func (t *WindowsNativeShellTextInput) SetSelectionNavigationHandler(ctx context.Context, handler TextInputSelectionNavigationHandler) error {
	_ = ctx
	t.controller.mu.Lock()
	t.controller.navigationHandler = handler
	t.controller.mu.Unlock()
	return nil
}

func (t *WindowsNativeShellTextInput) SetSubmitHandler(ctx context.Context, handler TextInputSubmitHandler) error {
	_ = ctx
	t.controller.mu.Lock()
	t.controller.submitHandler = handler
	t.controller.mu.Unlock()
	return nil
}

func (c *windowsNativeShellController) start(appearance WindowAppearance) error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return nil
	}

	ready := make(chan error, 1)
	c.commands = make(chan func())
	c.threadDone = make(chan struct{})
	c.appearance = appearance
	c.mu.Unlock()

	go c.runUIThread(ready)

	if err := <-ready; err != nil {
		c.mu.Lock()
		c.commands = nil
		c.threadDone = nil
		c.mu.Unlock()
		return err
	}

	c.mu.Lock()
	c.started = true
	c.mu.Unlock()
	return nil
}

func (c *windowsNativeShellController) stop() error {
	c.mu.Lock()
	if !c.started {
		c.mu.Unlock()
		return nil
	}
	threadDone := c.threadDone
	c.mu.Unlock()

	if err := c.call(func() {
		c.destroyNativeControlsLocked()
		if c.commands != nil {
			close(c.commands)
		}
	}); err != nil {
		return err
	}

	if threadDone != nil {
		<-threadDone
	}

	c.mu.Lock()
	c.started = false
	c.visible = false
	c.focused = false
	c.showRequest = ShowRequest{}
	c.textInputState = TextInputState{}
	c.commands = nil
	c.threadDone = nil
	c.mu.Unlock()
	return nil
}

func (c *windowsNativeShellController) show(ctx context.Context, request ShowRequest) error {
	return c.call(func() {
		c.mu.Lock()
		c.showRequest = request
		c.queryFrameAbsolute = request.ShowContext.WindowPosition != nil
		c.visible = true
		c.refreshBrushesLocked()
		c.mu.Unlock()

		c.applyThemeAppearanceLocked()
		c.applyShowRequestLocked()
		c.applyTextInputStateLocked()
		util.GetLogger().Info(ctx, fmt.Sprintf("native shell show resultsVisible=%v results=%d selectedIndex=%d previewVisible=%v", request.Results.Visible, len(request.Results.Items), request.Results.SelectedIndex, request.Preview.Visible))
		if c.windowHandle != 0 {
			win.InvalidateRect(c.windowHandle, nil, true)
		}
	})
}

func (c *windowsNativeShellController) hide() error {
	return c.call(func() {
		c.mu.Lock()
		c.visible = false
		c.focused = false
		c.mu.Unlock()

		if c.editControl != 0 {
			win.ShowWindow(c.editControl, win.SW_HIDE)
		}
		if c.windowHandle != 0 {
			win.ShowWindow(c.windowHandle, win.SW_HIDE)
		}
	})
}

func (c *windowsNativeShellController) isVisible() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.visible
}

func (c *windowsNativeShellController) nativeWindowHandle() uintptr {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return uintptr(c.windowHandle)
}

func (c *windowsNativeShellController) updateTextInputState(state TextInputState) error {
	return c.call(func() {
		c.mu.Lock()
		c.textInputState = state
		c.suppressTextChange = true
		c.mu.Unlock()

		if c.editControl == 0 {
			c.mu.Lock()
			c.suppressTextChange = false
			c.mu.Unlock()
			return
		}

		win.SendMessage(c.editControl, win.WM_SETTEXT, 0, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(state.QueryBox.Text))))
		win.SendMessage(c.editControl, win.EM_SETSEL, uintptr(state.SelectionStart), uintptr(state.SelectionEnd))
		c.mu.Lock()
		c.suppressTextChange = false
		c.mu.Unlock()
		c.applyTextInputStateLocked()
	})
}

func (c *windowsNativeShellController) focusTextInput() error {
	return c.call(func() {
		c.mu.Lock()
		c.focused = true
		c.mu.Unlock()

		if c.windowHandle != 0 {
			win.ShowWindow(c.windowHandle, win.SW_SHOW)
			win.SetForegroundWindow(c.windowHandle)
		}
		if c.editControl != 0 {
			win.ShowWindow(c.editControl, win.SW_SHOW)
			win.SetFocus(c.editControl)
		}
	})
}

func (c *windowsNativeShellController) blurTextInput() error {
	return c.call(func() {
		c.mu.Lock()
		c.focused = false
		c.mu.Unlock()

		if c.editControl != 0 {
			win.ShowWindow(c.editControl, win.SW_HIDE)
		}
	})
}

func (c *windowsNativeShellController) runUIThread(ready chan<- error) {
	runtimeLockOSThread()
	defer runtimeUnlockOSThread()
	defer close(c.threadDone)

	if err := ensureNativeShellWindowClass(); err != nil {
		ready <- err
		return
	}

	if err := c.createNativeControlsLocked(); err != nil {
		ready <- err
		return
	}
	ready <- nil

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case cmd, ok := <-c.commands:
			if !ok {
				return
			}
			if cmd == nil {
				return
			}
			cmd()
		case <-ticker.C:
		}

		var msg win.MSG
		for win.PeekMessage(&msg, 0, 0, 0, win.PM_REMOVE) {
			win.TranslateMessage(&msg)
			win.DispatchMessage(&msg)
		}
	}
}

func (c *windowsNativeShellController) call(fn func()) error {
	c.mu.RLock()
	started := c.started
	commands := c.commands
	c.mu.RUnlock()

	if !started || commands == nil {
		return nil
	}

	done := make(chan struct{})
	select {
	case commands <- func() {
		fn()
		close(done)
	}:
		<-done
		return nil
	case <-time.After(2 * time.Second):
		return fmt.Errorf("native shell command timeout")
	}
}

func ensureNativeShellWindowClass() error {
	nativeShellClassOnce.Do(func() {
		bufferedPaintInitOnce.Do(func() {
			procBufferedPaintInit.Call()
		})

		var wc win.WNDCLASSEX
		wc.CbSize = uint32(unsafe.Sizeof(wc))
		wc.LpfnWndProc = nativeShellWndProc
		wc.HInstance = win.GetModuleHandle(nil)
		wc.HCursor = win.LoadCursor(0, (*uint16)(unsafe.Pointer(uintptr(win.IDC_ARROW))))
		wc.LpszClassName = syscall.StringToUTF16Ptr(nativeShellWindowClassName)
		if atom := win.RegisterClassEx(&wc); atom == 0 {
			if err := win.GetLastError(); int(err) != errorClassAlreadyExists {
				nativeShellClassErr = fmt.Errorf("register native shell window class failed: %v", err)
			}
		}
	})
	return nativeShellClassErr
}

func (c *windowsNativeShellController) createNativeControlsLocked() error {
	window := win.CreateWindowEx(
		win.WS_EX_TOOLWINDOW|win.WS_EX_TOPMOST,
		syscall.StringToUTF16Ptr(nativeShellWindowClassName),
		syscall.StringToUTF16Ptr("Wox Native Launcher"),
		win.WS_POPUP|win.WS_CLIPCHILDREN,
		-32000,
		-32000,
		defaultShellWidth,
		defaultShellHeight,
		0,
		0,
		0,
		nil,
	)
	if window == 0 {
		return fmt.Errorf("failed to create native shell window")
	}

	edit := win.CreateWindowEx(
		0,
		syscall.StringToUTF16Ptr("EDIT"),
		syscall.StringToUTF16Ptr(""),
		win.WS_CHILD|win.WS_TABSTOP|win.ES_LEFT|win.ES_AUTOHSCROLL,
		0,
		0,
		1,
		1,
		window,
		0,
		0,
		nil,
	)
	if edit == 0 {
		win.DestroyWindow(window)
		return fmt.Errorf("failed to create native shell edit control")
	}

	win.SendMessage(edit, win.WM_SETFONT, uintptr(win.GetStockObject(win.DEFAULT_GUI_FONT)), 1)
	win.SendMessage(edit, win.EM_SETMARGINS, uintptr(ecLeftMargin|ecRightMargin), makeLPARAM(defaultQueryBoxInnerPaddingX, defaultQueryBoxInnerPaddingX))

	editWndProc := win.GetWindowLongPtr(edit, win.GWLP_WNDPROC)
	win.SetWindowLongPtr(edit, win.GWLP_WNDPROC, nativeShellEditProc)

	nativeShellControllers.Store(uintptr(window), c)
	nativeShellEditControllers.Store(uintptr(edit), c)

	titleFont := createUIFont(defaultWindowTitleFontSize, fwSemibold)
	resultFont := createUIFont(defaultResultTitleFontSize, fwSemibold)
	subFont := createUIFont(defaultResultSubtitleFontSize, fwNormal)
	previewFont := createUIFont(defaultPreviewTitleFontSize, fwSemibold)
	previewBodyFont := createUIFont(defaultPreviewBodyFontSize, fwNormal)

	c.mu.Lock()
	c.windowHandle = window
	c.editControl = edit
	c.editWndProc = editWndProc
	c.titleFont = titleFont
	c.resultFont = resultFont
	c.subFont = subFont
	c.previewFont = previewFont
	c.previewBodyFont = previewBodyFont
	c.refreshBrushesLocked()
	c.mu.Unlock()

	if previewBodyFont != 0 {
		win.SendMessage(edit, win.WM_SETFONT, uintptr(previewBodyFont), 1)
	}

	applyNativeShellAppearance(window, c.appearance, launchertheme.DefaultPaintTheme())
	return nil
}

func (c *windowsNativeShellController) destroyNativeControlsLocked() {
	if c.queryBrush != 0 {
		deleteGDIObject(uintptr(c.queryBrush))
		c.queryBrush = 0
	}
	if c.previewBodyFont != 0 {
		deleteGDIObject(uintptr(c.previewBodyFont))
		c.previewBodyFont = 0
	}
	if c.previewFont != 0 {
		deleteGDIObject(uintptr(c.previewFont))
		c.previewFont = 0
	}
	if c.subFont != 0 {
		deleteGDIObject(uintptr(c.subFont))
		c.subFont = 0
	}
	if c.resultFont != 0 {
		deleteGDIObject(uintptr(c.resultFont))
		c.resultFont = 0
	}
	if c.titleFont != 0 {
		deleteGDIObject(uintptr(c.titleFont))
		c.titleFont = 0
	}
	if c.editControl != 0 {
		if c.editWndProc != 0 {
			win.SetWindowLongPtr(c.editControl, win.GWLP_WNDPROC, c.editWndProc)
			c.editWndProc = 0
		}
		nativeShellEditControllers.Delete(uintptr(c.editControl))
		win.DestroyWindow(c.editControl)
		c.editControl = 0
	}
	if c.windowHandle != 0 {
		nativeShellControllers.Delete(uintptr(c.windowHandle))
		win.DestroyWindow(c.windowHandle)
		c.windowHandle = 0
	}
}

func (c *windowsNativeShellController) refreshBrushesLocked() {
	if c.queryBrush != 0 {
		deleteGDIObject(uintptr(c.queryBrush))
	}

	c.queryBrush = createSolidBrush(colorRefFromRGBA(resolveQueryBoxSolidColor(c.showRequest.Theme)))
}

func (c *windowsNativeShellController) applyShowRequestLocked() {
	if c.windowHandle == 0 {
		return
	}

	x, y := resolveNativeShellOrigin(c.showRequest.ShowContext)
	width := c.showRequest.ShowContext.WindowWidth
	if width <= 0 {
		width = defaultShellWidth
	}
	height := c.showRequest.WindowHeight
	if height <= 0 {
		height = defaultShellHeight
	}

	win.SetWindowPos(
		c.windowHandle,
		win.HWND_TOPMOST,
		int32(x),
		int32(y),
		int32(width),
		int32(height),
		win.SWP_NOACTIVATE,
	)
	win.ShowWindow(c.windowHandle, win.SW_SHOW)
}

func (c *windowsNativeShellController) applyThemeAppearanceLocked() {
	c.mu.RLock()
	windowHandle := c.windowHandle
	appearance := c.appearance
	theme := c.showRequest.Theme
	c.mu.RUnlock()

	if windowHandle == 0 {
		return
	}

	applyNativeShellAppearance(windowHandle, appearance, theme)
}

func (c *windowsNativeShellController) applyTextInputStateLocked() {
	if c.windowHandle == 0 || c.editControl == 0 {
		return
	}

	frame := c.textInputState.QueryBox.Frame
	if !c.textInputState.QueryBox.Visible || frame.IsEmpty() || !c.visible {
		win.ShowWindow(c.editControl, win.SW_HIDE)
		return
	}

	if c.queryFrameAbsolute {
		var rect win.RECT
		if win.GetWindowRect(c.windowHandle, &rect) {
			frame.X -= int(rect.Left)
			frame.Y -= int(rect.Top)
		}
	}

	win.SetWindowPos(
		c.editControl,
		0,
		int32(frame.X),
		int32(frame.Y),
		int32(frame.Width),
		int32(frame.Height),
		win.SWP_NOZORDER|win.SWP_NOACTIVATE,
	)
	if c.textInputState.QueryBox.Placeholder != "" {
		win.SendMessage(c.editControl, win.EM_SETCUEBANNER, 0, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(c.textInputState.QueryBox.Placeholder))))
	}
	win.ShowWindow(c.editControl, win.SW_SHOW)
}

func nativeShellWindowProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	controller := getNativeShellController(hwnd)

	switch msg {
	case win.WM_NCHITTEST:
		if controller == nil {
			break
		}
		x := int(int16(uint16(lParam & 0xffff)))
		y := int(int16(uint16((lParam >> 16) & 0xffff)))
		var rect win.RECT
		win.GetWindowRect(hwnd, &rect)
		clientX := x - int(rect.Left)
		clientY := y - int(rect.Top)

		controller.mu.RLock()
		frame := controller.textInputState.QueryBox.Frame
		absolute := controller.queryFrameAbsolute
		controller.mu.RUnlock()

		if absolute {
			frame.X -= int(rect.Left)
			frame.Y -= int(rect.Top)
		}

		if clientX >= frame.X && clientX < frame.X+frame.Width && clientY >= frame.Y && clientY < frame.Y+frame.Height {
			return uintptr(win.HTCLIENT)
		}
		return uintptr(win.HTCAPTION)
	case win.WM_CTLCOLOREDIT:
		if controller == nil {
			break
		}
		hdc := win.HDC(wParam)
		controller.mu.RLock()
		foreground := colorRefFromRGBA(resolveThemeColor(controller.showRequest.Theme.QueryBox.ForegroundColor, launchertheme.RGBAColor{R: 0xE2, G: 0xE8, B: 0xF0, A: 0xFF}))
		background := colorRefFromRGBA(resolveQueryBoxSolidColor(controller.showRequest.Theme))
		brush := controller.queryBrush
		controller.mu.RUnlock()

		win.SetTextColor(hdc, foreground)
		win.SetBkMode(hdc, win.OPAQUE)
		win.SetBkColor(hdc, background)
		return uintptr(brush)
	case win.WM_COMMAND:
		if controller == nil {
			break
		}
		if win.HWND(lParam) == controller.editControl && win.HIWORD(uint32(wParam)) == win.EN_CHANGE {
			controller.handleUserEditChange()
			return 0
		}
	case win.WM_ERASEBKGND:
		return 1
	case win.WM_PAINT:
		if controller == nil {
			break
		}
		return controller.handlePaint(hwnd)
	case win.WM_DESTROY:
		nativeShellControllers.Delete(uintptr(hwnd))
		return 0
	}

	return win.DefWindowProc(hwnd, msg, wParam, lParam)
}

func nativeShellEditWindowProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	controller := getNativeShellEditController(hwnd)
	if controller == nil {
		return win.DefWindowProc(hwnd, msg, wParam, lParam)
	}

	switch msg {
	case win.WM_KEYDOWN:
		switch wParam {
		case win.VK_UP:
			controller.handleSelectionNavigation(-1)
			return 0
		case win.VK_DOWN:
			controller.handleSelectionNavigation(1)
			return 0
		case win.VK_RETURN:
			controller.handleSubmit()
			return 0
		}
	}

	controller.mu.RLock()
	editWndProc := controller.editWndProc
	controller.mu.RUnlock()
	if editWndProc != 0 {
		return win.CallWindowProc(editWndProc, hwnd, msg, wParam, lParam)
	}

	return win.DefWindowProc(hwnd, msg, wParam, lParam)
}

func (c *windowsNativeShellController) handlePaint(hwnd win.HWND) uintptr {
	var ps win.PAINTSTRUCT
	paintHDC := win.BeginPaint(hwnd, &ps)
	defer win.EndPaint(hwnd, &ps)

	var rect win.RECT
	win.GetClientRect(hwnd, &rect)
	hdc, paintBuffer := beginBufferedPaint(paintHDC, &rect)
	if paintBuffer != 0 {
		defer endBufferedPaint(paintBuffer)
	}

	c.mu.RLock()
	queryFrame := c.textInputState.QueryBox.Frame
	queryVisible := c.textInputState.QueryBox.Visible
	absoluteFrame := c.queryFrameAbsolute
	queryRadius := c.showRequest.Theme.QueryBox.BorderRadius
	resultTitleColor := colorRefFromRGBA(resolveThemeColor(c.showRequest.Theme.Results.TitleColor, launchertheme.RGBAColor{R: 0xE2, G: 0xE8, B: 0xF0, A: 0xFF}))
	resultSubtitleColor := colorRefFromRGBA(resolveThemeColor(c.showRequest.Theme.Results.SubtitleColor, launchertheme.RGBAColor{R: 0x9C, G: 0xA3, B: 0xAF, A: 0xFF}))
	activeTitleColor := colorRefFromRGBA(resolveThemeColor(c.showRequest.Theme.Results.ActiveTitleColor, launchertheme.RGBAColor{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}))
	activeSubtitleColor := colorRefFromRGBA(resolveThemeColor(c.showRequest.Theme.Results.ActiveSubtitleColor, launchertheme.RGBAColor{R: 0xE2, G: 0xE8, B: 0xF0, A: 0xFF}))
	activeBackgroundColor := colorRefFromRGBA(resolveResultActiveColor(c.showRequest.Theme))
	resultRadius := c.showRequest.Theme.Results.BorderRadius
	previewFontColor := colorRefFromRGBA(resolveThemeColor(c.showRequest.Theme.Preview.FontColor, launchertheme.RGBAColor{R: 0xE2, G: 0xE8, B: 0xF0, A: 0xFF}))
	previewPropertyTitleColor := colorRefFromRGBA(resolveThemeColor(c.showRequest.Theme.Preview.PropertyTitleColor, launchertheme.RGBAColor{R: 0x9C, G: 0xA3, B: 0xAF, A: 0xFF}))
	previewPropertyContentColor := colorRefFromRGBA(resolveThemeColor(c.showRequest.Theme.Preview.PropertyContentColor, launchertheme.RGBAColor{R: 0xE2, G: 0xE8, B: 0xF0, A: 0xFF}))
	previewSplitLineColor := colorRefFromRGBA(resolveThemeColor(c.showRequest.Theme.Preview.SplitLineColor, launchertheme.RGBAColor{R: 0x4A, G: 0x55, B: 0x68, A: 0xFF}))
	resultState := c.showRequest.Results
	preview := c.showRequest.Preview
	resultFont := c.resultFont
	subFont := c.subFont
	previewFont := c.previewFont
	previewBodyFont := c.previewBodyFont
	c.mu.RUnlock()

	if queryVisible && !queryFrame.IsEmpty() {
		if absoluteFrame {
			var windowRect win.RECT
			if win.GetWindowRect(hwnd, &windowRect) {
				queryFrame.X -= int(windowRect.Left)
				queryFrame.Y -= int(windowRect.Top)
			}
		}

		drawRoundedQueryBox(hdc, win.RECT{
			Left:   int32(queryFrame.X),
			Top:    int32(queryFrame.Y),
			Right:  int32(queryFrame.X + queryFrame.Width),
			Bottom: int32(queryFrame.Y + queryFrame.Height),
		}, colorRefFromRGBA(resolveQueryBoxSolidColor(c.showRequest.Theme)), queryRadius)
	}

	win.SetBkMode(hdc, win.TRANSPARENT)

	if resultState.Visible && !resultState.Frame.IsEmpty() {
		if absoluteFrame {
			var windowRect win.RECT
			if win.GetWindowRect(hwnd, &windowRect) {
				resultState.Frame.X -= int(windowRect.Left)
				resultState.Frame.Y -= int(windowRect.Top)
			}
		}

		drawResultList(hdc, resultState, resultTitleColor, resultSubtitleColor, activeBackgroundColor, activeTitleColor, activeSubtitleColor, resultRadius, resultFont, subFont)
	}

	if preview.Visible && !preview.Frame.IsEmpty() {
		if absoluteFrame {
			var windowRect win.RECT
			if win.GetWindowRect(hwnd, &windowRect) {
				preview.Frame.X -= int(windowRect.Left)
				preview.Frame.Y -= int(windowRect.Top)
			}
		}

		splitX := preview.Frame.X - (defaultPaneGapWidth / 2)
		drawVerticalSplitLine(hdc, splitX, preview.Frame.Y, preview.Frame.Height, previewSplitLineColor)
		drawPreviewPane(hdc, preview.Frame, preview, previewFontColor, previewPropertyTitleColor, previewPropertyContentColor, previewFont, previewBodyFont)
	}
	return 0
}

func getNativeShellController(hwnd win.HWND) *windowsNativeShellController {
	if hwnd == 0 {
		return nil
	}
	if value, ok := nativeShellControllers.Load(uintptr(hwnd)); ok {
		controller, _ := value.(*windowsNativeShellController)
		return controller
	}
	return nil
}

func getNativeShellEditController(hwnd win.HWND) *windowsNativeShellController {
	if hwnd == 0 {
		return nil
	}
	if value, ok := nativeShellEditControllers.Load(uintptr(hwnd)); ok {
		controller, _ := value.(*windowsNativeShellController)
		return controller
	}
	return nil
}

func (c *windowsNativeShellController) handleUserEditChange() {
	c.mu.RLock()
	if c.editControl == 0 || c.suppressTextChange {
		c.mu.RUnlock()
		return
	}
	editControl := c.editControl
	handler := c.textChangeHandler
	state := c.textInputState
	c.mu.RUnlock()

	state.QueryBox.Text = readWindowText(editControl)
	selection := uint32(win.SendMessage(editControl, win.EM_GETSEL, 0, 0))
	state.SelectionStart = int(win.LOWORD(selection))
	state.SelectionEnd = int(win.HIWORD(selection))

	c.mu.Lock()
	c.textInputState = state
	c.mu.Unlock()

	if handler != nil {
		handler(context.Background(), state)
	}
}

func (c *windowsNativeShellController) handleSelectionNavigation(delta int) {
	c.mu.RLock()
	handler := c.navigationHandler
	c.mu.RUnlock()

	if handler != nil {
		handler(context.Background(), delta)
	}
}

func (c *windowsNativeShellController) handleSubmit() {
	c.mu.RLock()
	handler := c.submitHandler
	c.mu.RUnlock()

	if handler != nil {
		handler(context.Background())
	}
}

func resolveNativeShellOrigin(showContext common.ShowContext) (int, int) {
	if showContext.WindowPosition != nil {
		return showContext.WindowPosition.X, showContext.WindowPosition.Y
	}

	width := showContext.WindowWidth
	if width <= 0 {
		width = defaultShellWidth
	}
	screenWidth := int(win.GetSystemMetrics(win.SM_CXSCREEN))
	return (screenWidth - width) / 2, int(shellWindowTopOffset)
}

func applyNativeShellAppearance(hwnd win.HWND, appearance WindowAppearance, theme launchertheme.PaintTheme) {
	dark := int32(1)
	procDwmSetWindowAttribute.Call(uintptr(hwnd), uintptr(dwMWAUseImmersiveDarkMode), uintptr(unsafe.Pointer(&dark)), unsafe.Sizeof(dark))

	if appearance.RoundedCorners {
		corner := int32(dwMWCPRound)
		procDwmSetWindowAttribute.Call(uintptr(hwnd), uintptr(dwMWAWindowCornerPreference), uintptr(unsafe.Pointer(&corner)), unsafe.Sizeof(corner))
	}

	if appearance.Acrylic {
		accentColor := accentGradientColor(resolveThemeColor(theme.Window.BackgroundColor, launchertheme.RGBAColor{R: 35, G: 41, B: 51, A: 191}))
		accentEnabled := tryEnableAccent(hwnd, accentEnableAcrylicBlurBehind, accentColor, 2)
		if !accentEnabled {
			accentEnabled = tryEnableAccent(hwnd, accentEnableHostBackdrop, accentColor, 0)
		}

		if accentEnabled {
			m := margins{}
			procDwmExtendFrameIntoClientArea.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&m)))

			backdrop := int32(dwMSBTNone)
			procDwmSetWindowAttribute.Call(uintptr(hwnd), uintptr(dwMWASystemBackdropType), uintptr(unsafe.Pointer(&backdrop)), unsafe.Sizeof(backdrop))
			return
		}

		backdrop := int32(dwMSBTTransientWindow)
		procDwmSetWindowAttribute.Call(uintptr(hwnd), uintptr(dwMWASystemBackdropType), uintptr(unsafe.Pointer(&backdrop)), unsafe.Sizeof(backdrop))
		m := margins{Left: -1}
		procDwmExtendFrameIntoClientArea.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&m)))
	}
}

func createSolidBrush(color win.COLORREF) win.HBRUSH {
	brush, _, _ := procCreateSolidBrush.Call(uintptr(color))
	return win.HBRUSH(brush)
}

func deleteGDIObject(handle uintptr) {
	if handle == 0 {
		return
	}
	procDeleteObject.Call(handle)
}

func drawText(hdc win.HDC, text string, rect *win.RECT, format uint32) {
	if rect == nil {
		return
	}
	utf16 := syscall.StringToUTF16(text)
	procDrawTextW.Call(
		uintptr(hdc),
		uintptr(unsafe.Pointer(&utf16[0])),
		uintptr(len(utf16)-1),
		uintptr(unsafe.Pointer(rect)),
		uintptr(format),
	)
}

func drawTextWithFont(hdc win.HDC, font win.HFONT, text string, rect *win.RECT, format uint32) {
	if rect == nil {
		return
	}

	var oldFont win.HGDIOBJ
	if font != 0 {
		oldFont = win.SelectObject(hdc, win.HGDIOBJ(font))
		defer win.SelectObject(hdc, oldFont)
	}

	drawText(hdc, text, rect, format)
}

func createUIFont(size int, weight int32) win.HFONT {
	dpi := int32(96)
	if hdc := win.GetDC(0); hdc != 0 {
		dpi = win.GetDeviceCaps(hdc, win.LOGPIXELSY)
		win.ReleaseDC(0, hdc)
	}

	height := -int32(math.Round(float64(size) * float64(dpi) / 72.0))
	family := syscall.StringToUTF16Ptr(defaultUIFontFamily)
	handle, _, _ := procCreateFontW.Call(
		uintptr(height),
		0,
		0,
		0,
		uintptr(weight),
		0,
		0,
		0,
		uintptr(win.DEFAULT_CHARSET),
		uintptr(win.OUT_DEFAULT_PRECIS),
		uintptr(win.CLIP_DEFAULT_PRECIS),
		uintptr(win.CLEARTYPE_QUALITY),
		uintptr(win.DEFAULT_PITCH|win.FF_DONTCARE),
		uintptr(unsafe.Pointer(family)),
	)
	return win.HFONT(handle)
}

func beginBufferedPaint(paintHDC win.HDC, rect *win.RECT) (win.HDC, uintptr) {
	if paintHDC == 0 || rect == nil {
		return paintHDC, 0
	}

	var bufferedHDC win.HDC
	buffer, _, _ := procBeginBufferedPaint.Call(
		uintptr(paintHDC),
		uintptr(unsafe.Pointer(rect)),
		uintptr(bpbfTopDownDIB),
		0,
		uintptr(unsafe.Pointer(&bufferedHDC)),
	)
	if buffer == 0 || bufferedHDC == 0 {
		return paintHDC, 0
	}

	procBufferedPaintClear.Call(buffer, uintptr(unsafe.Pointer(rect)))
	return bufferedHDC, buffer
}

func endBufferedPaint(buffer uintptr) {
	if buffer == 0 {
		return
	}
	procEndBufferedPaint.Call(buffer, 1)
}

func tryEnableAccent(hwnd win.HWND, state uint32, gradientColor uint32, accentFlags uint32) bool {
	if procSetWindowCompositionAttr.Find() != nil {
		return false
	}

	policy := accentPolicy{
		AccentState:   state,
		AccentFlags:   accentFlags,
		GradientColor: gradientColor,
	}
	data := windowCompositionAttribData{
		Attrib: wcaAccentPolicy,
		Data:   unsafe.Pointer(&policy),
		Size:   unsafe.Sizeof(policy),
	}
	ret, _, _ := procSetWindowCompositionAttr.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&data)))
	return ret != 0
}

func resolveThemeColor(value string, fallback launchertheme.RGBAColor) launchertheme.RGBAColor {
	if parsed, ok := launchertheme.ParseColor(value); ok {
		return parsed
	}
	return fallback
}

func resolveQueryBoxSolidColor(theme launchertheme.PaintTheme) launchertheme.RGBAColor {
	windowColor := resolveThemeColor(theme.Window.BackgroundColor, launchertheme.RGBAColor{R: 35, G: 41, B: 51, A: 191})
	queryColor := resolveThemeColor(theme.QueryBox.BackgroundColor, launchertheme.RGBAColor{R: 49, G: 56, B: 68, A: 77})
	return queryColor.CompositeOver(launchertheme.RGBAColor{
		R: windowColor.R,
		G: windowColor.G,
		B: windowColor.B,
		A: 255,
	})
}

func resolveResultActiveColor(theme launchertheme.PaintTheme) launchertheme.RGBAColor {
	windowColor := resolveThemeColor(theme.Window.BackgroundColor, launchertheme.RGBAColor{R: 35, G: 41, B: 51, A: 191})
	activeColor := resolveThemeColor(theme.Results.ActiveBackgroundColor, launchertheme.RGBAColor{R: 0, G: 168, B: 142, A: 179})
	return activeColor.CompositeOver(launchertheme.RGBAColor{
		R: windowColor.R,
		G: windowColor.G,
		B: windowColor.B,
		A: 255,
	})
}

func colorRefFromRGBA(color launchertheme.RGBAColor) win.COLORREF {
	return win.COLORREF(color.R) | (win.COLORREF(color.G) << 8) | (win.COLORREF(color.B) << 16)
}

func drawRoundedQueryBox(hdc win.HDC, rect win.RECT, color win.COLORREF, radius int) {
	if rect.Right <= rect.Left || rect.Bottom <= rect.Top {
		return
	}

	brush := createSolidBrush(color)
	if brush == 0 {
		return
	}
	defer deleteGDIObject(uintptr(brush))

	oldBrush := win.SelectObject(hdc, win.HGDIOBJ(brush))
	defer win.SelectObject(hdc, oldBrush)

	oldPen := win.SelectObject(hdc, win.GetStockObject(win.NULL_PEN))
	defer win.SelectObject(hdc, oldPen)

	if radius <= 0 {
		radius = 8
	}

	win.RoundRect(hdc, rect.Left, rect.Top, rect.Right, rect.Bottom, int32(radius*2), int32(radius*2))
}

func drawResultList(hdc win.HDC, state ResultListState, titleColor win.COLORREF, subtitleColor win.COLORREF, activeBackgroundColor win.COLORREF, activeTitleColor win.COLORREF, activeSubtitleColor win.COLORREF, radius int, titleFont win.HFONT, subtitleFont win.HFONT) {
	frame := state.Frame
	if frame.IsEmpty() {
		return
	}

	items := state.Items
	selectedIndex := state.SelectedIndex
	itemHeight := state.ItemHeight()
	currentY := frame.Y

	for index, item := range items {
		if currentY+itemHeight > frame.Y+frame.Height {
			return
		}

		itemRect := win.RECT{
			Left:   int32(frame.X),
			Top:    int32(currentY),
			Right:  int32(frame.X + frame.Width),
			Bottom: int32(currentY + itemHeight),
		}

		if index == selectedIndex {
			drawRoundedQueryBox(hdc, itemRect, activeBackgroundColor, radius)
		}

		titleRect := win.RECT{
			Left:   itemRect.Left + 14,
			Top:    itemRect.Top + 8,
			Right:  itemRect.Right - 14,
			Bottom: itemRect.Top + 8 + defaultResultTitleHeight,
		}
		subtitleRect := win.RECT{
			Left:   itemRect.Left + 14,
			Top:    titleRect.Bottom - 2,
			Right:  itemRect.Right - 14,
			Bottom: titleRect.Bottom - 2 + defaultResultSubtitleHeight,
		}

		if item.IsGroup {
			win.SetTextColor(hdc, subtitleColor)
			drawTextWithFont(hdc, subtitleFont, item.Title, &titleRect, win.DT_LEFT|win.DT_SINGLELINE|win.DT_VCENTER|win.DT_END_ELLIPSIS)
		} else {
			if index == selectedIndex {
				win.SetTextColor(hdc, activeTitleColor)
			} else {
				win.SetTextColor(hdc, titleColor)
			}
			drawTextWithFont(hdc, titleFont, item.Title, &titleRect, win.DT_LEFT|win.DT_SINGLELINE|win.DT_VCENTER|win.DT_END_ELLIPSIS)
			if item.Subtitle != "" {
				if index == selectedIndex {
					win.SetTextColor(hdc, activeSubtitleColor)
				} else {
					win.SetTextColor(hdc, subtitleColor)
				}
				drawTextWithFont(hdc, subtitleFont, item.Subtitle, &subtitleRect, win.DT_LEFT|win.DT_SINGLELINE|win.DT_VCENTER|win.DT_END_ELLIPSIS)
			}
		}

		currentY += itemHeight + defaultResultItemSpacingY
	}
}

func drawVerticalSplitLine(hdc win.HDC, x int, y int, height int, color win.COLORREF) {
	if height <= 0 {
		return
	}
	brush := createSolidBrush(color)
	if brush == 0 {
		return
	}
	defer deleteGDIObject(uintptr(brush))

	rect := win.RECT{
		Left:   int32(x),
		Top:    int32(y),
		Right:  int32(x + 1),
		Bottom: int32(y + height),
	}
	procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&rect)), uintptr(brush))
}

func drawPreviewPane(hdc win.HDC, frame Rect, preview PreviewState, bodyColor win.COLORREF, propertyTitleColor win.COLORREF, propertyContentColor win.COLORREF, titleFont win.HFONT, bodyFont win.HFONT) {
	if frame.IsEmpty() {
		return
	}

	titleRect := win.RECT{
		Left:   int32(frame.X + defaultPreviewPaddingX),
		Top:    int32(frame.Y + defaultPreviewPaddingY),
		Right:  int32(frame.X + frame.Width - defaultPreviewPaddingX),
		Bottom: int32(frame.Y + defaultPreviewPaddingY + defaultPreviewTitleHeight),
	}
	win.SetTextColor(hdc, bodyColor)
	drawTextWithFont(hdc, titleFont, preview.Title, &titleRect, win.DT_LEFT|win.DT_SINGLELINE|win.DT_VCENTER|win.DT_END_ELLIPSIS)

	bodyRect := win.RECT{
		Left:   titleRect.Left,
		Top:    titleRect.Bottom + 8,
		Right:  titleRect.Right,
		Bottom: int32(frame.Y + frame.Height - defaultPreviewPaddingY),
	}
	propertyBlockTop := bodyRect.Top
	if preview.Body.Content != "" {
		if preview.Body.Kind == PreviewKindUnsupported {
			win.SetTextColor(hdc, propertyTitleColor)
		} else {
			win.SetTextColor(hdc, bodyColor)
		}
		drawTextWithFont(hdc, bodyFont, preview.Body.Content, &bodyRect, win.DT_LEFT|win.DT_TOP|win.DT_WORDBREAK|win.DT_EDITCONTROL)
		propertyBlockTop = bodyRect.Top + 84
	}

	for _, property := range preview.Body.Properties {
		if propertyBlockTop >= bodyRect.Bottom {
			break
		}

		titleRect := win.RECT{
			Left:   bodyRect.Left,
			Top:    propertyBlockTop,
			Right:  bodyRect.Right,
			Bottom: propertyBlockTop + 18,
		}
		contentRect := win.RECT{
			Left:   bodyRect.Left,
			Top:    titleRect.Bottom + 2,
			Right:  bodyRect.Right,
			Bottom: titleRect.Bottom + 40,
		}

		win.SetTextColor(hdc, propertyTitleColor)
		drawTextWithFont(hdc, bodyFont, property.Title, &titleRect, win.DT_LEFT|win.DT_SINGLELINE|win.DT_VCENTER|win.DT_END_ELLIPSIS)
		win.SetTextColor(hdc, propertyContentColor)
		drawTextWithFont(hdc, bodyFont, property.Content, &contentRect, win.DT_LEFT|win.DT_TOP|win.DT_WORDBREAK|win.DT_EDITCONTROL)
		propertyBlockTop = contentRect.Bottom + 10
	}
}

func makeLPARAM(low, high int) uintptr {
	return uintptr(uint32(uint16(low)) | (uint32(uint16(high)) << 16))
}

func accentGradientColor(color launchertheme.RGBAColor) uint32 {
	return uint32(color.A)<<24 | uint32(color.B)<<16 | uint32(color.G)<<8 | uint32(color.R)
}

func readWindowText(hwnd win.HWND) string {
	if hwnd == 0 {
		return ""
	}

	length := int(win.SendMessage(hwnd, win.WM_GETTEXTLENGTH, 0, 0))
	if length == 0 {
		return ""
	}

	buffer := make([]uint16, length+1)
	win.SendMessage(hwnd, win.WM_GETTEXT, uintptr(len(buffer)), uintptr(unsafe.Pointer(&buffer[0])))
	return syscall.UTF16ToString(buffer)
}
