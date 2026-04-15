//go:build windows

package platform

import (
	"context"
	"fmt"
	"sync"
	"syscall"
	"time"
	"unsafe"
	"wox/common"
	launchertheme "wox/launcher/theme"

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
	procCreateSolidBrush             = modGdi32.NewProc("CreateSolidBrush")
	procDeleteObject                 = modGdi32.NewProc("DeleteObject")
	modUser32                        = syscall.NewLazyDLL("user32.dll")
	procDrawTextW                    = modUser32.NewProc("DrawTextW")
	procSetWindowCompositionAttr     = modUser32.NewProc("SetWindowCompositionAttribute")

	nativeShellControllers     sync.Map
	nativeShellEditControllers sync.Map
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
	ecLeftMargin                  = 0x1
	ecRightMargin                 = 0x2
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

	queryBrush win.HBRUSH
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
	_ = ctx
	return h.controller.show(request)
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
	return HostDebugSnapshot{
		Visible:            h.controller.isVisible(),
		NativeWindowHandle: h.controller.nativeWindowHandle(),
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

func (c *windowsNativeShellController) show(request ShowRequest) error {
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
	c.mu.Lock()
	c.windowHandle = window
	c.editControl = edit
	c.editWndProc = editWndProc
	c.refreshBrushesLocked()
	c.mu.Unlock()

	applyNativeShellAppearance(window, c.appearance, launchertheme.DefaultPaintTheme())
	return nil
}

func (c *windowsNativeShellController) destroyNativeControlsLocked() {
	if c.queryBrush != 0 {
		deleteGDIObject(uintptr(c.queryBrush))
		c.queryBrush = 0
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

	win.SetWindowPos(
		c.windowHandle,
		win.HWND_TOPMOST,
		int32(x),
		int32(y),
		int32(width),
		int32(defaultShellHeight),
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
	hdc := win.BeginPaint(hwnd, &ps)
	defer win.EndPaint(hwnd, &ps)

	var rect win.RECT
	win.GetClientRect(hwnd, &rect)

	c.mu.RLock()
	title := "Wox Native Launcher"
	if c.showRequest.ShowContext.ShowSource != "" {
		title = fmt.Sprintf("%s (%s)", title, c.showRequest.ShowContext.ShowSource)
	}
	queryFrame := c.textInputState.QueryBox.Frame
	queryVisible := c.textInputState.QueryBox.Visible
	absoluteFrame := c.queryFrameAbsolute
	queryRadius := c.showRequest.Theme.QueryBox.BorderRadius
	textColor := colorRefFromRGBA(resolveThemeColor(c.showRequest.Theme.QueryBox.ForegroundColor, launchertheme.RGBAColor{R: 0xE2, G: 0xE8, B: 0xF0, A: 0xFF}))
	subtitleColor := colorRefFromRGBA(resolveThemeColor(c.showRequest.Theme.QueryBox.ForegroundColor, launchertheme.RGBAColor{R: 0xE2, G: 0xE8, B: 0xF0, A: 0xFF}).WithAlphaScale(0.72))
	resultFrame := c.showRequest.Results.Frame
	resultVisible := c.showRequest.Results.Visible
	resultItems := append([]ResultListItem(nil), c.showRequest.Results.Items...)
	selectedIndex := c.showRequest.Results.SelectedIndex
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
	win.SetTextColor(hdc, textColor)

	titleRect := win.RECT{
		Left:   defaultNativeShellPaddingX,
		Top:    defaultNativeShellTitleY,
		Right:  rect.Right - defaultNativeShellPaddingX,
		Bottom: defaultNativeShellTitleY + defaultNativeShellTitleHeight,
	}
	drawText(hdc, title, &titleRect, win.DT_LEFT|win.DT_SINGLELINE|win.DT_VCENTER)

	if resultVisible && !resultFrame.IsEmpty() {
		if absoluteFrame {
			var windowRect win.RECT
			if win.GetWindowRect(hwnd, &windowRect) {
				resultFrame.X -= int(windowRect.Left)
				resultFrame.Y -= int(windowRect.Top)
			}
		}

		drawResultList(hdc, resultFrame, resultItems, selectedIndex, textColor, subtitleColor)
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

func drawResultList(hdc win.HDC, frame Rect, items []ResultListItem, selectedIndex int, titleColor win.COLORREF, subtitleColor win.COLORREF) {
	if frame.IsEmpty() {
		return
	}

	itemHeight := defaultResultListItemHeight
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
			drawRoundedQueryBox(hdc, itemRect, colorRefFromRGBA(launchertheme.RGBAColor{R: 255, G: 255, B: 255, A: 20}), 10)
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
			drawText(hdc, item.Title, &titleRect, win.DT_LEFT|win.DT_SINGLELINE|win.DT_VCENTER|win.DT_END_ELLIPSIS)
		} else {
			win.SetTextColor(hdc, titleColor)
			drawText(hdc, item.Title, &titleRect, win.DT_LEFT|win.DT_SINGLELINE|win.DT_VCENTER|win.DT_END_ELLIPSIS)
			if item.Subtitle != "" {
				win.SetTextColor(hdc, subtitleColor)
				drawText(hdc, item.Subtitle, &subtitleRect, win.DT_LEFT|win.DT_SINGLELINE|win.DT_VCENTER|win.DT_END_ELLIPSIS)
			}
		}

		currentY += itemHeight + defaultResultItemSpacingY
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
