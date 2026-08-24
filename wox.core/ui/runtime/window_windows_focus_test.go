//go:build windows

package woxui

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
