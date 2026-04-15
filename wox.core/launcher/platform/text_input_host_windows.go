//go:build windows

package platform

import (
	"context"
	"fmt"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/win"
)

type WindowsTextInputHost struct {
	mu      sync.RWMutex
	started bool
	focused bool
	state   TextInputState

	parentWindow win.HWND
	hostWindow   win.HWND
	editControl  win.HWND
	commands     chan func()
	threadDone   chan struct{}
}

func NewWindowsTextInputHost() *WindowsTextInputHost {
	return &WindowsTextInputHost{}
}

func (h *WindowsTextInputHost) Start(ctx context.Context) error {
	_ = ctx

	h.mu.Lock()
	if h.started {
		h.mu.Unlock()
		return nil
	}
	ready := make(chan error, 1)
	h.commands = make(chan func())
	h.threadDone = make(chan struct{})
	h.mu.Unlock()

	go h.runUIThread(ready)

	if err := <-ready; err != nil {
		h.mu.Lock()
		h.commands = nil
		h.threadDone = nil
		h.mu.Unlock()
		return err
	}

	h.mu.Lock()
	h.started = true
	h.mu.Unlock()
	return nil
}

func (h *WindowsTextInputHost) Stop(ctx context.Context) error {
	_ = ctx

	h.mu.Lock()
	if !h.started {
		h.mu.Unlock()
		return nil
	}
	threadDone := h.threadDone
	h.mu.Unlock()

	if err := h.call(func() {
		h.destroyNativeControls()
		if h.commands != nil {
			close(h.commands)
		}
	}); err != nil {
		return err
	}

	if threadDone != nil {
		<-threadDone
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.started = false
	h.focused = false
	h.state = TextInputState{}
	h.commands = nil
	h.threadDone = nil
	return nil
}

func (h *WindowsTextInputHost) UpdateState(ctx context.Context, state TextInputState) error {
	_ = ctx

	return h.call(func() {
		h.mu.Lock()
		h.state = state
		h.mu.Unlock()

		if h.editControl == 0 {
			return
		}

		win.SendMessage(h.editControl, win.WM_SETTEXT, 0, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(state.QueryBox.Text))))
		win.SendMessage(h.editControl, win.EM_SETSEL, uintptr(state.SelectionStart), uintptr(state.SelectionEnd))
		h.applyStateToNativeControls(state)
	})
}

func (h *WindowsTextInputHost) Focus(ctx context.Context) error {
	_ = ctx

	return h.call(func() {
		h.mu.Lock()
		h.focused = true
		state := h.state
		h.mu.Unlock()

		if state.QueryBox.Visible && !state.QueryBox.Frame.IsEmpty() && h.hostWindow != 0 {
			win.ShowWindow(h.hostWindow, win.SW_SHOW)
			if h.editControl != 0 {
				win.SetFocus(h.editControl)
			}
		}
	})
}

func (h *WindowsTextInputHost) Blur(ctx context.Context) error {
	_ = ctx

	return h.call(func() {
		h.mu.Lock()
		h.focused = false
		h.mu.Unlock()

		if h.hostWindow != 0 {
			win.ShowWindow(h.hostWindow, win.SW_HIDE)
		}
	})
}

func (h *WindowsTextInputHost) SetParentWindow(ctx context.Context, windowHandle uintptr) error {
	_ = ctx
	var attachErr error
	err := h.call(func() {
		parentWindow := win.HWND(windowHandle)
		if h.parentWindow == parentWindow {
			return
		}

		if err := h.recreateNativeControls(parentWindow); err != nil {
			attachErr = err
			return
		}

		h.parentWindow = parentWindow
		h.applyStateToNativeControls(h.state)
		if h.focused && h.editControl != 0 {
			win.SetFocus(h.editControl)
		}
	})
	if err != nil {
		return err
	}
	return attachErr
}

func NewDefaultTextInputHost() TextInputHost {
	return NewWindowsTextInputHost()
}

func (h *WindowsTextInputHost) DebugSnapshot(ctx context.Context) TextInputDebugSnapshot {
	_ = ctx

	h.mu.RLock()
	defer h.mu.RUnlock()

	return TextInputDebugSnapshot{
		ParentWindowHandle: uintptr(h.parentWindow),
		HostWindowHandle:   uintptr(h.hostWindow),
		EditControlHandle:  uintptr(h.editControl),
		Focused:            h.focused,
		HostVisible:        h.hostWindow != 0 && win.IsWindowVisible(h.hostWindow),
		EditVisible:        h.editControl != 0 && win.IsWindowVisible(h.editControl),
		Frame:              h.state.QueryBox.Frame,
	}
}

