//go:build windows

package window

import (
	"testing"

	"github.com/lxn/win"
)

func TestWindowsTopmostShowFlagsKeepNonactivatingTooltipsPassive(t *testing.T) {
	activating := windowsTopmostShowFlags(false)
	if activating&win.SWP_NOMOVE == 0 || activating&win.SWP_NOSIZE == 0 {
		t.Fatalf("activating topmost flags = %#x, want SWP_NOMOVE|SWP_NOSIZE", activating)
	}
	if activating&win.SWP_NOACTIVATE != 0 {
		t.Fatalf("activating topmost flags = %#x, must still allow focus for preview overlays", activating)
	}

	nonactivating := windowsTopmostShowFlags(true)
	if nonactivating&win.SWP_NOACTIVATE == 0 {
		t.Fatalf("nonactivating topmost flags = %#x, want SWP_NOACTIVATE so tooltips cannot steal launcher focus", nonactivating)
	}
}

func TestIsNonactivatingNativeWindowIgnoresOwnedTooltips(t *testing.T) {
	if isNonactivatingNativeWindow(0) {
		t.Fatal("a missing HWND is not a Wox tooltip")
	}

	tooltip := win.HWND(0x1001)
	nativeWindows.Store(uintptr(tooltip), &platformWindow{options: WindowOptions{Nonactivating: true}})
	t.Cleanup(func() { nativeWindows.Delete(uintptr(tooltip)) })
	if !isNonactivatingNativeWindow(tooltip) {
		t.Fatal("tooltip overlays must stay inside the hide-on-blur ignore set")
	}

	preview := win.HWND(0x1002)
	nativeWindows.Store(uintptr(preview), &platformWindow{options: WindowOptions{}})
	t.Cleanup(func() { nativeWindows.Delete(uintptr(preview)) })
	if isNonactivatingNativeWindow(preview) {
		t.Fatal("focus-taking overlays must still count as a real focus loss")
	}

	if isNonactivatingNativeWindow(win.HWND(0x1003)) {
		t.Fatal("foreign windows must still count as a real focus loss")
	}
}

func TestHandleBlurIgnoresOwnedNativeFileDialog(t *testing.T) {
	hidden := false
	window := &platformWindow{
		options: WindowOptions{
			HideOnBlur: true,
			OnFocus: func(event FocusEvent) {
				if !event.Active {
					hidden = true
				}
			},
		},
		nativeDialogActive: true,
		focus: focusRuntime{
			visible:             true,
			activationConfirmed: true,
			active:              true,
		},
	}

	window.handleBlur(win.HWND(0x2001))
	if hidden {
		t.Fatal("Wox-owned file pickers must not count as hide-on-blur focus loss")
	}
	if !window.focus.visible || !window.focus.active {
		t.Fatal("launcher must stay visible while a Wox-owned file picker is open")
	}
}

func TestHandleBlurIgnoresResultFileDrag(t *testing.T) {
	hidden := false
	window := &platformWindow{
		options: WindowOptions{
			HideOnBlur: true,
			OnFocus: func(event FocusEvent) {
				if !event.Active {
					hidden = true
				}
			},
		},
		fileDragActive: true,
		focus: focusRuntime{
			visible:             true,
			activationConfirmed: true,
			active:              true,
		},
	}

	window.handleBlur(win.HWND(0x2001))
	if hidden {
		t.Fatal("result file drag must not count as hide-on-blur focus loss")
	}
	if !window.focus.visible || !window.focus.active {
		t.Fatal("launcher must stay visible while a result file drag is active")
	}
}

