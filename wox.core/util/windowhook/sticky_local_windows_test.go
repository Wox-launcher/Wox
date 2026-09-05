//go:build windows && cgo

package windowhook

import (
	"fmt"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"unsafe"
	woxui "wox/ui/runtime"
)

// TestLocalStickyMovesSynchronously runs the real subclass on Wox's UI thread.
// Position assertions run before pumping messages, so polling cannot pass this test.
func TestLocalStickyMovesSynchronously(t *testing.T) {
	if os.Getenv("WOX_WINDOWS_STICKY_INTEGRATION") != "1" {
		t.Skip("set WOX_WINDOWS_STICKY_INTEGRATION=1 for native sticky tests")
	}
	dllPath := filepath.Join(t.TempDir(), "WoxWindowHook.dll")
	if output, err := exec.Command("gcc", "-shared", "-O2", "-static-libgcc", "-o", dllPath, "hook/window_hook.c", "-lole32", "-lshell32", "-luuid", "-lcomctl32").CombinedOutput(); err != nil {
		t.Fatalf("build hook: %v: %s", err, output)
	}
	err := woxui.Run(func() error {
		target, err := woxui.Open(woxui.WindowOptions{Title: "Wox sticky target"})
		if err != nil {
			return err
		}
		defer target.Close()
		overlay, err := woxui.Open(woxui.WindowOptions{Title: "Wox sticky overlay", Nonactivating: true})
		if err != nil {
			return err
		}
		defer overlay.Close()
		if _, err = target.Show(); err != nil {
			return err
		}
		if _, err = overlay.Show(); err != nil {
			return err
		}
		targetHWND, overlayHWND := target.WindowsHandle(), overlay.WindowsHandle()
		flags := uint32(win.SWP_NOSIZE | win.SWP_NOZORDER | win.SWP_NOACTIVATE)
		win.SetWindowPos(win.HWND(targetHWND), 0, 200, 200, 0, 0, flags)
		win.SetWindowPos(win.HWND(overlayHWND), 0, 220, 260, 0, 0, flags)
		dll, err := windows.LoadDLL(dllPath)
		if err != nil {
			return err
		}
		attach, err := dll.FindProc("WoxWindowHookAttachSticky")
		if err != nil {
			dll.Release()
			return err
		}
		var diagnostic navigationDiagnostic
		handle, _, _ := attach.Call(targetHWND, uintptr(os.Getpid()), overlayHWND, uintptr(unsafe.Pointer(&diagnostic)))
		if handle == 0 {
			dll.Release()
			return fmt.Errorf("direct attach failed: %+v", diagnostic)
		}
		hook := &StickyHook{dll: dll, handle: handle, target: targetHWND}
		defer hook.Detach()
		hook.PublishStickyOffset(overlayHWND)
		var initialTarget, initialOverlay win.RECT
		win.GetWindowRect(win.HWND(targetHWND), &initialTarget)
		win.GetWindowRect(win.HWND(overlayHWND), &initialOverlay)
		// Native boundary coordinates and offsets are physical pixels, including
		// negative desktop origins. No logical/DIP conversion belongs in translation.
		dx, dy := initialOverlay.Left-initialTarget.Left, initialOverlay.Top-initialTarget.Top
		for _, point := range [][2]int32{{280, 240}, {-100, -80}, {300, 260}} {
			win.SetWindowPos(win.HWND(targetHWND), 0, point[0], point[1], 0, 0, flags)
			var targetRect, overlayRect win.RECT
			win.GetWindowRect(win.HWND(targetHWND), &targetRect)
			win.GetWindowRect(win.HWND(overlayHWND), &overlayRect)
			if overlayRect.Left-targetRect.Left != dx || overlayRect.Top-targetRect.Top != dy {
				return fmt.Errorf("overlay lagged at %v: target=%+v overlay=%+v offset=(%d,%d)", point, targetRect, overlayRect, dx, dy)
			}
		}
		if !hook.Detach() {
			return fmt.Errorf("direct detach failed")
		}
		var before, after win.RECT
		win.GetWindowRect(win.HWND(overlayHWND), &before)
		win.SetWindowPos(win.HWND(targetHWND), 0, 400, 300, 0, 0, flags)
		win.GetWindowRect(win.HWND(overlayHWND), &after)
		if before != after {
			return fmt.Errorf("detached overlay still follows target")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