func (h *WindowsTextInputHost) runUIThread(ready chan<- error) {
	runtimeLockOSThread()
	defer runtimeUnlockOSThread()
	defer close(h.threadDone)

	if err := h.createNativeControls(0); err != nil {
		ready <- err
		return
	}
	ready <- nil

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case cmd, ok := <-h.commands:
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

func (h *WindowsTextInputHost) call(fn func()) error {
	h.mu.RLock()
	started := h.started
	commands := h.commands
	h.mu.RUnlock()

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
		return fmt.Errorf("text input host command timeout")
	}
}

func (h *WindowsTextInputHost) recreateNativeControls(parentWindow win.HWND) error {
	h.destroyNativeControls()
	return h.createNativeControls(parentWindow)
}

func (h *WindowsTextInputHost) createNativeControls(parentWindow win.HWND) error {
	hostClass := syscall.StringToUTF16Ptr("STATIC")
	hostTitle := syscall.StringToUTF16Ptr("WoxNativeTextInputHost")
	hostExStyle := uint32(win.WS_EX_TOOLWINDOW | win.WS_EX_TOPMOST)
	hostStyle := uint32(win.WS_POPUP)
	hostX := int32(-32000)
	hostY := int32(-32000)
	hostParent := win.HWND(0)
	if parentWindow != 0 {
		hostExStyle = 0
		hostStyle = win.WS_CHILD | win.WS_CLIPSIBLINGS | win.WS_CLIPCHILDREN
		hostX = 0
		hostY = 0
		hostParent = parentWindow
	}
	hostWindow := win.CreateWindowEx(
		hostExStyle,
		hostClass,
		hostTitle,
		hostStyle,
		hostX,
		hostY,
		1,
		1,
		hostParent,
		0,
		0,
		nil,
	)
	if hostWindow == 0 {
		return fmt.Errorf("failed to create Windows text input host window")
	}

	editClass := syscall.StringToUTF16Ptr("EDIT")
	editControl := win.CreateWindowEx(
		win.WS_EX_CLIENTEDGE,
		editClass,
		syscall.StringToUTF16Ptr(""),
		win.WS_CHILD|win.WS_TABSTOP|win.WS_VISIBLE|win.ES_LEFT|win.ES_AUTOHSCROLL,
		0,
		0,
		1,
		1,
		hostWindow,
		0,
		0,
		nil,
	)
	if editControl == 0 {
		win.DestroyWindow(hostWindow)
		return fmt.Errorf("failed to create Windows text input edit control")
	}

	h.mu.Lock()
	h.parentWindow = parentWindow
	h.hostWindow = hostWindow
	h.editControl = editControl
	h.mu.Unlock()

	return nil
}

func (h *WindowsTextInputHost) applyStateToNativeControls(state TextInputState) {
	if h.hostWindow == 0 || h.editControl == 0 {
		return
	}

	frame := state.QueryBox.Frame
	if !state.QueryBox.Visible || frame.IsEmpty() {
		win.ShowWindow(h.editControl, win.SW_HIDE)
		win.ShowWindow(h.hostWindow, win.SW_HIDE)
		return
	}

	if h.parentWindow != 0 {
		var parentRect win.RECT
		if win.GetWindowRect(h.parentWindow, &parentRect) {
			frame.X -= int(parentRect.Left)
			frame.Y -= int(parentRect.Top)
		}
	}

	win.SetWindowPos(
		h.hostWindow,
		0,
		int32(frame.X),
		int32(frame.Y),
		int32(frame.Width),
		int32(frame.Height),
		win.SWP_NOZORDER|win.SWP_NOACTIVATE,
	)
	win.SetWindowPos(
		h.editControl,
		0,
		0,
		0,
		int32(frame.Width),
		int32(frame.Height),
		win.SWP_NOZORDER|win.SWP_NOACTIVATE,
	)

	showCmd := win.SW_SHOWNOACTIVATE
	if state.QueryBox.HasFocus {
		showCmd = win.SW_SHOW
	}
	win.ShowWindow(h.editControl, int32(showCmd))
	win.ShowWindow(h.hostWindow, int32(showCmd))
}

func (h *WindowsTextInputHost) destroyNativeControls() {
	if h.editControl != 0 {
		win.DestroyWindow(h.editControl)
	}
	if h.hostWindow != 0 {
		win.DestroyWindow(h.hostWindow)
	}

	h.mu.Lock()
	h.parentWindow = 0
	h.hostWindow = 0
	h.editControl = 0
	h.mu.Unlock()
}