func TestWindowsRestoreFromMinimizeReclaimsKeyboardFocus(t *testing.T) {
	err := Run(func() error {
		var committed string
		window, err := Open(WindowOptions{
			Title:     "Wox minimize restore focus test",
			Size:      Size{Width: 320, Height: 240},
			Role:      WindowRoleApplication,
			Resizable: true,
			OnTextInput: func(event TextInputEvent) {
				if event.Kind == TextInputCommit {
					committed += event.Text
				}
			},
		})
		if err != nil {
			return err
		}
		defer window.Close()
		if err := window.SetTextInputState(TextInputState{
			Enabled:    true,
			CursorRect: Rect{X: 20, Y: 20, Width: 2, Height: 18},
		}); err != nil {
			return err
		}
		if _, err := window.Show(); err != nil {
			return err
		}
		if err := window.Minimize(); err != nil {
			return err
		}
		if !win.IsIconic(window.native.hwnd) {
			t.Fatal("window was not minimized")
		}

		// Taskbar restore dispatches WM_SYSCOMMAND SC_RESTORE without a client click.
		win.SendMessage(window.native.hwnd, win.WM_SYSCOMMAND, 0xF120, 0)
		if win.IsIconic(window.native.hwnd) {
			t.Fatal("window stayed minimized after SC_RESTORE")
		}
		if got := win.GetFocus(); got != window.native.hwnd {
			t.Fatalf("GetFocus() = %#x after restore, want %#x", got, window.native.hwnd)
		}
		win.SendMessage(window.native.hwnd, win.WM_CHAR, 'a', 0)
		if committed != "a" {
			t.Fatalf("committed text = %q after restore, want %q", committed, "a")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWindowsMinimizeSkipsIconicFramePaint(t *testing.T) {
	err := Run(func() error {
		var iconicFrameHeight float32
		window, err := Open(WindowOptions{
			Title:     "Wox minimize iconic paint test",
			Size:      Size{Width: 320, Height: 240},
			Role:      WindowRoleApplication,
			Resizable: true,
			OnFrame: func(_ *DisplayList, frame FrameInfo) {
				if frame.Size.Height < 100 {
					iconicFrameHeight = frame.Size.Height
				}
			},
		})
		if err != nil {
			return err
		}
		defer window.Close()
		if _, err := window.Show(); err != nil {
			return err
		}
		if err := window.Minimize(); err != nil {
			return err
		}
		win.RedrawWindow(window.native.hwnd, nil, 0, win.RDW_INVALIDATE|win.RDW_UPDATENOW)
		if iconicFrameHeight != 0 {
			t.Fatalf("minimized window rendered frame with iconic height %.1f", iconicFrameHeight)
		}
		win.SendMessage(window.native.hwnd, win.WM_SYSCOMMAND, 0xF120, 0)
		if win.IsIconic(window.native.hwnd) {
			t.Fatal("window stayed minimized after SC_RESTORE")
		}
		if iconicFrameHeight != 0 {
			t.Fatalf("restore triggered iconic frame height %.1f", iconicFrameHeight)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWindowsActivateWhileMinimizedDoesNotTakeFocus(t *testing.T) {
	err := Run(func() error {
		window, err := Open(WindowOptions{
			Title:     "Wox minimized activate test",
			Size:      Size{Width: 320, Height: 240},
			Role:      WindowRoleApplication,
			Resizable: true,
		})
		if err != nil {
			return err
		}
		defer window.Close()
		if _, err := window.Show(); err != nil {
			return err
		}
		if err := window.Minimize(); err != nil {
			return err
		}
		if !win.IsIconic(window.native.hwnd) {
			t.Fatal("window was not minimized")
		}
		if window.native.focus.active {
			t.Fatal("window stayed active after minimizing")
		}

		// HIWORD(wParam) != 0 indicates activation while minimized.
		win.SendMessage(window.native.hwnd, win.WM_ACTIVATE, uintptr(win.WA_ACTIVE)|(1<<16), 0)
		if got := win.GetFocus(); got == window.native.hwnd {
			t.Fatalf("minimized window unexpectedly claimed GetFocus() = %#x", got)
		}
		if window.native.focus.active {
			t.Fatal("minimized window must not mark itself active")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
