//go:build wox_ui_smoke && windows

package hotkey

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/lxn/win"
	utilwindow "wox/util/window"
)

const (
	fullscreenApplyMessage = win.WM_APP + 41
	fullscreenFocusTimerID = 1
	// fullscreenFocusBudget covers the activation Windows posts after a fullscreen
	// popup shrinks and uncovers the window underneath.
	fullscreenFocusBudget = 4 * time.Second
)

var (
	fullscreenUser32         = syscall.NewLazyDLL("user32.dll")
	allowSetForegroundWindow = fullscreenUser32.NewProc("AllowSetForegroundWindow")
	getWindowTextW           = fullscreenUser32.NewProc("GetWindowTextW")
)

// fullscreenPlacement is processed on the target window thread.
type fullscreenPlacement struct {
	fullscreen bool
	done       chan error
}

func requireFullscreenHotkeyRuntime(t *testing.T) { t.Helper() }

// newFullscreenHotkeyTarget owns a native window in the test process, separate
// from Wox. Placement and foreground retries run on its message thread.
// Windows drops SetForegroundWindow once another app is in front, which is what
// happens when a fullscreen popup shrinks and uncovers the window below it.
func newFullscreenHotkeyTarget(t *testing.T) func(bool) error {
	t.Helper()
	ready := make(chan win.HWND, 1)
	closed := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(closed)
		setDPI := fullscreenUser32.NewProc("SetThreadDpiAwarenessContext")
		previous, _, _ := setDPI.Call(^uintptr(3))
		defer setDPI.Call(previous)
		isWindow := fullscreenUser32.NewProc("IsWindow")
		class, _ := syscall.UTF16PtrFromString("STATIC")
		title, _ := syscall.UTF16PtrFromString("Wox fullscreen hotkey smoke")
		hwnd := win.CreateWindowEx(0, class, title, win.WS_OVERLAPPEDWINDOW, 100, 100, 640, 480, 0, 0, 0, nil)
		ready <- hwnd
		if hwnd == 0 {
			return
		}

		var pending *fullscreenPlacement
		var focusDeadline time.Time
		// held is set only when a tick finds the target already in front. The next
		// tick must see it still in front, so a late activation from the uncovered
		// window cannot pass the wait.
		var held bool
		finish := func(err error) {
			if pending == nil {
				return
			}
			win.KillTimer(hwnd, fullscreenFocusTimerID)
			request := pending
			pending = nil
			request.done <- err
		}
		tryFocus := func() {
			if pending == nil {
				return
			}
			if win.GetForegroundWindow() == hwnd {
				if held {
					finish(nil)
					return
				}
				held = true
				return
			}
			held = false
			if time.Now().After(focusDeadline) {
				finish(fmt.Errorf("target did not gain foreground focus: %s", describeForegroundWindow()))
				return
			}
			claimFullscreenForeground(hwnd)
		}

		var message win.MSG
		for win.GetMessage(&message, 0, 0, 0) > 0 {
			switch message.Message {
			case fullscreenApplyMessage:
				request := (*fullscreenPlacement)(unsafe.Pointer(message.LParam))
				if pending != nil {
					finish(errors.New("fullscreen target placement overlapped"))
				}
				pending = request
				held = false
				focusDeadline = time.Now().Add(fullscreenFocusBudget)
				if err := placeFullscreenTarget(hwnd, request.fullscreen); err != nil {
					finish(err)
					continue
				}
				if win.SetTimer(hwnd, fullscreenFocusTimerID, 25, 0) == 0 {
					finish(errors.New("start fullscreen focus timer"))
					continue
				}
				tryFocus()
				continue
			case win.WM_TIMER:
				if message.WParam == fullscreenFocusTimerID {
					tryFocus()
					continue
				}
			case win.WM_CLOSE:
				finish(errors.New("fullscreen target window closed"))
			}
			win.TranslateMessage(&message)
			win.DispatchMessage(&message)
			// DestroyWindow does not post another message, so the closed check has to
			// run in this iteration. Otherwise GetMessage waits forever.
			if exists, _, _ := isWindow.Call(uintptr(hwnd)); exists == 0 {
				finish(errors.New("fullscreen target window closed"))
				return
			}
		}
	}()
	hwnd := <-ready
	if hwnd == 0 {
		t.Fatal("create native fullscreen target")
	}
	t.Cleanup(func() {
		win.PostMessage(hwnd, win.WM_CLOSE, 0, 0)
		select {
		case <-closed:
		case <-time.After(5 * time.Second):
			t.Error("native fullscreen target did not close")
		}
	})
	return func(fullscreen bool) error {
		request := &fullscreenPlacement{fullscreen: fullscreen, done: make(chan error, 1)}
		if win.PostMessage(hwnd, fullscreenApplyMessage, 0, uintptr(unsafe.Pointer(request))) == 0 {
			return errors.New("post fullscreen target placement")
		}
		select {
		case err := <-request.done:
			runtime.KeepAlive(request)
			return err
		case <-time.After(fullscreenFocusBudget + 4*time.Second):
			runtime.KeepAlive(request)
			return errors.New("fullscreen target did not answer")
		}
	}
}

