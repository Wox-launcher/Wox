//go:build wox_ui_smoke && windows

package hotkey

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	enumWindows              = windows.NewLazySystemDLL("user32.dll").NewProc("EnumWindows")
	getForegroundWindow      = windows.NewLazySystemDLL("user32.dll").NewProc("GetForegroundWindow")
	getWindowThreadProcessID = windows.NewLazySystemDLL("user32.dll").NewProc("GetWindowThreadProcessId")
	isWindowVisible          = windows.NewLazySystemDLL("user32.dll").NewProc("IsWindowVisible")
	setForegroundWindow      = windows.NewLazySystemDLL("user32.dll").NewProc("SetForegroundWindow")
)

func requireIgnoredHotkeyAppRuntime(t *testing.T) {
	t.Helper()
}

func ignoredHotkeyAppTarget(t *testing.T) (string, string) {
	t.Helper()
	return "notepad.exe", "notepad.exe"
}

func ignoredHotkeyAppHotkey(t *testing.T) string {
	t.Helper()
	return "alt+space"
}

func sendIgnoredAppNativeHotkey(t *testing.T, hotkey string) {
	t.Helper()
	sendNativeHotkey(t, hotkey)
}

// activateIgnoredHotkeyApp starts one isolated Notepad process and waits until its window owns foreground focus.
func activateIgnoredHotkeyApp(t *testing.T, ctx context.Context) {
	t.Helper()
	systemRoot := os.Getenv("SystemRoot")
	if systemRoot == "" {
		systemRoot = `C:\Windows`
	}
	command := exec.Command(filepath.Join(systemRoot, "System32", "notepad.exe"))
	if err := command.Start(); err != nil {
		t.Fatalf("start Windows Notepad: %v", err)
	}
	t.Cleanup(func() {
		if command.Process != nil {
			_ = command.Process.Kill()
			_, _ = command.Process.Wait()
		}
	})

	pid := uint32(command.Process.Pid)
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		window := visibleWindowForProcess(pid)
		if window != 0 {
			setForegroundWindow.Call(window)
			if foregroundWindowProcessID() == pid {
				return
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for Windows Notepad process %d to become foreground: %v", pid, ctx.Err())
		case <-ticker.C:
		}
	}
}

// visibleWindowForProcess finds the visible top-level window owned by one process.
func visibleWindowForProcess(pid uint32) uintptr {
	var found uintptr
	callback := windows.NewCallback(func(window uintptr, _ uintptr) uintptr {
		visible, _, _ := isWindowVisible.Call(window)
		if visible == 0 {
			return 1
		}
		var windowPID uint32
		getWindowThreadProcessID.Call(window, uintptr(unsafe.Pointer(&windowPID)))
		if windowPID == pid {
			found = window
			return 0
		}
		return 1
	})
	enumWindows.Call(callback, 0)
	return found
}

// foregroundWindowProcessID returns the process currently owning the Windows foreground window.
func foregroundWindowProcessID() uint32 {
	window, _, _ := getForegroundWindow.Call()
	if window == 0 {
		return 0
	}
	var pid uint32
	getWindowThreadProcessID.Call(window, uintptr(unsafe.Pointer(&pid)))
	return pid
}