// placeFullscreenTarget sizes the window on its own thread.
// Fullscreen uses the monitor bounds. The restored window is a normal overlapped
// window. Both rects are physical pixels.
func placeFullscreenTarget(hwnd win.HWND, fullscreen bool) error {
	monitor := win.MONITORINFO{}
	monitor.CbSize = uint32(unsafe.Sizeof(monitor))
	if !win.GetMonitorInfo(win.MonitorFromWindow(hwnd, win.MONITOR_DEFAULTTONEAREST), &monitor) {
		return errors.New("read target monitor")
	}
	rect := monitor.RcMonitor
	style := uint32(win.WS_POPUP | win.WS_VISIBLE)
	if !fullscreen {
		style = win.WS_OVERLAPPEDWINDOW | win.WS_VISIBLE
		rect.Right = rect.Left + 640
		rect.Bottom = rect.Top + 480
	}
	win.SetWindowLong(hwnd, win.GWL_STYLE, int32(style))
	if !win.SetWindowPos(hwnd, win.HWND_TOP, rect.Left, rect.Top, rect.Right-rect.Left, rect.Bottom-rect.Top, win.SWP_FRAMECHANGED|win.SWP_SHOWWINDOW) {
		return errors.New("resize fullscreen target")
	}
	return nil
}

// claimFullscreenForeground makes hwnd the foreground window.
// The thread-input attachment matches the product activation path. No synthetic
// modifier is injected, so the following Ctrl+F12 chord stays unmodified.
func claimFullscreenForeground(hwnd win.HWND) bool {
	if win.GetForegroundWindow() == hwnd {
		return true
	}
	// TOPMOST can raise the window when SetForegroundWindow is locked. NOTOPMOST
	// clears that band before the call returns, so the fixture does not stay above Wox.
	win.SetWindowPos(hwnd, win.HWND_TOPMOST, 0, 0, 0, 0, win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOACTIVATE)
	win.SetWindowPos(hwnd, win.HWND_NOTOPMOST, 0, 0, 0, 0, win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_SHOWWINDOW)
	attachment := utilwindow.AttachThreadInput(uintptr(hwnd), utilwindow.QueuesToCurrentThread)
	win.SetForegroundWindow(hwnd)
	win.BringWindowToTop(hwnd)
	win.SetActiveWindow(hwnd)
	attachment.Close()
	if win.GetForegroundWindow() == hwnd {
		return true
	}
	allowSetForegroundWindow.Call(uintptr(^uint32(0)))
	win.SetForegroundWindow(hwnd)
	win.BringWindowToTop(hwnd)
	return win.GetForegroundWindow() == hwnd
}

// describeForegroundWindow names the window that actually owns the foreground.
func describeForegroundWindow() string {
	hwnd := win.GetForegroundWindow()
	if hwnd == 0 {
		return "none"
	}
	var title [512]uint16
	getWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&title[0])), uintptr(len(title)))
	var pid uint32
	win.GetWindowThreadProcessId(hwnd, &pid)
	return fmt.Sprintf("hwnd=%d pid=%d title=%q", hwnd, pid, syscall.UTF16ToString(title[:]))
}
